import { useState, useEffect, useRef } from 'react'
import {
  listExtensions,
  listVoicemailBoxes,
  listInboundNumbers,
} from '../../api'

interface EntityOption {
  id: number
  label: string
  sublabel?: string
}

interface Props {
  entityType: string
  selectedId: number | null
  onSelect: (id: number | null) => void
  onCreateNew?: () => void
}

/** Entity type display labels. */
const ENTITY_LABELS: Record<string, string> = {
  extension: 'Extension',
  voicemail_box: 'Voicemail Box',
  inbound_number: 'Inbound Number',
  ring_group: 'Ring Group',
  ivr_menu: 'IVR Menu',
  time_switch: 'Time Switch',
  conference: 'Conference',
}

/** Searchable dropdown for selecting an entity with a "Create new" option. */
export default function EntitySelector({ entityType, selectedId, onSelect, onCreateNew }: Props) {
  const [options, setOptions] = useState<EntityOption[]>([])
  const [loading, setLoading] = useState(false)
  const [open, setOpen] = useState(false)
  const [search, setSearch] = useState('')
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    loadOptions()
  }, [entityType])

  // Close on click outside
  useEffect(() => {
    function handleClickOutside(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as HTMLElement)) {
        setOpen(false)
      }
    }
    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [])

  async function loadOptions() {
    setLoading(true)
    try {
      const opts = await fetchEntities(entityType)
      setOptions(opts)
    } catch {
      setOptions([])
    } finally {
      setLoading(false)
    }
  }

  const selected = options.find((o) => o.id === selectedId)
  const filtered = options.filter(
    (o) =>
      o.label.toLowerCase().includes(search.toLowerCase()) ||
      o.sublabel?.toLowerCase().includes(search.toLowerCase()),
  )
  const label = ENTITY_LABELS[entityType] ?? entityType

  return (
    <div ref={ref} className="relative">
      <label className="block text-sm font-medium text-gray-700 mb-1">{label}</label>
      <button
        type="button"
        onClick={() => setOpen(!open)}
        className="w-full flex items-center justify-between rounded-md border border-gray-300 bg-white px-3 py-2 text-sm text-left focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
      >
        <span className={selected ? 'text-gray-900' : 'text-gray-400'}>
          {loading ? 'Loading...' : selected ? selected.label : `Select ${label}...`}
        </span>
        <svg className="w-4 h-4 text-gray-400" viewBox="0 0 20 20" fill="currentColor">
          <path fillRule="evenodd" d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z" clipRule="evenodd" />
        </svg>
      </button>

      {open && (
        <div className="absolute z-50 mt-1 w-full bg-white border border-gray-200 rounded-md shadow-lg max-h-60 flex flex-col">
          {/* Search */}
          <div className="p-2 border-b border-gray-100">
            <input
              type="text"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Search..."
              className="w-full text-sm rounded border border-gray-200 px-2 py-1 focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
              autoFocus
            />
          </div>

          {/* Create new option */}
          {onCreateNew && (
            <button
              type="button"
              onClick={() => {
                onCreateNew()
                setOpen(false)
              }}
              className="flex items-center gap-2 px-3 py-2 text-sm text-blue-600 hover:bg-blue-50 border-b border-gray-100"
            >
              <svg className="w-4 h-4" viewBox="0 0 20 20" fill="currentColor">
                <path fillRule="evenodd" d="M10 3a1 1 0 011 1v5h5a1 1 0 110 2h-5v5a1 1 0 11-2 0v-5H4a1 1 0 110-2h5V4a1 1 0 011-1z" clipRule="evenodd" />
              </svg>
              New {label}...
            </button>
          )}

          {/* Options list */}
          <div className="flex-1 overflow-y-auto">
            {/* None option */}
            <button
              type="button"
              onClick={() => {
                onSelect(null)
                setOpen(false)
              }}
              className="w-full text-left px-3 py-2 text-sm text-gray-400 hover:bg-gray-50"
            >
              None
            </button>

            {filtered.map((opt) => (
              <button
                key={opt.id}
                type="button"
                onClick={() => {
                  onSelect(opt.id)
                  setOpen(false)
                  setSearch('')
                }}
                className={`w-full text-left px-3 py-2 text-sm hover:bg-gray-50 ${
                  opt.id === selectedId ? 'bg-blue-50 text-blue-700' : 'text-gray-900'
                }`}
              >
                <div className="font-medium">{opt.label}</div>
                {opt.sublabel && <div className="text-xs text-gray-500">{opt.sublabel}</div>}
              </button>
            ))}

            {filtered.length === 0 && !loading && (
              <div className="px-3 py-2 text-sm text-gray-400">No results</div>
            )}
          </div>
        </div>
      )}
    </div>
  )
}

/** Fetch entity options by type from the API. */
async function fetchEntities(entityType: string): Promise<EntityOption[]> {
  switch (entityType) {
    case 'extension': {
      const res = await listExtensions({ limit: 200 })
      return res.items.map((e) => ({
        id: e.id,
        label: `${e.extension} - ${e.name}`,
        sublabel: e.email || undefined,
      }))
    }
    case 'voicemail_box': {
      const res = await listVoicemailBoxes({ limit: 200 })
      return res.items.map((v) => ({
        id: v.id,
        label: `${v.mailbox_number} - ${v.name}`,
        sublabel: v.email_address || undefined,
      }))
    }
    case 'inbound_number': {
      const res = await listInboundNumbers({ limit: 200 })
      return res.items.map((n) => ({
        id: n.id,
        label: n.number,
        sublabel: n.name || undefined,
      }))
    }
    // For entity types without dedicated API endpoints yet, return empty.
    case 'ring_group':
    case 'ivr_menu':
    case 'time_switch':
    case 'conference':
      return []
    default:
      return []
  }
}
