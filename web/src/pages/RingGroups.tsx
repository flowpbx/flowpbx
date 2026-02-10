import { useState, useEffect, type FormEvent } from 'react'
import { listRingGroups, createRingGroup, updateRingGroup, deleteRingGroup, listExtensions, ApiError } from '../api'
import type { RingGroup, RingGroupRequest, Extension } from '../api'
import DataTable, { type Column } from '../components/DataTable'
import { TextInput, NumberInput, SelectField } from '../components/FormFields'

const STRATEGY_LABELS: Record<string, string> = {
  ring_all: 'Ring All',
  round_robin: 'Round Robin',
  random: 'Random',
  longest_idle: 'Longest Idle',
}

export default function RingGroups() {
  const [groups, setGroups] = useState<RingGroup[]>([])
  const [loading, setLoading] = useState(true)
  const [editing, setEditing] = useState<RingGroup | null>(null)
  const [creating, setCreating] = useState(false)
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)
  const [extensions, setExtensions] = useState<Extension[]>([])

  const [form, setForm] = useState<RingGroupRequest>(emptyForm())

  function emptyForm(): RingGroupRequest {
    return {
      name: '',
      strategy: 'ring_all',
      ring_timeout: 30,
      members: [],
      caller_id_mode: 'pass',
    }
  }

  function load() {
    setLoading(true)
    listRingGroups()
      .then((res) => setGroups(res))
      .catch(() => setGroups([]))
      .finally(() => setLoading(false))
  }

  function loadExtensions() {
    listExtensions({ limit: 100, offset: 0 })
      .then((res) => setExtensions(res.items))
      .catch(() => setExtensions([]))
  }

  useEffect(() => {
    load()
    loadExtensions()
  }, [])

  function openCreate() {
    setForm(emptyForm())
    setEditing(null)
    setCreating(true)
    setError('')
  }

  function openEdit(rg: RingGroup) {
    setForm({
      name: rg.name,
      strategy: rg.strategy,
      ring_timeout: rg.ring_timeout,
      members: rg.members ?? [],
      caller_id_mode: rg.caller_id_mode,
    })
    setEditing(rg)
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
        await updateRingGroup(editing.id, form)
      } else {
        await createRingGroup(form)
      }
      closeForm()
      load()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'unable to save ring group')
    } finally {
      setSaving(false)
    }
  }

  async function handleDelete(rg: RingGroup) {
    if (!confirm(`Delete ring group "${rg.name}"?`)) return
    try {
      await deleteRingGroup(rg.id)
      load()
    } catch (err) {
      alert(err instanceof ApiError ? err.message : 'unable to delete ring group')
    }
  }

  function toggleMember(extId: number) {
    setForm((prev) => {
      const members = prev.members.includes(extId)
        ? prev.members.filter((id) => id !== extId)
        : [...prev.members, extId]
      return { ...prev, members }
    })
  }

  const columns: Column<RingGroup>[] = [
    { key: 'name', header: 'Name', render: (r) => r.name },
    {
      key: 'strategy',
      header: 'Strategy',
      render: (r) => (
        <span className="inline-flex items-center rounded-full bg-blue-50 px-2 py-0.5 text-xs font-medium text-blue-700">
          {STRATEGY_LABELS[r.strategy] ?? r.strategy}
        </span>
      ),
    },
    { key: 'ring_timeout', header: 'Timeout', render: (r) => `${r.ring_timeout}s` },
    {
      key: 'members',
      header: 'Members',
      render: (r) => {
        const count = r.members?.length ?? 0
        return <span className="text-gray-600">{count} extension{count !== 1 ? 's' : ''}</span>
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
            {editing ? 'Edit Ring Group' : 'New Ring Group'}
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
            label="Group Name"
            id="rg_name"
            required
            value={form.name}
            onChange={(e) => setForm({ ...form, name: e.currentTarget.value })}
            placeholder="Sales Team"
          />

          <div className="grid grid-cols-2 gap-4">
            <SelectField
              label="Ring Strategy"
              id="rg_strategy"
              value={form.strategy ?? 'ring_all'}
              onChange={(e) => setForm({ ...form, strategy: e.currentTarget.value })}
            >
              <option value="ring_all">Ring All</option>
              <option value="round_robin">Round Robin</option>
              <option value="random">Random</option>
              <option value="longest_idle">Longest Idle</option>
            </SelectField>

            <NumberInput
              label="Ring Timeout (s)"
              id="rg_timeout"
              min={5}
              max={300}
              value={form.ring_timeout ?? 30}
              onChange={(e) => setForm({ ...form, ring_timeout: Number(e.currentTarget.value) })}
            />
          </div>

          <SelectField
            label="Caller ID Mode"
            id="rg_caller_id"
            value={form.caller_id_mode ?? 'pass'}
            onChange={(e) => setForm({ ...form, caller_id_mode: e.currentTarget.value })}
          >
            <option value="pass">Pass Through</option>
            <option value="prepend">Prepend Group Name</option>
          </SelectField>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-2">Members</label>
            {extensions.length === 0 ? (
              <p className="text-sm text-gray-400">No extensions available.</p>
            ) : (
              <div className="border border-gray-200 rounded-md max-h-48 overflow-y-auto">
                {extensions.map((ext) => (
                  <label
                    key={ext.id}
                    className="flex items-center gap-3 px-3 py-2 hover:bg-gray-50 cursor-pointer border-b border-gray-100 last:border-b-0"
                  >
                    <input
                      type="checkbox"
                      checked={form.members.includes(ext.id)}
                      onChange={() => toggleMember(ext.id)}
                      className="h-4 w-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500"
                    />
                    <span className="text-sm text-gray-900">
                      {ext.extension} — {ext.name}
                    </span>
                  </label>
                ))}
              </div>
            )}
            {form.members.length > 0 && (
              <p className="mt-1 text-xs text-gray-500">
                {form.members.length} member{form.members.length !== 1 ? 's' : ''} selected
              </p>
            )}
          </div>

          <div className="pt-4 border-t border-gray-100">
            <button
              type="submit"
              disabled={saving}
              className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
            >
              {saving ? 'Saving...' : editing ? 'Update Ring Group' : 'Create Ring Group'}
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
          <h1 className="text-2xl font-bold text-gray-900">Ring Groups</h1>
          <p className="mt-1 text-sm text-gray-500">Manage ring groups for simultaneous or sequential ringing.</p>
        </div>
        <button
          type="button"
          onClick={openCreate}
          className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 transition-colors"
        >
          Add Ring Group
        </button>
      </div>

      {loading ? (
        <p className="text-sm text-gray-400">Loading...</p>
      ) : (
        <DataTable
          columns={columns}
          rows={groups}
          keyFn={(r) => r.id}
          total={groups.length}
          limit={groups.length || 1}
          offset={0}
          onPageChange={() => {}}
          onRowClick={openEdit}
          emptyMessage="No ring groups configured yet."
        />
      )}
    </div>
  )
}
