/**
 * Holiday / specific-date override editor for time switches.
 * Overrides are evaluated before regular day/time rules — if the
 * current date matches an override it takes priority.
 */
import type { TimeSwitchOverride } from '../api'

function emptyOverride(): TimeSwitchOverride {
  return { label: '', date: '', start: '', end: '', dest_node: '' }
}

interface Props {
  overrides: TimeSwitchOverride[]
  onChange: (overrides: TimeSwitchOverride[]) => void
}

export default function HolidayOverrideEditor({ overrides, onChange }: Props) {
  function updateOverride(index: number, patch: Partial<TimeSwitchOverride>) {
    onChange(overrides.map((ov, i) => (i === index ? { ...ov, ...patch } : ov)))
  }

  function addOverride() {
    onChange([...overrides, emptyOverride()])
  }

  function removeOverride(index: number) {
    onChange(overrides.filter((_, i) => i !== index))
  }

  function moveOverride(index: number, direction: -1 | 1) {
    const target = index + direction
    if (target < 0 || target >= overrides.length) return
    const next = [...overrides]
    ;[next[index], next[target]] = [next[target], next[index]]
    onChange(next)
  }

  return (
    <div>
      <label className="block text-sm font-medium text-gray-700 mb-1">
        Holiday / Date Overrides
      </label>
      <p className="text-xs text-gray-500 mb-3">
        Specific-date overrides take priority over regular time rules. Leave start/end empty for all-day overrides.
      </p>

      <div className="border border-gray-200 rounded-lg overflow-hidden">
        {/* Header */}
        <div className="bg-gray-50 px-3 py-2 border-b border-gray-200">
          <div className="flex items-center justify-between">
            <span className="text-xs font-medium text-gray-500 uppercase tracking-wide">
              Date Overrides
            </span>
            <span className="text-xs text-gray-400">
              {overrides.length} override{overrides.length !== 1 ? 's' : ''}
            </span>
          </div>
        </div>

        {/* Override list */}
        {overrides.length === 0 ? (
          <div className="px-4 py-6 text-center text-sm text-gray-400">
            No overrides configured. Add holidays or specific dates that should bypass regular rules.
          </div>
        ) : (
          <div className="divide-y divide-gray-100">
            {overrides.map((ov, idx) => (
              <div key={idx} className="p-4 space-y-3 bg-white hover:bg-gray-50/50 transition-colors">
                {/* Override header: reorder, badge, label, remove */}
                <div className="flex items-center gap-2">
                  {/* Reorder arrows */}
                  <div className="flex flex-col shrink-0">
                    <button
                      type="button"
                      onClick={() => moveOverride(idx, -1)}
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
                      onClick={() => moveOverride(idx, 1)}
                      disabled={idx === overrides.length - 1}
                      className="text-gray-400 hover:text-gray-600 disabled:opacity-25 disabled:cursor-not-allowed p-0.5"
                      title="Move down"
                    >
                      <svg className="w-3.5 h-3.5" viewBox="0 0 20 20" fill="currentColor">
                        <path fillRule="evenodd" d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z" clipRule="evenodd" />
                      </svg>
                    </button>
                  </div>

                  {/* Badge */}
                  <span className="flex-shrink-0 inline-flex items-center justify-center w-5 h-5 rounded bg-amber-100 text-xs font-semibold text-amber-700">
                    {idx + 1}
                  </span>

                  {/* Label */}
                  <input
                    type="text"
                    value={ov.label}
                    onChange={(e) => updateOverride(idx, { label: e.currentTarget.value })}
                    placeholder="Override label (e.g. Christmas Day)"
                    className="flex-1 min-w-0 rounded-md border border-gray-300 px-3 py-1.5 text-sm text-gray-900 placeholder-gray-400 focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
                  />

                  {/* Remove */}
                  <button
                    type="button"
                    onClick={() => removeOverride(idx)}
                    className="shrink-0 text-gray-400 hover:text-red-500 transition-colors p-1"
                    title="Remove override"
                  >
                    <svg className="w-4 h-4" viewBox="0 0 20 20" fill="currentColor">
                      <path fillRule="evenodd" d="M9 2a1 1 0 00-.894.553L7.382 4H4a1 1 0 000 2v10a2 2 0 002 2h8a2 2 0 002-2V6a1 1 0 100-2h-3.382l-.724-1.447A1 1 0 0011 2H9zM7 8a1 1 0 012 0v6a1 1 0 11-2 0V8zm5-1a1 1 0 00-1 1v6a1 1 0 102 0V8a1 1 0 00-1-1z" clipRule="evenodd" />
                    </svg>
                  </button>
                </div>

                {/* Date and optional time range */}
                <div className="grid grid-cols-3 gap-3">
                  <div>
                    <label className="block text-xs text-gray-500 mb-0.5">Date</label>
                    <input
                      type="date"
                      value={ov.date}
                      onChange={(e) => updateOverride(idx, { date: e.currentTarget.value })}
                      className="block w-full rounded-md border border-gray-300 px-3 py-1.5 text-sm text-gray-900 focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
                    />
                  </div>
                  <div>
                    <label className="block text-xs text-gray-500 mb-0.5">
                      Start Time <span className="text-gray-400">(optional)</span>
                    </label>
                    <input
                      type="time"
                      value={ov.start ?? ''}
                      onChange={(e) => updateOverride(idx, { start: e.currentTarget.value })}
                      className="block w-full rounded-md border border-gray-300 px-3 py-1.5 text-sm text-gray-900 focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
                    />
                  </div>
                  <div>
                    <label className="block text-xs text-gray-500 mb-0.5">
                      End Time <span className="text-gray-400">(optional)</span>
                    </label>
                    <input
                      type="time"
                      value={ov.end ?? ''}
                      onChange={(e) => updateOverride(idx, { end: e.currentTarget.value })}
                      className="block w-full rounded-md border border-gray-300 px-3 py-1.5 text-sm text-gray-900 focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
                    />
                  </div>
                </div>

                {/* All-day indicator */}
                {!ov.start && !ov.end && ov.date && (
                  <div className="flex items-center gap-1.5 text-xs text-amber-600">
                    <svg className="w-3.5 h-3.5" viewBox="0 0 20 20" fill="currentColor">
                      <path fillRule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm1-12a1 1 0 10-2 0v4a1 1 0 00.293.707l2.828 2.829a1 1 0 101.415-1.415L11 9.586V6z" clipRule="evenodd" />
                    </svg>
                    All-day override — applies for the entire date
                  </div>
                )}

                {/* Destination */}
                <div>
                  <label className="block text-xs text-gray-500 mb-0.5">Destination</label>
                  <input
                    type="text"
                    value={ov.dest_node}
                    onChange={(e) => updateOverride(idx, { dest_node: e.currentTarget.value })}
                    placeholder="Node ID — the flow node to route to on this date"
                    className="block w-full rounded-md border border-gray-300 px-3 py-1.5 text-sm text-gray-900 placeholder-gray-400 focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
                  />
                </div>
              </div>
            ))}
          </div>
        )}

        {/* Add override button */}
        <div className="border-t border-gray-200 bg-gray-50 px-3 py-2">
          <button
            type="button"
            onClick={addOverride}
            className="inline-flex items-center gap-1.5 text-sm text-amber-600 hover:text-amber-800 transition-colors"
          >
            <svg className="w-4 h-4" viewBox="0 0 20 20" fill="currentColor">
              <path fillRule="evenodd" d="M10 3a1 1 0 011 1v5h5a1 1 0 110 2h-5v5a1 1 0 11-2 0v-5H4a1 1 0 110-2h5V4a1 1 0 011-1z" clipRule="evenodd" />
            </svg>
            Add Override
          </button>
        </div>
      </div>
    </div>
  )
}
