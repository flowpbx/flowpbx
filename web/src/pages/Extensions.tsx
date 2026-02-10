import { useState, useEffect, type FormEvent } from 'react'
import { listExtensions, createExtension, updateExtension, deleteExtension, ApiError } from '../api'
import type { Extension, ExtensionRequest, FollowMeNumber } from '../api'
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
      follow_me_numbers: [],
      follow_me_strategy: 'sequential',
      follow_me_confirm: false,
      recording_mode: 'off',
      max_registrations: 5,
    }
  }

  function addFollowMeNumber() {
    const nums = [...(form.follow_me_numbers ?? [])]
    nums.push({ number: '', delay: 0, timeout: 20 })
    setForm({ ...form, follow_me_numbers: nums })
  }

  function updateFollowMeNumber(idx: number, field: keyof FollowMeNumber, value: string | number) {
    const nums = [...(form.follow_me_numbers ?? [])]
    nums[idx] = { ...nums[idx], [field]: value }
    setForm({ ...form, follow_me_numbers: nums })
  }

  function removeFollowMeNumber(idx: number) {
    const nums = (form.follow_me_numbers ?? []).filter((_, i) => i !== idx)
    setForm({ ...form, follow_me_numbers: nums })
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
      follow_me_numbers: ext.follow_me_numbers ?? [],
      follow_me_strategy: ext.follow_me_strategy || 'sequential',
      follow_me_confirm: ext.follow_me_confirm ?? false,
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
      key: 'follow_me',
      header: 'Follow Me',
      render: (r) => (
        <span className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${r.follow_me_enabled ? 'bg-green-50 text-green-700' : 'bg-gray-100 text-gray-500'}`}>
          {r.follow_me_enabled ? 'On' : 'Off'}
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

          {form.follow_me_enabled && (
            <div className="rounded-md border border-gray-200 p-4 space-y-4">
              <h3 className="text-sm font-medium text-gray-900">Follow-Me Configuration</h3>

              <div className="grid grid-cols-2 gap-4">
                <SelectField
                  label="Ring Strategy"
                  id="follow_me_strategy"
                  value={form.follow_me_strategy ?? 'sequential'}
                  onChange={(e) => setForm({ ...form, follow_me_strategy: e.currentTarget.value })}
                >
                  <option value="sequential">Sequential</option>
                  <option value="simultaneous">Simultaneous</option>
                </SelectField>
                <div className="flex items-end pb-2">
                  <Toggle
                    label="Require confirmation (Press 1)"
                    checked={form.follow_me_confirm ?? false}
                    onChange={(v) => setForm({ ...form, follow_me_confirm: v })}
                  />
                </div>
              </div>

              <div>
                <div className="flex items-center justify-between mb-2">
                  <label className="block text-sm font-medium text-gray-700">External Numbers</label>
                  <button
                    type="button"
                    onClick={addFollowMeNumber}
                    className="text-sm text-blue-600 hover:text-blue-800"
                  >
                    + Add Number
                  </button>
                </div>

                {(form.follow_me_numbers ?? []).length === 0 ? (
                  <p className="text-sm text-gray-400">No external numbers configured.</p>
                ) : (
                  <div className="space-y-2">
                    {(form.follow_me_numbers ?? []).map((num, idx) => (
                      <div key={idx} className="flex items-end gap-2">
                        <div className="flex-1">
                          <label className="block text-xs text-gray-500 mb-0.5">Number</label>
                          <input
                            type="tel"
                            className="block w-full rounded-md border border-gray-300 px-2 py-1.5 text-sm text-gray-900 placeholder-gray-400 focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
                            placeholder="0412345678"
                            value={num.number}
                            onChange={(e) => updateFollowMeNumber(idx, 'number', e.currentTarget.value)}
                          />
                        </div>
                        <div className="w-20">
                          <label className="block text-xs text-gray-500 mb-0.5">Delay (s)</label>
                          <input
                            type="number"
                            min={0}
                            max={300}
                            className="block w-full rounded-md border border-gray-300 px-2 py-1.5 text-sm text-gray-900 focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
                            value={num.delay}
                            onChange={(e) => updateFollowMeNumber(idx, 'delay', Number(e.currentTarget.value))}
                          />
                        </div>
                        <div className="w-24">
                          <label className="block text-xs text-gray-500 mb-0.5">Timeout (s)</label>
                          <input
                            type="number"
                            min={1}
                            max={300}
                            className="block w-full rounded-md border border-gray-300 px-2 py-1.5 text-sm text-gray-900 focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
                            value={num.timeout}
                            onChange={(e) => updateFollowMeNumber(idx, 'timeout', Number(e.currentTarget.value))}
                          />
                        </div>
                        <button
                          type="button"
                          onClick={() => removeFollowMeNumber(idx)}
                          className="mb-0.5 rounded p-1.5 text-gray-400 hover:text-red-600 hover:bg-red-50"
                          title="Remove number"
                        >
                          <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                            <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
                          </svg>
                        </button>
                      </div>
                    ))}
                  </div>
                )}
                {form.follow_me_strategy === 'sequential' && (form.follow_me_numbers ?? []).length > 0 && (
                  <p className="mt-2 text-xs text-gray-500">
                    Numbers are tried in order. Delay controls when each number starts ringing after the call begins.
                  </p>
                )}
                {form.follow_me_strategy === 'simultaneous' && (form.follow_me_numbers ?? []).length > 0 && (
                  <p className="mt-2 text-xs text-gray-500">
                    All numbers ring at the same time. First to answer wins.
                  </p>
                )}
              </div>
            </div>
          )}

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
