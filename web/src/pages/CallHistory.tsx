import { useState, useEffect } from 'react'
import { listCDRs, buildExportURL } from '../api'
import type { CDR } from '../api'
import DataTable, { type Column } from '../components/DataTable'

const PAGE_SIZE = 20

export default function CallHistory() {
  const [cdrs, setCdrs] = useState<CDR[]>([])
  const [total, setTotal] = useState(0)
  const [offset, setOffset] = useState(0)
  const [loading, setLoading] = useState(true)

  // Filter state.
  const [search, setSearch] = useState('')
  const [direction, setDirection] = useState('')
  const [startDate, setStartDate] = useState('')
  const [endDate, setEndDate] = useState('')

  function load(newOffset: number) {
    setLoading(true)
    listCDRs({
      limit: PAGE_SIZE,
      offset: newOffset,
      search: search || undefined,
      direction: direction || undefined,
      start_date: startDate || undefined,
      end_date: endDate || undefined,
    })
      .then((res) => {
        setCdrs(res.items)
        setTotal(res.total)
        setOffset(newOffset)
      })
      .catch(() => {
        setCdrs([])
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

  function handleExport() {
    const url = buildExportURL({
      search: search || undefined,
      direction: direction || undefined,
      start_date: startDate || undefined,
      end_date: endDate || undefined,
    })
    window.open(url, '_blank')
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

  const columns: Column<CDR>[] = [
    {
      key: 'start_time',
      header: 'Time',
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
      key: 'disposition',
      header: 'Status',
      render: (r) => <DispositionBadge disposition={r.disposition} />,
    },
    {
      key: 'hangup_cause',
      header: 'Cause',
      render: (r) => (
        <span className="text-xs text-gray-500">{r.hangup_cause || '—'}</span>
      ),
    },
  ]

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Call History</h1>
          <p className="mt-1 text-sm text-gray-500">
            Searchable and filterable call detail records.
          </p>
        </div>
        <button
          type="button"
          onClick={handleExport}
          className="rounded-md border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 transition-colors"
        >
          Export CSV
        </button>
      </div>

      {/* Filters */}
      <div className="flex flex-wrap items-end gap-3 mb-4">
        <div>
          <label
            htmlFor="cdr_search"
            className="block text-xs font-medium text-gray-500 mb-1"
          >
            Search
          </label>
          <input
            id="cdr_search"
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
            htmlFor="cdr_direction"
            className="block text-xs font-medium text-gray-500 mb-1"
          >
            Direction
          </label>
          <select
            id="cdr_direction"
            value={direction}
            onChange={(e) => setDirection(e.currentTarget.value)}
            className="block w-32 rounded-md border border-gray-300 px-3 py-1.5 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500"
          >
            <option value="">All</option>
            <option value="inbound">Inbound</option>
            <option value="outbound">Outbound</option>
            <option value="internal">Internal</option>
          </select>
        </div>
        <div>
          <label
            htmlFor="cdr_start"
            className="block text-xs font-medium text-gray-500 mb-1"
          >
            From
          </label>
          <input
            id="cdr_start"
            type="date"
            value={startDate}
            onChange={(e) => setStartDate(e.currentTarget.value)}
            className="block rounded-md border border-gray-300 px-3 py-1.5 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500"
          />
        </div>
        <div>
          <label
            htmlFor="cdr_end"
            className="block text-xs font-medium text-gray-500 mb-1"
          >
            To
          </label>
          <input
            id="cdr_end"
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
        {(search || direction || startDate || endDate) && (
          <button
            type="button"
            onClick={() => {
              setSearch('')
              setDirection('')
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
          rows={cdrs}
          keyFn={(r) => r.id}
          total={total}
          limit={PAGE_SIZE}
          offset={offset}
          onPageChange={load}
          emptyMessage="No call records found."
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

function DispositionBadge({ disposition }: { disposition: string }) {
  const styles: Record<string, string> = {
    answered: 'bg-green-50 text-green-700',
    no_answer: 'bg-yellow-50 text-yellow-700',
    busy: 'bg-orange-50 text-orange-700',
    cancelled: 'bg-gray-100 text-gray-500',
    failed: 'bg-red-50 text-red-700',
    in_progress: 'bg-blue-50 text-blue-700',
  }
  const labels: Record<string, string> = {
    answered: 'Answered',
    no_answer: 'No Answer',
    busy: 'Busy',
    cancelled: 'Cancelled',
    failed: 'Failed',
    in_progress: 'In Progress',
  }
  return (
    <span
      className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${
        styles[disposition] ?? 'bg-gray-100 text-gray-700'
      }`}
    >
      {labels[disposition] ?? disposition}
    </span>
  )
}
