import type { SelectHTMLAttributes } from 'react'

const labelClass = 'block text-sm font-medium text-gray-700 mb-1'
const inputClass =
  'block w-full rounded-md border border-gray-300 px-3 py-2 text-sm text-gray-900 placeholder-gray-400 focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500'

const TIMEZONE_GROUPS: { group: string; zones: string[] }[] = [
  {
    group: 'Australia',
    zones: [
      'Australia/Sydney',
      'Australia/Melbourne',
      'Australia/Brisbane',
      'Australia/Perth',
      'Australia/Adelaide',
      'Australia/Hobart',
      'Australia/Darwin',
      'Australia/Lord_Howe',
    ],
  },
  {
    group: 'Pacific',
    zones: [
      'Pacific/Auckland',
      'Pacific/Fiji',
      'Pacific/Guam',
      'Pacific/Honolulu',
      'Pacific/Noumea',
      'Pacific/Tongatapu',
    ],
  },
  {
    group: 'Asia',
    zones: [
      'Asia/Tokyo',
      'Asia/Shanghai',
      'Asia/Hong_Kong',
      'Asia/Singapore',
      'Asia/Kolkata',
      'Asia/Dubai',
      'Asia/Seoul',
      'Asia/Bangkok',
      'Asia/Jakarta',
      'Asia/Karachi',
      'Asia/Manila',
      'Asia/Taipei',
      'Asia/Kuala_Lumpur',
      'Asia/Riyadh',
      'Asia/Tehran',
      'Asia/Dhaka',
    ],
  },
  {
    group: 'Europe',
    zones: [
      'Europe/London',
      'Europe/Berlin',
      'Europe/Paris',
      'Europe/Amsterdam',
      'Europe/Rome',
      'Europe/Madrid',
      'Europe/Zurich',
      'Europe/Stockholm',
      'Europe/Vienna',
      'Europe/Warsaw',
      'Europe/Brussels',
      'Europe/Helsinki',
      'Europe/Lisbon',
      'Europe/Athens',
      'Europe/Bucharest',
      'Europe/Moscow',
      'Europe/Istanbul',
    ],
  },
  {
    group: 'Americas',
    zones: [
      'America/New_York',
      'America/Chicago',
      'America/Denver',
      'America/Los_Angeles',
      'America/Anchorage',
      'America/Phoenix',
      'America/Toronto',
      'America/Vancouver',
      'America/Mexico_City',
      'America/Bogota',
      'America/Lima',
      'America/Santiago',
      'America/Sao_Paulo',
      'America/Argentina/Buenos_Aires',
    ],
  },
  {
    group: 'Africa',
    zones: [
      'Africa/Cairo',
      'Africa/Johannesburg',
      'Africa/Lagos',
      'Africa/Nairobi',
      'Africa/Casablanca',
    ],
  },
  {
    group: 'Other',
    zones: ['UTC'],
  },
]

/** Format a zone ID for display: "Australia/Sydney" → "Sydney" */
function formatZone(zone: string): string {
  const parts = zone.split('/')
  const name = parts[parts.length - 1].replace(/_/g, ' ')
  return zone === 'UTC' ? 'UTC' : name
}

/** Reusable timezone selector with grouped IANA timezones. */
export default function TimezoneSelector({
  label = 'Timezone',
  id,
  ...props
}: { label?: string } & Omit<SelectHTMLAttributes<HTMLSelectElement>, 'children'>) {
  return (
    <div>
      <label htmlFor={id} className={labelClass}>
        {label}
      </label>
      <select id={id} className={inputClass} {...props}>
        {TIMEZONE_GROUPS.map((group) => (
          <optgroup key={group.group} label={group.group}>
            {group.zones.map((tz) => (
              <option key={tz} value={tz}>
                {formatZone(tz)}
              </option>
            ))}
          </optgroup>
        ))}
      </select>
    </div>
  )
}
