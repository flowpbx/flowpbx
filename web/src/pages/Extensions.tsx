import { useState, useEffect, type FormEvent } from 'react'
import { listExtensions, createExtension, updateExtension, deleteExtension, ApiError } from '../api'
import type { Extension, ExtensionRequest } from '../api'
import DataTable, { type Column } from '../components/DataTable'
import { TextInput, NumberInput, SelectField, Toggle } from '../components/FormFields'

const PAGE_SIZE = 20

export default function Extensions() {
  const [extensions, setExtensions] = useState<Extension[]>([])
  const [total, setTotal] = useState(0)
  const [offset, setOffset] = useState(0)
  const [loading, setLoading] = useState(true)
  const [editing, setEditing] = useState<Extension | null>(null)
  const [creating, setCreating] = useState(false)
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)

  const [form, setForm] = useState<ExtensionRequest>(emptyForm())

  function emptyForm(): ExtensionRequest {
    return {
      extension: '',
      name: '',
      email: '',
      sip_username: '',
      sip_password: '',
      ring_timeout: 30,
      dnd: false,
      follow_me_enabled: false,
      recording_mode: 'off',
      max_registrations: 5,
    }
  }

  function load(newOffset: number) {
    setLoading(true)
    listExtensions({ limit: PAGE_SIZE, offset: newOffset })
      .then((res) => {
        setExtensions(res.items)
        setTotal(res.total)
        setOffset(newOffset)
      })
      .catch(() => {
        setExtensions([])
        setTotal(0)
      })
      .finally(() => setLoading(false))
  }

  useEffect(() => {
    load(0)
  }, [])

  function openCreate() {
    setForm(emptyForm())
    setEditing(null)
    setCreating(true)
    setError('')
  }

  function openEdit(ext: Extension) {
    setForm({
      extension: ext.extension,
      name: ext.name,
      email: ext.email,
      sip_username: ext.sip_username,
      sip_password: '',
      ring_timeout: ext.ring_timeout,
      dnd: ext.dnd,
      follow_me_enabled: ext.follow_me_enabled,
      recording_mode: ext.recording_mode,
      max_registrations: ext.max_registrations,
    })
    setEditing(ext)
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
        await updateExtension(editing.id, form)
      } else {
        await createExtension(form)
      }
      closeForm()
      load(editing ? offset : 0)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'unable to save extension')
    } finally {
      setSaving(false)
    }
  }

  async function handleDelete(ext: Extension) {
    if (!confirm(`Delete extension ${ext.extension} (${ext.name})?`)) return
    try {
      await deleteExtension(ext.id)
      load(offset)
    } catch (err) {
      alert(err instanceof ApiError ? err.message : 'unable to delete extension')
    }
  }

  const columns: Column<Extension>[] = [
    { key: 'extension', header: 'Extension', render: (r) => r.extension },
    { key: 'name', header: 'Name', render: (r) => r.name },
    { key: 'email', header: 'Email', render: (r) => r.email || '—' },
    { key: 'sip_username', header: 'SIP Username', render: (r) => r.sip_username },
    {
      key: 'dnd',
      header: 'DND',
      render: (r) => (
        <span className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${r.dnd ? 'bg-red-50 text-red-700' : 'bg-gray-100 text-gray-500'}`}>
          {r.dnd ? 'On' : 'Off'}
        </span>
      ),
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
            {editing ? 'Edit Extension' : 'New Extension'}
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

          <div className="grid grid-cols-2 gap-4">
            <TextInput
              label="Extension Number"
              id="ext_number"
              required
              value={form.extension}
              onChange={(e) => setForm({ ...form, extension: e.currentTarget.value })}
              placeholder="101"
            />
            <TextInput
              label="Display Name"
              id="ext_name"
              required
              value={form.name}
              onChange={(e) => setForm({ ...form, name: e.currentTarget.value })}
              placeholder="John Smith"
            />
          </div>

          <TextInput
            label="Email"
            id="ext_email"
            type="email"
            value={form.email ?? ''}
            onChange={(e) => setForm({ ...form, email: e.currentTarget.value })}
            placeholder="john@example.com"
          />

          <div className="grid grid-cols-2 gap-4">
            <TextInput
              label="SIP Username"
              id="sip_username"
              required
              value={form.sip_username}
              onChange={(e) => setForm({ ...form, sip_username: e.currentTarget.value })}
            />
            <TextInput
              label={editing ? 'SIP Password (leave blank to keep)' : 'SIP Password'}
              id="sip_password"
              type="password"
              required={!editing}
              value={form.sip_password ?? ''}
              onChange={(e) => setForm({ ...form, sip_password: e.currentTarget.value })}
              autoComplete="off"
            />
          </div>

          <div className="grid grid-cols-2 gap-4">
            <NumberInput
              label="Ring Timeout (s)"
              id="ring_timeout"
              min={5}
              max={300}
              value={form.ring_timeout ?? 30}
              onChange={(e) => setForm({ ...form, ring_timeout: Number(e.currentTarget.value) })}
            />
            <NumberInput
              label="Max Registrations"
              id="max_registrations"
              min={1}
              max={20}
              value={form.max_registrations ?? 5}
              onChange={(e) => setForm({ ...form, max_registrations: Number(e.currentTarget.value) })}
            />
          </div>

          <SelectField
            label="Recording Mode"
            id="recording_mode"
            value={form.recording_mode ?? 'off'}
            onChange={(e) => setForm({ ...form, recording_mode: e.currentTarget.value })}
          >
            <option value="off">Off</option>
            <option value="always">Always</option>
            <option value="on_demand">On Demand</option>
          </SelectField>

          <div className="flex gap-6">
            <Toggle
              label="Do Not Disturb"
              checked={form.dnd ?? false}
              onChange={(v) => setForm({ ...form, dnd: v })}
            />
            <Toggle
              label="Follow Me"
              checked={form.follow_me_enabled ?? false}
              onChange={(v) => setForm({ ...form, follow_me_enabled: v })}
            />
          </div>

          <div className="pt-4 border-t border-gray-100">
            <button
              type="submit"
              disabled={saving}
              className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
            >
              {saving ? 'Saving...' : editing ? 'Update Extension' : 'Create Extension'}
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
          <h1 className="text-2xl font-bold text-gray-900">Extensions</h1>
          <p className="mt-1 text-sm text-gray-500">Manage SIP extensions and user accounts.</p>
        </div>
        <button
          type="button"
          onClick={openCreate}
          className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 transition-colors"
        >
          Add Extension
        </button>
      </div>

      {loading ? (
        <p className="text-sm text-gray-400">Loading...</p>
      ) : (
        <DataTable
          columns={columns}
          rows={extensions}
          keyFn={(r) => r.id}
          total={total}
          limit={PAGE_SIZE}
          offset={offset}
          onPageChange={load}
          onRowClick={openEdit}
          emptyMessage="No extensions configured yet."
        />
      )}
    </div>
  )
}
