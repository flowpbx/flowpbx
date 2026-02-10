/**
 * Weekly grid preview — shows all time switch rules overlaid on a
 * 7-day × 24-hour grid so users can visualise full-week coverage.
 */
import type { TimeSwitchRule } from '../api'

const ALL_DAYS = ['mon', 'tue', 'wed', 'thu', 'fri', 'sat', 'sun'] as const
const DAY_LABELS: Record<string, string> = {
  mon: 'Mon', tue: 'Tue', wed: 'Wed', thu: 'Thu', fri: 'Fri', sat: 'Sat', sun: 'Sun',
}

const HOURS = [0, 3, 6, 9, 12, 15, 18, 21, 24]

const RULE_COLORS = [
  { bg: 'bg-blue-400', text: 'text-blue-700', dot: 'bg-blue-400', label: 'bg-blue-50' },
  { bg: 'bg-emerald-400', text: 'text-emerald-700', dot: 'bg-emerald-400', label: 'bg-emerald-50' },
  { bg: 'bg-amber-400', text: 'text-amber-700', dot: 'bg-amber-400', label: 'bg-amber-50' },
  { bg: 'bg-violet-400', text: 'text-violet-700', dot: 'bg-violet-400', label: 'bg-violet-50' },
  { bg: 'bg-rose-400', text: 'text-rose-700', dot: 'bg-rose-400', label: 'bg-rose-50' },
  { bg: 'bg-cyan-400', text: 'text-cyan-700', dot: 'bg-cyan-400', label: 'bg-cyan-50' },
  { bg: 'bg-orange-400', text: 'text-orange-700', dot: 'bg-orange-400', label: 'bg-orange-50' },
  { bg: 'bg-teal-400', text: 'text-teal-700', dot: 'bg-teal-400', label: 'bg-teal-50' },
]

/** Parse "HH:MM" to fraction of 24h (0..1). Returns 0 for invalid input. */
function timeToFraction(t: string): number {
  const parts = t.split(':')
  if (parts.length !== 2) return 0
  const h = parseInt(parts[0], 10)
  const m = parseInt(parts[1], 10)
  if (isNaN(h) || isNaN(m)) return 0
  return (h * 60 + m) / 1440
}

interface Props {
  rules: TimeSwitchRule[]
}

export default function WeeklyGridPreview({ rules }: Props) {
  if (rules.length === 0) return null

  return (
    <div>
      <label className="block text-sm font-medium text-gray-700 mb-1">Weekly Preview</label>
      <div className="border border-gray-200 rounded-lg overflow-hidden bg-white">
        {/* Hour header */}
        <div className="flex border-b border-gray-200">
          <div className="w-12 shrink-0" />
          <div className="flex-1 relative h-5">
            {HOURS.map((h) => (
              <span
                key={h}
                className="absolute top-0.5 text-[9px] text-gray-400 -translate-x-1/2"
                style={{ left: `${(h / 24) * 100}%` }}
              >
                {h.toString().padStart(2, '0')}
              </span>
            ))}
          </div>
        </div>

        {/* Day rows */}
        {ALL_DAYS.map((day) => {
          const isWeekend = day === 'sat' || day === 'sun'
          return (
            <div
              key={day}
              className={`flex items-center border-b border-gray-100 last:border-b-0 ${isWeekend ? 'bg-gray-50/50' : ''}`}
            >
              <div className={`w-12 shrink-0 px-2 py-2 text-xs font-medium ${isWeekend ? 'text-indigo-600' : 'text-gray-600'}`}>
                {DAY_LABELS[day]}
              </div>
              <div className="flex-1 relative h-7">
                {/* Hour grid lines */}
                {HOURS.slice(1, -1).map((h) => (
                  <div
                    key={h}
                    className="absolute top-0 bottom-0 border-l border-gray-100"
                    style={{ left: `${(h / 24) * 100}%` }}
                  />
                ))}
                {/* Rule bars for this day */}
                {rules.map((rule, idx) => {
                  if (!rule.days.includes(day)) return null
                  const color = RULE_COLORS[idx % RULE_COLORS.length]
                  const startFrac = timeToFraction(rule.start)
                  const endFrac = timeToFraction(rule.end)
                  const isOvernight = endFrac <= startFrac

                  if (isOvernight) {
                    return (
                      <span key={idx}>
                        <div
                          className={`absolute top-1 bottom-1 ${color.bg} rounded-sm opacity-70`}
                          style={{ left: 0, width: `${endFrac * 100}%` }}
                          title={`${rule.label || `Rule ${idx + 1}`}: ${rule.start}–${rule.end}`}
                        />
                        <div
                          className={`absolute top-1 bottom-1 ${color.bg} rounded-sm opacity-70`}
                          style={{ left: `${startFrac * 100}%`, right: 0 }}
                          title={`${rule.label || `Rule ${idx + 1}`}: ${rule.start}–${rule.end}`}
                        />
                      </span>
                    )
                  }

                  return (
                    <div
                      key={idx}
                      className={`absolute top-1 bottom-1 ${color.bg} rounded-sm opacity-70`}
                      style={{ left: `${startFrac * 100}%`, width: `${(endFrac - startFrac) * 100}%` }}
                      title={`${rule.label || `Rule ${idx + 1}`}: ${rule.start}–${rule.end}`}
                    />
                  )
                })}
              </div>
            </div>
          )
        })}

        {/* Legend */}
        {rules.length > 0 && (
          <div className="border-t border-gray-200 bg-gray-50 px-3 py-2 flex flex-wrap gap-3">
            {rules.map((rule, idx) => {
              const color = RULE_COLORS[idx % RULE_COLORS.length]
              return (
                <span key={idx} className={`inline-flex items-center gap-1.5 ${color.label} rounded-full px-2 py-0.5`}>
                  <span className={`w-2 h-2 rounded-full ${color.dot}`} />
                  <span className={`text-[10px] font-medium ${color.text}`}>
                    {rule.label || `Rule ${idx + 1}`}
                  </span>
                  <span className="text-[10px] text-gray-400">
                    {rule.start}–{rule.end}
                  </span>
                </span>
              )
            })}
          </div>
        )}
      </div>
    </div>
  )
}
