import { useState, useEffect, type FormEvent } from 'react'
import { listConferenceBridges, createConferenceBridge, updateConferenceBridge, deleteConferenceBridge, ApiError } from '../api'
import type { ConferenceBridge, ConferenceBridgeRequest } from '../api'
import DataTable, { type Column } from '../components/DataTable'
import { TextInput, NumberInput, Toggle } from '../components/FormFields'

export default function ConferenceBridges() {
  const [bridges, setBridges] = useState<ConferenceBridge[]>([])
  const [loading, setLoading] = useState(true)
  const [editing, setEditing] = useState<ConferenceBridge | null>(null)
  const [creating, setCreating] = useState(false)
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)

  const [form, setForm] = useState<ConferenceBridgeRequest>(emptyForm())

  function emptyForm(): ConferenceBridgeRequest {
    return {
      name: '',
      extension: '',
      pin: '',
      max_members: 10,
      record: false,
      mute_on_join: false,
      announce_joins: false,
    }
  }

  function load() {
    setLoading(true)
    listConferenceBridges()
      .then((res) => setBridges(res))
      .catch(() => setBridges([]))
      .finally(() => setLoading(false))
  }

  useEffect(() => {
    load()
  }, [])

  function openCreate() {
    setForm(emptyForm())
    setEditing(null)
    setCreating(true)
    setError('')
  }

  function openEdit(bridge: ConferenceBridge) {
    setForm({
      name: bridge.name,
      extension: bridge.extension,
      pin: bridge.pin,
      max_members: bridge.max_members,
      record: bridge.record,
      mute_on_join: bridge.mute_on_join,
      announce_joins: bridge.announce_joins,
    })
    setEditing(bridge)
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
        await updateConferenceBridge(editing.id, form)
      } else {
        await createConferenceBridge(form)
      }
      closeForm()
      load()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'unable to save conference bridge')
    } finally {
      setSaving(false)
    }
  }

  async function handleDelete(bridge: ConferenceBridge) {
    if (!confirm(`Delete conference bridge "${bridge.name}"?`)) return
    try {
      await deleteConferenceBridge(bridge.id)
      load()
    } catch (err) {
      alert(err instanceof ApiError ? err.message : 'unable to delete conference bridge')
    }
  }

  const columns: Column<ConferenceBridge>[] = [
    { key: 'name', header: 'Name', render: (b) => b.name },
    {
      key: 'extension',
      header: 'Extension',
      render: (b) => b.extension || <span className="text-gray-400">—</span>,
    },
    {
      key: 'max_members',
      header: 'Max Members',
      render: (b) => String(b.max_members),
    },
    {
      key: 'pin',
      header: 'PIN',
      render: (b) =>
        b.pin ? (
          <span className="inline-flex items-center rounded-full bg-green-50 px-2 py-0.5 text-xs font-medium text-green-700">
            Protected
          </span>
        ) : (
          <span className="text-gray-400">None</span>
        ),
    },
    {
      key: 'features',
      header: 'Features',
      render: (b) => {
        const features: string[] = []
        if (b.record) features.push('Record')
        if (b.mute_on_join) features.push('Mute on join')
        if (b.announce_joins) features.push('Announce')
        return features.length > 0 ? (
          <span className="text-xs text-gray-600">{features.join(', ')}</span>
        ) : (
          <span className="text-gray-400">—</span>
        )
      },
    },
    {
      key: 'actions',
      header: '',
      className: 'w-24',
      render: (b) => (
        <div className="flex gap-2">
          <button
            type="button"
            onClick={(e) => { e.stopPropagation(); openEdit(b) }}
            className="text-sm text-blue-600 hover:text-blue-800"
          >
            Edit
          </button>
          <button
            type="button"
            onClick={(e) => { e.stopPropagation(); handleDelete(b) }}
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
            {editing ? 'Edit Conference Bridge' : 'New Conference Bridge'}
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
            label="Bridge Name"
            id="cb_name"
            required
            value={form.name}
            onChange={(e) => setForm({ ...form, name: e.currentTarget.value })}
            placeholder="Team Standup"
          />

          <div className="grid grid-cols-2 gap-4">
            <TextInput
              label="Dial-in Extension"
              id="cb_extension"
              value={form.extension ?? ''}
              onChange={(e) => setForm({ ...form, extension: e.currentTarget.value })}
              placeholder="800"
            />

            <NumberInput
              label="Max Members"
              id="cb_max_members"
              min={2}
              max={100}
              value={form.max_members ?? 10}
              onChange={(e) => setForm({ ...form, max_members: Number(e.currentTarget.value) })}
            />
          </div>

          <TextInput
            label="Access PIN (optional)"
            id="cb_pin"
            value={form.pin ?? ''}
            onChange={(e) => setForm({ ...form, pin: e.currentTarget.value })}
            placeholder="1234"
          />

          <div className="space-y-3 pt-2">
            <Toggle
              label="Record conference"
              checked={form.record ?? false}
              onChange={(checked) => setForm({ ...form, record: checked })}
            />
            <Toggle
              label="Mute participants on join"
              checked={form.mute_on_join ?? false}
              onChange={(checked) => setForm({ ...form, mute_on_join: checked })}
            />
            <Toggle
              label="Announce joins/leaves"
              checked={form.announce_joins ?? false}
              onChange={(checked) => setForm({ ...form, announce_joins: checked })}
            />
          </div>

          <div className="pt-4 border-t border-gray-100">
            <button
              type="submit"
              disabled={saving}
              className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
            >
              {saving ? 'Saving...' : editing ? 'Update Conference Bridge' : 'Create Conference Bridge'}
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
          <h1 className="text-2xl font-bold text-gray-900">Conference Bridges</h1>
          <p className="mt-1 text-sm text-gray-500">Manage conference bridges for multi-party audio calls.</p>
        </div>
        <button
          type="button"
          onClick={openCreate}
          className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 transition-colors"
        >
          Add Conference Bridge
        </button>
      </div>

      {loading ? (
        <p className="text-sm text-gray-400">Loading...</p>
      ) : (
        <DataTable
          columns={columns}
          rows={bridges}
          keyFn={(b) => b.id}
          total={bridges.length}
          limit={bridges.length || 1}
          offset={0}
          onPageChange={() => {}}
          onRowClick={openEdit}
          emptyMessage="No conference bridges configured yet."
        />
      )}
    </div>
  )
}
