import { useState, useEffect, type FormEvent } from 'react'
import { listIVRMenus, createIVRMenu, updateIVRMenu, deleteIVRMenu, ApiError } from '../api'
import type { IVRMenu, IVRMenuRequest } from '../api'
import DataTable, { type Column } from '../components/DataTable'
import { TextInput, NumberInput } from '../components/FormFields'
import DigitMappingEditor from '../components/DigitMappingEditor'
import PromptSelector from '../components/PromptSelector'

export default function IVRMenus() {
  const [menus, setMenus] = useState<IVRMenu[]>([])
  const [loading, setLoading] = useState(true)
  const [editing, setEditing] = useState<IVRMenu | null>(null)
  const [creating, setCreating] = useState(false)
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)

  const [form, setForm] = useState<IVRMenuRequest>(emptyForm())
  const [greetingType, setGreetingType] = useState<'none' | 'tts' | 'audio'>('none')

  function emptyForm(): IVRMenuRequest {
    return {
      name: '',
      greeting_file: '',
      greeting_tts: '',
      timeout: 10,
      max_retries: 3,
      digit_timeout: 3,
      options: {},
    }
  }

  function load() {
    setLoading(true)
    listIVRMenus()
      .then((res) => setMenus(res))
      .catch(() => setMenus([]))
      .finally(() => setLoading(false))
  }

  useEffect(() => {
    load()
  }, [])

  function openCreate() {
    setForm(emptyForm())
    setGreetingType('none')
    setEditing(null)
    setCreating(true)
    setError('')
  }

  function openEdit(ivr: IVRMenu) {
    setForm({
      name: ivr.name,
      greeting_file: ivr.greeting_file ?? '',
      greeting_tts: ivr.greeting_tts ?? '',
      timeout: ivr.timeout,
      max_retries: ivr.max_retries,
      digit_timeout: ivr.digit_timeout,
      options: ivr.options ?? {},
    })
    setGreetingType(ivr.greeting_file ? 'audio' : ivr.greeting_tts ? 'tts' : 'none')
    setEditing(ivr)
    setCreating(true)
    setError('')
  }

  function closeForm() {
    setCreating(false)
    setEditing(null)
    setError('')
  }

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setError('')
    setSaving(true)

    try {
      if (editing) {
        await updateIVRMenu(editing.id, form)
      } else {
        await createIVRMenu(form)
      }
      closeForm()
      load()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'unable to save ivr menu')
    } finally {
      setSaving(false)
    }
  }

  async function handleDelete(ivr: IVRMenu) {
    if (!confirm(`Delete IVR menu "${ivr.name}"?`)) return
    try {
      await deleteIVRMenu(ivr.id)
      load()
    } catch (err) {
      alert(err instanceof ApiError ? err.message : 'unable to delete ivr menu')
    }
  }

  function setOption(key: string, value: string) {
    setForm((prev) => {
      const opts = { ...prev.options }
      if (value === '') {
        delete opts[key]
      } else {
        opts[key] = value
      }
      return { ...prev, options: opts }
    })
  }

  function countOptions(options: Record<string, string> | undefined): number {
    if (!options) return 0
    return Object.keys(options).length
  }

  const columns: Column<IVRMenu>[] = [
    { key: 'name', header: 'Name', render: (r) => r.name },
    {
      key: 'timeout',
      header: 'Timeout',
      render: (r) => `${r.timeout}s`,
    },
    {
      key: 'max_retries',
      header: 'Max Retries',
      render: (r) => String(r.max_retries),
    },
    {
      key: 'options',
      header: 'Options',
      render: (r) => {
        const count = countOptions(r.options)
        return (
          <span className="text-gray-600">
            {count} mapping{count !== 1 ? 's' : ''}
          </span>
        )
      },
    },
    {
      key: 'greeting',
      header: 'Greeting',
      render: (r) => {
        if (r.greeting_file) {
          return <span className="inline-flex items-center rounded-full bg-green-50 px-2 py-0.5 text-xs font-medium text-green-700">Audio</span>
        }
        if (r.greeting_tts) {
          return <span className="inline-flex items-center rounded-full bg-blue-50 px-2 py-0.5 text-xs font-medium text-blue-700">TTS</span>
        }
        return <span className="text-gray-400">None</span>
      },
    },
    {
      key: 'actions',
      header: '',
      className: 'w-24',
      render: (r) => (
        <div className="flex gap-2">
          <button
            type="button"
            onClick={(e) => { e.stopPropagation(); openEdit(r) }}
            className="text-sm text-blue-600 hover:text-blue-800"
          >
            Edit
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

  if (creating) {
    return (
      <div>
        <div className="flex items-center justify-between mb-6">
          <h1 className="text-2xl font-bold text-gray-900">
            {editing ? 'Edit IVR Menu' : 'New IVR Menu'}
          </h1>
          <button
            type="button"
            onClick={closeForm}
            className="text-sm text-gray-500 hover:text-gray-700"
          >
            Cancel
          </button>
        </div>

        <form onSubmit={handleSubmit} className="max-w-lg space-y-4">
          {error && (
            <div className="rounded-md bg-red-50 border border-red-200 px-3 py-2">
              <p className="text-sm text-red-700">{error}</p>
            </div>
          )}

          <TextInput
            label="Menu Name"
            id="ivr_name"
            required
            value={form.name}
            onChange={(e) => setForm({ ...form, name: e.currentTarget.value })}
            placeholder="Main Menu"
          />

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Greeting</label>
            <div className="flex gap-1 mb-2">
              {(['none', 'tts', 'audio'] as const).map((t) => (
                <button
                  key={t}
                  type="button"
                  onClick={() => {
                    setGreetingType(t)
                    if (t !== 'audio') setForm((f) => ({ ...f, greeting_file: '' }))
                    if (t !== 'tts') setForm((f) => ({ ...f, greeting_tts: '' }))
                  }}
                  className={`px-3 py-1 text-xs font-medium rounded-md border transition-colors ${
                    greetingType === t
                      ? 'bg-blue-600 text-white border-blue-600'
                      : 'bg-white text-gray-700 border-gray-300 hover:bg-gray-50'
                  }`}
                >
                  {t === 'none' ? 'None' : t === 'tts' ? 'TTS Text' : 'Audio File'}
                </button>
              ))}
            </div>

            {greetingType === 'tts' && (
              <TextInput
                label=""
                id="ivr_greeting_tts"
                value={form.greeting_tts ?? ''}
                onChange={(e) => setForm({ ...form, greeting_tts: e.currentTarget.value })}
                placeholder="Press 1 for Sales, Press 2 for Support..."
              />
            )}

            {greetingType === 'audio' && (
              <PromptSelector
                value={form.greeting_file ?? ''}
                onChange={(filename) => setForm({ ...form, greeting_file: filename })}
              />
            )}
          </div>

          <div className="grid grid-cols-3 gap-4">
            <NumberInput
              label="Timeout (s)"
              id="ivr_timeout"
              min={1}
              max={60}
              value={form.timeout ?? 10}
              onChange={(e) => setForm({ ...form, timeout: Number(e.currentTarget.value) })}
            />

            <NumberInput
              label="Max Retries"
              id="ivr_max_retries"
              min={0}
              max={10}
              value={form.max_retries ?? 3}
              onChange={(e) => setForm({ ...form, max_retries: Number(e.currentTarget.value) })}
            />

            <NumberInput
              label="Digit Timeout (s)"
              id="ivr_digit_timeout"
              min={1}
              max={30}
              value={form.digit_timeout ?? 3}
              onChange={(e) => setForm({ ...form, digit_timeout: Number(e.currentTarget.value) })}
            />
          </div>

          <DigitMappingEditor
            options={form.options}
            onChange={setOption}
          />

          <div className="pt-4 border-t border-gray-100">
            <button
              type="submit"
              disabled={saving}
              className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
            >
              {saving ? 'Saving...' : editing ? 'Update IVR Menu' : 'Create IVR Menu'}
            </button>
          </div>
        </form>
      </div>
    )
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">IVR Menus</h1>
          <p className="mt-1 text-sm text-gray-500">Manage interactive voice response menus for caller self-service.</p>
        </div>
        <button
          type="button"
          onClick={openCreate}
          className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 transition-colors"
        >
          Add IVR Menu
        </button>
      </div>

      {loading ? (
        <p className="text-sm text-gray-400">Loading...</p>
      ) : (
        <DataTable
          columns={columns}
          rows={menus}
          keyFn={(r) => r.id}
          total={menus.length}
          limit={menus.length || 1}
          offset={0}
          onPageChange={() => {}}
          onRowClick={openEdit}
          emptyMessage="No IVR menus configured yet."
        />
      )}
    </div>
  )
}
