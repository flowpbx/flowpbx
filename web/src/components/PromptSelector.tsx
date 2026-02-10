import { useState, useEffect, useRef } from 'react'
import { listPrompts, uploadPrompt, promptAudioURL } from '../api'
import type { AudioPrompt } from '../api'

interface PromptSelectorProps {
  /** Currently selected prompt filename (matches greeting_file). */
  value: string
  /** Called when the user selects or clears a prompt. */
  onChange: (filename: string) => void
}

/**
 * Allows the user to select an audio prompt from the library or upload a new one.
 * The selected value is the prompt filename stored in greeting_file.
 */
export default function PromptSelector({ value, onChange }: PromptSelectorProps) {
  const [prompts, setPrompts] = useState<AudioPrompt[]>([])
  const [loading, setLoading] = useState(true)
  const [uploading, setUploading] = useState(false)
  const [error, setError] = useState('')
  const [playingId, setPlayingId] = useState<number | null>(null)
  const audioRef = useRef<HTMLAudioElement | null>(null)
  const fileInputRef = useRef<HTMLInputElement | null>(null)

  function load() {
    setLoading(true)
    listPrompts()
      .then((items) => setPrompts(items))
      .catch(() => setPrompts([]))
      .finally(() => setLoading(false))
  }

  useEffect(() => {
    load()
  }, [])

  async function handleUpload(files: FileList | null) {
    if (!files || files.length === 0) return
    setError('')
    setUploading(true)

    try {
      const uploaded = await uploadPrompt(files[0])
      load()
      onChange(uploaded.filename)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'upload failed')
    } finally {
      setUploading(false)
      if (fileInputRef.current) {
        fileInputRef.current.value = ''
      }
    }
  }

  function togglePlay(prompt: AudioPrompt) {
    if (playingId === prompt.id) {
      stopPlayback()
      return
    }

    stopPlayback()
    const audio = new Audio(promptAudioURL(prompt.id))
    audio.addEventListener('ended', () => setPlayingId(null))
    audio.addEventListener('error', () => setPlayingId(null))
    audio.play()
    audioRef.current = audio
    setPlayingId(prompt.id)
  }

  function stopPlayback() {
    if (audioRef.current) {
      audioRef.current.pause()
      audioRef.current = null
    }
    setPlayingId(null)
  }

  const selected = prompts.find((p) => p.filename === value)

  return (
    <div className="space-y-2">
      {error && (
        <p className="text-xs text-red-600">{error}</p>
      )}

      {/* Selected prompt display */}
      {selected && (
        <div className="flex items-center gap-2 rounded-md border border-green-200 bg-green-50 px-3 py-2">
          <svg className="h-4 w-4 text-green-600 shrink-0" fill="none" viewBox="0 0 24 24" strokeWidth="1.5" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" d="M19.114 5.636a9 9 0 010 12.728M16.463 8.288a5.25 5.25 0 010 7.424M6.75 8.25l4.72-4.72a.75.75 0 011.28.53v15.88a.75.75 0 01-1.28.53l-4.72-4.72H4.51c-.88 0-1.704-.507-1.938-1.354A9.01 9.01 0 012.25 12c0-.83.112-1.633.322-2.396C2.806 8.756 3.63 8.25 4.51 8.25H6.75z" />
          </svg>
          <span className="text-sm text-green-800 font-medium truncate flex-1">{selected.name}</span>
          <button
            type="button"
            onClick={() => togglePlay(selected)}
            className="text-xs text-green-700 hover:text-green-900 font-medium"
          >
            {playingId === selected.id ? 'Stop' : 'Play'}
          </button>
          <button
            type="button"
            onClick={() => { stopPlayback(); onChange('') }}
            className="text-xs text-gray-500 hover:text-gray-700"
          >
            Clear
          </button>
        </div>
      )}

      {/* Prompt list */}
      {!selected && (
        <>
          {loading ? (
            <p className="text-xs text-gray-400 py-2">Loading prompts...</p>
          ) : prompts.length === 0 ? (
            <p className="text-xs text-gray-500 py-2">No audio prompts in library. Upload one below.</p>
          ) : (
            <div className="max-h-40 overflow-y-auto rounded-md border border-gray-200">
              {prompts.map((p) => (
                <div
                  key={p.id}
                  className="flex items-center gap-2 px-3 py-1.5 hover:bg-gray-50 cursor-pointer border-b border-gray-100 last:border-b-0"
                  onClick={() => onChange(p.filename)}
                >
                  <span className="text-sm text-gray-800 flex-1 truncate">{p.name}</span>
                  <span className="text-xs text-gray-400">{p.format.toUpperCase()}</span>
                  <button
                    type="button"
                    onClick={(e) => { e.stopPropagation(); togglePlay(p) }}
                    className="text-xs text-blue-600 hover:text-blue-800"
                  >
                    {playingId === p.id ? 'Stop' : 'Play'}
                  </button>
                </div>
              ))}
            </div>
          )}
        </>
      )}

      {/* Upload button */}
      <div>
        <input
          ref={fileInputRef}
          type="file"
          accept=".wav,.alaw,.al,.ulaw,.ul"
          className="hidden"
          onChange={(e) => handleUpload(e.target.files)}
        />
        <button
          type="button"
          disabled={uploading}
          onClick={() => fileInputRef.current?.click()}
          className="text-xs text-blue-600 hover:text-blue-800 font-medium disabled:opacity-50 disabled:cursor-not-allowed"
        >
          {uploading ? 'Uploading...' : 'Upload new prompt'}
        </button>
      </div>
    </div>
  )
}
