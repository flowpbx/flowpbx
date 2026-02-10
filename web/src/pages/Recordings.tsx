import { useState, useEffect, useRef } from 'react'
import { listRecordings, deleteRecording, recordingDownloadURL, ApiError } from '../api'
import type { Recording } from '../api'
import DataTable, { type Column } from '../components/DataTable'

const PAGE_SIZE = 20

export default function Recordings() {
  const [recordings, setRecordings] = useState<Recording[]>([])
  const [total, setTotal] = useState(0)
  const [offset, setOffset] = useState(0)
  const [loading, setLoading] = useState(true)

  // Filter state.
  const [search, setSearch] = useState('')
  const [startDate, setStartDate] = useState('')
  const [endDate, setEndDate] = useState('')

  // Audio playback state.
  const [playingId, setPlayingId] = useState<number | null>(null)
  const audioRef = useRef<HTMLAudioElement | null>(null)

  function load(newOffset: number) {
    setLoading(true)
    listRecordings({
      limit: PAGE_SIZE,
      offset: newOffset,
      search: search || undefined,
      start_date: startDate || undefined,
      end_date: endDate || undefined,
    })
      .then((res) => {
        setRecordings(res.items)
        setTotal(res.total)
        setOffset(newOffset)
      })
      .catch(() => {
        setRecordings([])
        setTotal(0)
      })
      .finally(() => setLoading(false))
  }

  useEffect(() => {
    load(0)
  }, [])

  function handleFilter() {
    load(0)
  }

  function handlePlay(rec: Recording) {
    // Stop current playback if any.
    if (audioRef.current) {
      audioRef.current.pause()
      audioRef.current = null
    }

    if (playingId === rec.id) {
      setPlayingId(null)
      return
    }

    const audio = new Audio(recordingDownloadURL(rec.id))
    audio.onended = () => setPlayingId(null)
    audio.onerror = () => setPlayingId(null)
    audio.play()
    audioRef.current = audio
    setPlayingId(rec.id)
  }

  function handleDownload(rec: Recording) {
    const link = document.createElement('a')
    link.href = recordingDownloadURL(rec.id)
    link.download = rec.filename
    link.click()
  }

  async function handleDelete(rec: Recording) {
    if (!confirm(`Delete recording "${rec.filename}"? This cannot be undone.`)) return
    try {
      await deleteRecording(rec.id)
      // Stop playback if deleting the currently playing recording.
      if (playingId === rec.id && audioRef.current) {
        audioRef.current.pause()
        audioRef.current = null
        setPlayingId(null)
      }
      load(offset)
    } catch (err) {
      alert(err instanceof ApiError ? err.message : 'unable to delete recording')
    }
  }

  function formatDuration(seconds?: number): string {
    if (seconds === undefined || seconds === null) return '—'
    const m = Math.floor(seconds / 60)
    const s = seconds % 60
    return `${m}:${s.toString().padStart(2, '0')}`
  }

  function formatTime(iso: string): string {
    return new Date(iso).toLocaleString()
  }

  function formatFileSize(bytes?: number): string {
    if (bytes === undefined || bytes === null) return '—'
    if (bytes < 1024) return `${bytes} B`
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
  }

  const columns: Column<Recording>[] = [
    {
      key: 'start_time',
      header: 'Date',
      render: (r) => (
        <span className="whitespace-nowrap">{formatTime(r.start_time)}</span>
      ),
    },
    {
      key: 'direction',
      header: 'Direction',
      render: (r) => <DirectionBadge direction={r.direction} />,
    },
    {
      key: 'caller',
      header: 'From',
      render: (r) => {
        if (r.caller_id_name) {
          return (
            <span>
              <span className="font-medium">{r.caller_id_name}</span>{' '}
              <span className="text-gray-500">{r.caller_id_num}</span>
            </span>
          )
        }
        return r.caller_id_num || '—'
      },
    },
    {
      key: 'callee',
      header: 'To',
      render: (r) => r.callee || '—',
    },
    {
      key: 'duration',
      header: 'Duration',
      render: (r) => formatDuration(r.duration),
    },
    {
      key: 'file_size',
      header: 'Size',
      render: (r) => (
        <span className="text-xs text-gray-500">{formatFileSize(r.file_size)}</span>
      ),
    },
    {
      key: 'actions',
      header: '',
      className: 'w-36',
      render: (r) => (
        <div className="flex gap-2">
          <button
            type="button"
            onClick={(e) => { e.stopPropagation(); handlePlay(r) }}
            className={`text-sm ${playingId === r.id ? 'text-orange-600 hover:text-orange-800' : 'text-blue-600 hover:text-blue-800'}`}
            title={playingId === r.id ? 'Stop' : 'Play'}
          >
            {playingId === r.id ? 'Stop' : 'Play'}
          </button>
          <button
            type="button"
            onClick={(e) => { e.stopPropagation(); handleDownload(r) }}
            className="text-sm text-gray-600 hover:text-gray-800"
            title="Download"
          >
            Download
          </button>
          <button
            type="button"
            onClick={(e) => { e.stopPropagation(); handleDelete(r) }}
            className="text-sm text-red-600 hover:text-red-800"
            title="Delete"
          >
            Delete
          </button>
        </div>
      ),
    },
  ]

  // Calculate total storage size from current page.
  const totalSize = recordings.reduce((sum, r) => sum + (r.file_size ?? 0), 0)

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Recordings</h1>
          <p className="mt-1 text-sm text-gray-500">
            Browse, search, play, and manage call recordings.
            {total > 0 && (
              <span className="ml-2 text-gray-400">
                {total} recording{total !== 1 ? 's' : ''}
                {totalSize > 0 && ` — ${formatFileSize(totalSize)} on this page`}
              </span>
            )}
          </p>
        </div>
      </div>

      {/* Filters */}
      <div className="flex flex-wrap items-end gap-3 mb-4">
        <div>
          <label
            htmlFor="rec_search"
            className="block text-xs font-medium text-gray-500 mb-1"
          >
            Search
          </label>
          <input
            id="rec_search"
            type="text"
            value={search}
            onChange={(e) => setSearch(e.currentTarget.value)}
            onKeyDown={(e) => e.key === 'Enter' && handleFilter()}
            placeholder="Name, number, or callee"
            className="block w-56 rounded-md border border-gray-300 px-3 py-1.5 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500"
          />
        </div>
        <div>
          <label
            htmlFor="rec_start"
            className="block text-xs font-medium text-gray-500 mb-1"
          >
            From
          </label>
          <input
            id="rec_start"
            type="date"
            value={startDate}
            onChange={(e) => setStartDate(e.currentTarget.value)}
            className="block rounded-md border border-gray-300 px-3 py-1.5 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500"
          />
        </div>
        <div>
          <label
            htmlFor="rec_end"
            className="block text-xs font-medium text-gray-500 mb-1"
          >
            To
          </label>
          <input
            id="rec_end"
            type="date"
            value={endDate}
            onChange={(e) => setEndDate(e.currentTarget.value)}
            className="block rounded-md border border-gray-300 px-3 py-1.5 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500"
          />
        </div>
        <button
          type="button"
          onClick={handleFilter}
          className="rounded-md bg-blue-600 px-4 py-1.5 text-sm font-medium text-white hover:bg-blue-700 transition-colors"
        >
          Filter
        </button>
        {(search || startDate || endDate) && (
          <button
            type="button"
            onClick={() => {
              setSearch('')
              setStartDate('')
              setEndDate('')
              setTimeout(() => load(0), 0)
            }}
            className="text-sm text-gray-500 hover:text-gray-700"
          >
            Clear
          </button>
        )}
      </div>

      {loading ? (
        <p className="text-sm text-gray-400">Loading...</p>
      ) : (
        <DataTable
          columns={columns}
          rows={recordings}
          keyFn={(r) => r.id}
          total={total}
          limit={PAGE_SIZE}
          offset={offset}
          onPageChange={load}
          emptyMessage="No recordings found."
        />
      )}
    </div>
  )
}

function DirectionBadge({ direction }: { direction: string }) {
  const styles: Record<string, string> = {
    inbound: 'bg-blue-50 text-blue-700',
    outbound: 'bg-emerald-50 text-emerald-700',
    internal: 'bg-gray-100 text-gray-700',
  }
  const labels: Record<string, string> = {
    inbound: 'In',
    outbound: 'Out',
    internal: 'Int',
  }
  return (
    <span
      className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${
        styles[direction] ?? 'bg-gray-100 text-gray-700'
      }`}
    >
      {labels[direction] ?? direction}
    </span>
  )
}
