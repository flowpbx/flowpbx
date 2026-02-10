/**
 * IVR digit mapping editor — visual keypad-style grid for mapping
 * DTMF digits (0-9, *, #) plus timeout and invalid to destinations.
 */

const KEYPAD_ROWS = [
  ['1', '2', '3'],
  ['4', '5', '6'],
  ['7', '8', '9'],
  ['*', '0', '#'],
] as const

const SPECIAL_KEYS = [
  { key: 't', label: 'Timeout', description: 'No input received within the timeout period' },
  { key: 'i', label: 'Invalid', description: 'Caller pressed an unmapped digit (after max retries)' },
] as const

interface Props {
  options: Record<string, string>
  onChange: (key: string, value: string) => void
}

export default function DigitMappingEditor({ options, onChange }: Props) {
  const mappedCount = Object.keys(options).filter((k) => options[k]).length

  return (
    <div>
      <label className="block text-sm font-medium text-gray-700 mb-1">Digit Mappings</label>
      <p className="text-xs text-gray-500 mb-3">
        Assign a destination for each digit callers can press. Leave blank to ignore that digit.
      </p>

      {/* Keypad grid */}
      <div className="border border-gray-200 rounded-lg overflow-hidden">
        <div className="bg-gray-50 px-3 py-2 border-b border-gray-200">
          <div className="flex items-center justify-between">
            <span className="text-xs font-medium text-gray-500 uppercase tracking-wide">Keypad Digits</span>
            <span className="text-xs text-gray-400">
              {mappedCount} mapping{mappedCount !== 1 ? 's' : ''} configured
            </span>
          </div>
        </div>

        <div className="p-3 space-y-2">
          {KEYPAD_ROWS.map((row, ri) => (
            <div key={ri} className="grid grid-cols-3 gap-2">
              {row.map((digit) => (
                <DigitRow
                  key={digit}
                  label={digit}
                  value={options[digit] ?? ''}
                  onChange={(v) => onChange(digit, v)}
                />
              ))}
            </div>
          ))}
        </div>

        {/* Special keys */}
        <div className="border-t border-gray-200 bg-gray-50 px-3 py-2">
          <span className="text-xs font-medium text-gray-500 uppercase tracking-wide">Special Handlers</span>
        </div>
        <div className="p-3 space-y-2">
          {SPECIAL_KEYS.map(({ key, label, description }) => (
            <SpecialRow
              key={key}
              digitKey={key}
              label={label}
              description={description}
              value={options[key] ?? ''}
              onChange={(v) => onChange(key, v)}
            />
          ))}
        </div>
      </div>
    </div>
  )
}

/** Single keypad digit row — compact with inline input. */
function DigitRow({
  label,
  value,
  onChange,
}: {
  label: string
  value: string
  onChange: (value: string) => void
}) {
  const hasValue = value.length > 0

  return (
    <div className={`flex items-center gap-2 rounded-md border px-2 py-1.5 transition-colors ${
      hasValue ? 'border-blue-200 bg-blue-50/50' : 'border-gray-200 bg-white'
    }`}>
      <span className={`w-7 h-7 flex items-center justify-center rounded text-sm font-semibold shrink-0 ${
        hasValue ? 'bg-blue-600 text-white' : 'bg-gray-100 text-gray-500'
      }`}>
        {label}
      </span>
      <input
        type="text"
        value={value}
        onChange={(e) => onChange(e.currentTarget.value)}
        placeholder="destination"
        className="flex-1 min-w-0 bg-transparent text-sm text-gray-900 placeholder-gray-400 focus:outline-none"
      />
      {hasValue && (
        <button
          type="button"
          onClick={() => onChange('')}
          className="shrink-0 text-gray-400 hover:text-gray-600"
          title="Clear"
        >
          <svg className="w-3.5 h-3.5" viewBox="0 0 20 20" fill="currentColor">
            <path fillRule="evenodd" d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z" clipRule="evenodd" />
          </svg>
        </button>
      )}
    </div>
  )
}

/** Special handler row — wider layout with description text. */
function SpecialRow({
  digitKey,
  label,
  description,
  value,
  onChange,
}: {
  digitKey: string
  label: string
  description: string
  value: string
  onChange: (value: string) => void
}) {
  const hasValue = value.length > 0

  return (
    <div className={`flex items-center gap-3 rounded-md border px-3 py-2 transition-colors ${
      hasValue ? 'border-amber-200 bg-amber-50/50' : 'border-gray-200 bg-white'
    }`}>
      <div className="shrink-0 w-24">
        <span className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${
          hasValue
            ? digitKey === 't' ? 'bg-amber-100 text-amber-700' : 'bg-red-100 text-red-700'
            : 'bg-gray-100 text-gray-500'
        }`}>
          {label}
        </span>
        <p className="text-[10px] text-gray-400 mt-0.5 leading-tight">{description}</p>
      </div>
      <input
        type="text"
        value={value}
        onChange={(e) => onChange(e.currentTarget.value)}
        placeholder="destination"
        className="flex-1 min-w-0 rounded-md border border-gray-300 bg-white px-2 py-1 text-sm text-gray-900 placeholder-gray-400 focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
      />
      {hasValue && (
        <button
          type="button"
          onClick={() => onChange('')}
          className="shrink-0 text-gray-400 hover:text-gray-600"
          title="Clear"
        >
          <svg className="w-3.5 h-3.5" viewBox="0 0 20 20" fill="currentColor">
            <path fillRule="evenodd" d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z" clipRule="evenodd" />
          </svg>
        </button>
      )}
    </div>
  )
}
