import { useState, useEffect, useRef } from 'react'
import { listPrompts, uploadPrompt, deletePrompt, promptAudioURL } from '../api'
import type { AudioPrompt } from '../api'
import DataTable, { type Column } from '../components/DataTable'

export default function Prompts() {
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
      for (let i = 0; i < files.length; i++) {
        await uploadPrompt(files[i])
      }
      load()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'upload failed')
    } finally {
      setUploading(false)
      if (fileInputRef.current) {
        fileInputRef.current.value = ''
      }
    }
  }

  async function handleDelete(prompt: AudioPrompt) {
    if (!confirm(`Delete prompt "${prompt.name}"?`)) return
    try {
      await deletePrompt(prompt.id)
      stopPlayback()
      load()
    } catch (err) {
      alert(err instanceof Error ? err.message : 'unable to delete prompt')
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

  function formatSize(bytes: number): string {
    if (bytes < 1024) return `${bytes} B`
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
  }

  function formatDate(iso: string): string {
    return new Date(iso).toLocaleDateString(undefined, {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    })
  }

  const columns: Column<AudioPrompt>[] = [
    { key: 'name', header: 'Name', render: (r) => r.name },
    { key: 'filename', header: 'Filename', render: (r) => r.filename },
    {
      key: 'format',
      header: 'Format',
      render: (r) => (
        <span className="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium bg-gray-100 text-gray-700">
          {r.format.toUpperCase()}
        </span>
      ),
    },
    { key: 'file_size', header: 'Size', render: (r) => formatSize(r.file_size) },
    { key: 'created_at', header: 'Uploaded', render: (r) => formatDate(r.created_at) },
    {
      key: 'actions',
      header: '',
      className: 'w-28',
      render: (r) => (
        <div className="flex gap-2">
          <button
            type="button"
            onClick={(e) => { e.stopPropagation(); togglePlay(r) }}
            className="text-sm text-blue-600 hover:text-blue-800"
          >
            {playingId === r.id ? 'Stop' : 'Play'}
          </button>
          <button
            type="button"
            onClick={(e) => { e.stopPropagation(); handleDelete(r) }}
            className="text-sm text-red-600 hover:text-red-800"
          >
            Delete
          </button>
        </div>
      ),
    },
  ]

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Audio Prompts</h1>
          <p className="mt-1 text-sm text-gray-500">Upload and manage audio prompts for IVR menus and call flows.</p>
        </div>
        <div>
          <input
            ref={fileInputRef}
            type="file"
            accept=".wav,.alaw,.al,.ulaw,.ul"
            multiple
            className="hidden"
            onChange={(e) => handleUpload(e.target.files)}
          />
          <button
            type="button"
            disabled={uploading}
            onClick={() => fileInputRef.current?.click()}
            className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
          >
            {uploading ? 'Uploading...' : 'Upload Prompt'}
          </button>
        </div>
      </div>

      {error && (
        <div className="rounded-md bg-red-50 border border-red-200 px-3 py-2 mb-4">
          <p className="text-sm text-red-700">{error}</p>
        </div>
      )}

      {loading ? (
        <p className="text-sm text-gray-400">Loading...</p>
      ) : (
        <DataTable
          columns={columns}
          rows={prompts}
          keyFn={(r) => r.id}
          total={prompts.length}
          limit={prompts.length || 1}
          offset={0}
          onPageChange={() => {}}
          emptyMessage="No audio prompts uploaded yet."
        />
      )}
    </div>
  )
}
