/**
 * Visual time switch rule editor — day checkboxes with preset groups
 * and time range pickers for each routing rule.
 */
import type { TimeSwitchRule } from '../api'

const ALL_DAYS = ['mon', 'tue', 'wed', 'thu', 'fri', 'sat', 'sun'] as const
const DAY_LABELS: Record<string, string> = {
  mon: 'Mon', tue: 'Tue', wed: 'Wed', thu: 'Thu', fri: 'Fri', sat: 'Sat', sun: 'Sun',
}

const PRESETS = [
  { label: 'Weekdays', days: ['mon', 'tue', 'wed', 'thu', 'fri'] },
  { label: 'Weekend', days: ['sat', 'sun'] },
  { label: 'All', days: ['mon', 'tue', 'wed', 'thu', 'fri', 'sat', 'sun'] },
] as const

function emptyRule(): TimeSwitchRule {
  return { label: '', days: ['mon', 'tue', 'wed', 'thu', 'fri'], start: '08:30', end: '17:00', dest_node: '' }
}

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
  onChange: (rules: TimeSwitchRule[]) => void
}

export default function TimeSwitchRuleEditor({ rules, onChange }: Props) {
  function updateRule(index: number, patch: Partial<TimeSwitchRule>) {
    onChange(rules.map((rule, i) => (i === index ? { ...rule, ...patch } : rule)))
  }

  function toggleDay(ruleIndex: number, day: string) {
    onChange(
      rules.map((rule, i) => {
        if (i !== ruleIndex) return rule
        const days = rule.days.includes(day)
          ? rule.days.filter((d) => d !== day)
          : [...rule.days, day]
        return { ...rule, days }
      }),
    )
  }

  function applyPreset(ruleIndex: number, presetDays: readonly string[]) {
    onChange(
      rules.map((rule, i) => (i === ruleIndex ? { ...rule, days: [...presetDays] } : rule)),
    )
  }

  function addRule() {
    onChange([...rules, emptyRule()])
  }

  function removeRule(index: number) {
    onChange(rules.filter((_, i) => i !== index))
  }

  function moveRule(index: number, direction: -1 | 1) {
    const target = index + direction
    if (target < 0 || target >= rules.length) return
    const next = [...rules]
    ;[next[index], next[target]] = [next[target], next[index]]
    onChange(next)
  }

  return (
    <div>
      <label className="block text-sm font-medium text-gray-700 mb-1">Rules</label>
      <p className="text-xs text-gray-500 mb-3">
        Evaluated top to bottom — first matching rule wins. Drag to reorder or use the arrows.
      </p>

      <div className="border border-gray-200 rounded-lg overflow-hidden">
        {/* Header */}
        <div className="bg-gray-50 px-3 py-2 border-b border-gray-200">
          <div className="flex items-center justify-between">
            <span className="text-xs font-medium text-gray-500 uppercase tracking-wide">
              Time Rules
            </span>
            <span className="text-xs text-gray-400">
              {rules.length} rule{rules.length !== 1 ? 's' : ''}
            </span>
          </div>
        </div>

        {/* Rules list */}
        <div className="divide-y divide-gray-100">
          {rules.map((rule, idx) => (
            <div key={idx} className="p-4 space-y-3 bg-white hover:bg-gray-50/50 transition-colors">
              {/* Rule header: number, label, actions */}
              <div className="flex items-center gap-2">
                {/* Reorder arrows */}
                <div className="flex flex-col shrink-0">
                  <button
                    type="button"
                    onClick={() => moveRule(idx, -1)}
                    disabled={idx === 0}
                    className="text-gray-400 hover:text-gray-600 disabled:opacity-25 disabled:cursor-not-allowed p-0.5"
                    title="Move up"
                  >
                    <svg className="w-3.5 h-3.5" viewBox="0 0 20 20" fill="currentColor">
                      <path fillRule="evenodd" d="M14.707 12.707a1 1 0 01-1.414 0L10 9.414l-3.293 3.293a1 1 0 01-1.414-1.414l4-4a1 1 0 011.414 0l4 4a1 1 0 010 1.414z" clipRule="evenodd" />
                    </svg>
                  </button>
                  <button
                    type="button"
                    onClick={() => moveRule(idx, 1)}
                    disabled={idx === rules.length - 1}
                    className="text-gray-400 hover:text-gray-600 disabled:opacity-25 disabled:cursor-not-allowed p-0.5"
                    title="Move down"
                  >
                    <svg className="w-3.5 h-3.5" viewBox="0 0 20 20" fill="currentColor">
                      <path fillRule="evenodd" d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z" clipRule="evenodd" />
                    </svg>
                  </button>
                </div>

                {/* Rule number badge */}
                <span className="flex-shrink-0 inline-flex items-center justify-center w-5 h-5 rounded bg-blue-100 text-xs font-semibold text-blue-700">
                  {idx + 1}
                </span>

                {/* Label */}
                <input
                  type="text"
                  value={rule.label}
                  onChange={(e) => updateRule(idx, { label: e.currentTarget.value })}
                  placeholder="Rule label (e.g. Business Hours)"
                  className="flex-1 min-w-0 rounded-md border border-gray-300 px-3 py-1.5 text-sm text-gray-900 placeholder-gray-400 focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
                />

                {/* Remove */}
                {rules.length > 1 && (
                  <button
                    type="button"
                    onClick={() => removeRule(idx)}
                    className="shrink-0 text-gray-400 hover:text-red-500 transition-colors p-1"
                    title="Remove rule"
                  >
                    <svg className="w-4 h-4" viewBox="0 0 20 20" fill="currentColor">
                      <path fillRule="evenodd" d="M9 2a1 1 0 00-.894.553L7.382 4H4a1 1 0 000 2v10a2 2 0 002 2h8a2 2 0 002-2V6a1 1 0 100-2h-3.382l-.724-1.447A1 1 0 0011 2H9zM7 8a1 1 0 012 0v6a1 1 0 11-2 0V8zm5-1a1 1 0 00-1 1v6a1 1 0 102 0V8a1 1 0 00-1-1z" clipRule="evenodd" />
                    </svg>
                  </button>
                )}
              </div>

              {/* Day selection with presets */}
              <div className="space-y-2">
                <div className="flex items-center gap-3">
                  <span className="text-xs text-gray-500 shrink-0">Days</span>
                  <div className="flex gap-1">
                    {PRESETS.map((preset) => {
                      const isActive =
                        preset.days.length === rule.days.length &&
                        preset.days.every((d) => rule.days.includes(d))
                      return (
                        <button
                          key={preset.label}
                          type="button"
                          onClick={() => applyPreset(idx, preset.days)}
                          className={`px-2 py-0.5 rounded text-[10px] font-medium transition-colors ${
                            isActive
                              ? 'bg-blue-100 text-blue-700'
                              : 'bg-gray-50 text-gray-400 hover:bg-gray-100 hover:text-gray-600'
                          }`}
                        >
                          {preset.label}
                        </button>
                      )
                    })}
                  </div>
                </div>

                <div className="flex gap-1.5">
                  {ALL_DAYS.map((day) => {
                    const active = rule.days.includes(day)
                    const isWeekend = day === 'sat' || day === 'sun'
                    return (
                      <button
                        key={day}
                        type="button"
                        onClick={() => toggleDay(idx, day)}
                        className={`w-10 py-1.5 rounded text-xs font-medium transition-colors ${
                          active
                            ? isWeekend
                              ? 'bg-indigo-600 text-white'
                              : 'bg-blue-600 text-white'
                            : isWeekend
                              ? 'bg-gray-50 text-gray-400 hover:bg-gray-100 border border-dashed border-gray-300'
                              : 'bg-gray-100 text-gray-500 hover:bg-gray-200'
                        }`}
                      >
                        {DAY_LABELS[day]}
                      </button>
                    )
                  })}
                </div>
              </div>

              {/* Time range */}
              <div className="space-y-2">
                <div className="flex items-end gap-3">
                  <div className="flex-1">
                    <label className="block text-xs text-gray-500 mb-0.5">Start Time</label>
                    <input
                      type="time"
                      value={rule.start}
                      onChange={(e) => updateRule(idx, { start: e.currentTarget.value })}
                      className="block w-full rounded-md border border-gray-300 px-3 py-1.5 text-sm text-gray-900 focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
                    />
                  </div>
                  <span className="pb-2 text-gray-400 text-sm">to</span>
                  <div className="flex-1">
                    <label className="block text-xs text-gray-500 mb-0.5">End Time</label>
                    <input
                      type="time"
                      value={rule.end}
                      onChange={(e) => updateRule(idx, { end: e.currentTarget.value })}
                      className="block w-full rounded-md border border-gray-300 px-3 py-1.5 text-sm text-gray-900 focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
                    />
                  </div>
                </div>

                {/* Visual time bar */}
                <TimeRangeBar start={rule.start} end={rule.end} />
              </div>

              {/* Destination */}
              <div>
                <label className="block text-xs text-gray-500 mb-0.5">Destination</label>
                <input
                  type="text"
                  value={rule.dest_node}
                  onChange={(e) => updateRule(idx, { dest_node: e.currentTarget.value })}
                  placeholder="Node ID — the flow node to route to when this rule matches"
                  className="block w-full rounded-md border border-gray-300 px-3 py-1.5 text-sm text-gray-900 placeholder-gray-400 focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
                />
              </div>
            </div>
          ))}
        </div>

        {/* Add rule button */}
        <div className="border-t border-gray-200 bg-gray-50 px-3 py-2">
          <button
            type="button"
            onClick={addRule}
            className="inline-flex items-center gap-1.5 text-sm text-blue-600 hover:text-blue-800 transition-colors"
          >
            <svg className="w-4 h-4" viewBox="0 0 20 20" fill="currentColor">
              <path fillRule="evenodd" d="M10 3a1 1 0 011 1v5h5a1 1 0 110 2h-5v5a1 1 0 11-2 0v-5H4a1 1 0 110-2h5V4a1 1 0 011-1z" clipRule="evenodd" />
            </svg>
            Add Rule
          </button>
        </div>
      </div>
    </div>
  )
}

/** Visual bar showing the active time range within a 24h period. */
function TimeRangeBar({ start, end }: { start: string; end: string }) {
  const startFrac = timeToFraction(start)
  const endFrac = timeToFraction(end)
  const isOvernight = endFrac <= startFrac

  // Hour markers
  const hours = [0, 6, 12, 18, 24]

  return (
    <div className="relative">
      {/* Bar container */}
      <div className="relative h-5 bg-gray-100 rounded-full overflow-hidden">
        {isOvernight ? (
          <>
            {/* Overnight: two segments — start to midnight, midnight to end */}
            <div
              className="absolute inset-y-0 bg-blue-200 rounded-l-full"
              style={{ left: 0, width: `${endFrac * 100}%` }}
            />
            <div
              className="absolute inset-y-0 bg-blue-200 rounded-r-full"
              style={{ left: `${startFrac * 100}%`, right: 0 }}
            />
          </>
        ) : (
          <div
            className="absolute inset-y-0 bg-blue-200 rounded-full"
            style={{ left: `${startFrac * 100}%`, width: `${(endFrac - startFrac) * 100}%` }}
          />
        )}
      </div>

      {/* Hour labels */}
      <div className="relative h-3 mt-0.5">
        {hours.map((h) => (
          <span
            key={h}
            className="absolute text-[9px] text-gray-400 -translate-x-1/2"
            style={{ left: `${(h / 24) * 100}%` }}
          >
            {h.toString().padStart(2, '0')}
          </span>
        ))}
      </div>
    </div>
  )
}
