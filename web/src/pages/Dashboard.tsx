import { useState, useEffect } from 'react'
import { get } from '../api/client'

interface DashboardStats {
  active_calls: number
  registered_devices: number
  total_extensions: number
  total_trunks: number
  recent_cdrs: CdrEntry[]
}

interface CdrEntry {
  id: number
  caller: string
  callee: string
  direction: string
  duration: number
  status: string
  timestamp: string
}

const emptyStats: DashboardStats = {
  active_calls: 0,
  registered_devices: 0,
  total_extensions: 0,
  total_trunks: 0,
  recent_cdrs: [],
}

export default function Dashboard() {
  const [stats, setStats] = useState<DashboardStats>(emptyStats)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    let cancelled = false
    get<DashboardStats>('/dashboard/stats')
      .then((data) => {
        if (!cancelled) setStats(data)
      })
      .catch(() => {
        // API not available yet — show zeros
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [])

  function formatDuration(seconds: number): string {
    const m = Math.floor(seconds / 60)
    const s = seconds % 60
    return `${m}:${s.toString().padStart(2, '0')}`
  }

  return (
    <div>
      <h1 className="text-2xl font-bold text-gray-900">Dashboard</h1>
      <p className="mt-1 text-sm text-gray-500">System overview and recent activity.</p>

      {/* Stat cards */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4 mt-6">
        <StatCard label="Active Calls" value={stats.active_calls} loading={loading} color="blue" />
        <StatCard label="Registered Devices" value={stats.registered_devices} loading={loading} color="green" />
        <StatCard label="Extensions" value={stats.total_extensions} loading={loading} color="gray" />
        <StatCard label="Trunks" value={stats.total_trunks} loading={loading} color="gray" />
      </div>

      {/* Recent CDRs */}
      <div className="mt-8">
        <h2 className="text-lg font-semibold text-gray-900 mb-3">Recent Calls</h2>
        <div className="overflow-x-auto border border-gray-200 rounded-lg">
          <table className="min-w-full divide-y divide-gray-200">
            <thead className="bg-gray-50">
              <tr>
                <th className="px-4 py-2.5 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Time
                </th>
                <th className="px-4 py-2.5 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Direction
                </th>
                <th className="px-4 py-2.5 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  From
                </th>
                <th className="px-4 py-2.5 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  To
                </th>
                <th className="px-4 py-2.5 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Duration
                </th>
                <th className="px-4 py-2.5 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Status
                </th>
              </tr>
            </thead>
            <tbody className="bg-white divide-y divide-gray-200">
              {loading ? (
                <tr>
                  <td colSpan={6} className="px-4 py-8 text-center text-sm text-gray-400">
                    Loading...
                  </td>
                </tr>
              ) : stats.recent_cdrs.length === 0 ? (
                <tr>
                  <td colSpan={6} className="px-4 py-8 text-center text-sm text-gray-400">
                    No recent calls.
                  </td>
                </tr>
              ) : (
                stats.recent_cdrs.map((cdr) => (
                  <tr key={cdr.id}>
                    <td className="px-4 py-2.5 text-sm text-gray-900 whitespace-nowrap">
                      {new Date(cdr.timestamp).toLocaleString()}
                    </td>
                    <td className="px-4 py-2.5 text-sm text-gray-900">
                      <DirectionBadge direction={cdr.direction} />
                    </td>
                    <td className="px-4 py-2.5 text-sm text-gray-900">{cdr.caller}</td>
                    <td className="px-4 py-2.5 text-sm text-gray-900">{cdr.callee}</td>
                    <td className="px-4 py-2.5 text-sm text-gray-900">{formatDuration(cdr.duration)}</td>
                    <td className="px-4 py-2.5 text-sm">
                      <StatusBadge status={cdr.status} />
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  )
}

function StatCard({
  label,
  value,
  loading,
  color,
}: {
  label: string
  value: number
  loading: boolean
  color: 'blue' | 'green' | 'gray'
}) {
  const colorMap = {
    blue: 'text-blue-600',
    green: 'text-green-600',
    gray: 'text-gray-900',
  }
  return (
    <div className="bg-white border border-gray-200 rounded-lg p-4">
      <p className="text-sm font-medium text-gray-500">{label}</p>
      <p className={`mt-1 text-2xl font-bold ${colorMap[color]}`}>
        {loading ? '—' : value}
      </p>
    </div>
  )
}

function DirectionBadge({ direction }: { direction: string }) {
  const isInbound = direction === 'inbound'
  return (
    <span
      className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${
        isInbound
          ? 'bg-blue-50 text-blue-700'
          : 'bg-gray-100 text-gray-700'
      }`}
    >
      {isInbound ? 'In' : 'Out'}
    </span>
  )
}

function StatusBadge({ status }: { status: string }) {
  const styles: Record<string, string> = {
    answered: 'bg-green-50 text-green-700',
    missed: 'bg-red-50 text-red-700',
    busy: 'bg-yellow-50 text-yellow-700',
    failed: 'bg-red-50 text-red-700',
  }
  return (
    <span
      className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${
        styles[status] ?? 'bg-gray-100 text-gray-700'
      }`}
    >
      {status}
    </span>
  )
}
