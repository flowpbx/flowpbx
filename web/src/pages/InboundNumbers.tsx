import { useState, useEffect, type FormEvent } from 'react'
import { listInboundNumbers, createInboundNumber, updateInboundNumber, deleteInboundNumber, listTrunks, ApiError } from '../api'
import type { InboundNumber, InboundNumberRequest, Trunk } from '../api'
import DataTable, { type Column } from '../components/DataTable'
import { TextInput, NumberInput, SelectField, Toggle } from '../components/FormFields'

const PAGE_SIZE = 20

export default function InboundNumbers() {
  const [numbers, setNumbers] = useState<InboundNumber[]>([])
  const [total, setTotal] = useState(0)
  const [offset, setOffset] = useState(0)
  const [loading, setLoading] = useState(true)
  const [editing, setEditing] = useState<InboundNumber | null>(null)
  const [creating, setCreating] = useState(false)
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)

  const [trunks, setTrunks] = useState<Trunk[]>([])

  const [form, setForm] = useState<InboundNumberRequest>(emptyForm())

  function emptyForm(): InboundNumberRequest {
    return {
      number: '',
      name: '',
      trunk_id: null,
      flow_id: null,
      flow_entry_node: '',
      enabled: true,
    }
  }

  function load(newOffset: number) {
    setLoading(true)
    listInboundNumbers({ limit: PAGE_SIZE, offset: newOffset })
      .then((res) => {
        setNumbers(res.items)
        setTotal(res.total)
        setOffset(newOffset)
      })
      .catch(() => {
        setNumbers([])
        setTotal(0)
      })
      .finally(() => setLoading(false))
  }

  useEffect(() => {
    load(0)
    listTrunks({ limit: 100 })
      .then((res) => setTrunks(res.items))
      .catch(() => {})
  }, [])

  function openCreate() {
    setForm(emptyForm())
    setEditing(null)
    setCreating(true)
    setError('')
  }

  function openEdit(num: InboundNumber) {
    setForm({
      number: num.number,
      name: num.name,
      trunk_id: num.trunk_id,
      flow_id: num.flow_id,
      flow_entry_node: num.flow_entry_node,
      enabled: num.enabled,
    })
    setEditing(num)
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
        await updateInboundNumber(editing.id, form)
      } else {
        await createInboundNumber(form)
      }
      closeForm()
      load(editing ? offset : 0)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'unable to save inbound number')
    } finally {
      setSaving(false)
    }
  }

  async function handleDelete(num: InboundNumber) {
    if (!confirm(`Delete inbound number ${num.number} (${num.name})?`)) return
    try {
      await deleteInboundNumber(num.id)
      load(offset)
    } catch (err) {
      alert(err instanceof ApiError ? err.message : 'unable to delete inbound number')
    }
  }

  function trunkName(trunkId: number | null): string {
    if (trunkId == null) return '—'
    const t = trunks.find((tr) => tr.id === trunkId)
    return t ? t.name : `#${trunkId}`
  }

  const columns: Column<InboundNumber>[] = [
    { key: 'number', header: 'Number', render: (r) => r.number },
    { key: 'name', header: 'Name', render: (r) => r.name || '—' },
    { key: 'trunk_id', header: 'Trunk', render: (r) => trunkName(r.trunk_id) },
    {
      key: 'enabled',
      header: 'Status',
      render: (r) => (
        <span className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${r.enabled ? 'bg-green-50 text-green-700' : 'bg-gray-100 text-gray-500'}`}>
          {r.enabled ? 'Enabled' : 'Disabled'}
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
            {editing ? 'Edit Inbound Number' : 'New Inbound Number'}
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
              label="Number"
              id="number"
              required
              value={form.number}
              onChange={(e) => setForm({ ...form, number: e.currentTarget.value })}
              placeholder="+14155551234"
            />
            <TextInput
              label="Name"
              id="name"
              value={form.name}
              onChange={(e) => setForm({ ...form, name: e.currentTarget.value })}
              placeholder="Main Office"
            />
          </div>

          <SelectField
            label="Trunk"
            id="trunk_id"
            value={form.trunk_id ?? ''}
            onChange={(e) => setForm({ ...form, trunk_id: e.currentTarget.value ? Number(e.currentTarget.value) : null })}
          >
            <option value="">None</option>
            {trunks.map((t) => (
              <option key={t.id} value={t.id}>{t.name}</option>
            ))}
          </SelectField>

          <div className="grid grid-cols-2 gap-4">
            <NumberInput
              label="Flow ID"
              id="flow_id"
              min={0}
              value={form.flow_id ?? ''}
              onChange={(e) => setForm({ ...form, flow_id: e.currentTarget.value ? Number(e.currentTarget.value) : null })}
            />
            <TextInput
              label="Flow Entry Node"
              id="flow_entry_node"
              value={form.flow_entry_node ?? ''}
              onChange={(e) => setForm({ ...form, flow_entry_node: e.currentTarget.value })}
              placeholder="start"
            />
          </div>
          <p className="text-xs text-gray-400">Optional: route this number to a specific call flow and entry node.</p>

          <Toggle
            label="Enabled"
            checked={form.enabled ?? true}
            onChange={(v) => setForm({ ...form, enabled: v })}
          />

          <div className="pt-4 border-t border-gray-100">
            <button
              type="submit"
              disabled={saving}
              className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
            >
              {saving ? 'Saving...' : editing ? 'Update Number' : 'Create Number'}
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
          <h1 className="text-2xl font-bold text-gray-900">Inbound Numbers</h1>
          <p className="mt-1 text-sm text-gray-500">Manage DID numbers and route them to call flows.</p>
        </div>
        <button
          type="button"
          onClick={openCreate}
          className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 transition-colors"
        >
          Add Number
        </button>
      </div>

      {loading ? (
        <p className="text-sm text-gray-400">Loading...</p>
      ) : (
        <DataTable
          columns={columns}
          rows={numbers}
          keyFn={(r) => r.id}
          total={total}
          limit={PAGE_SIZE}
          offset={offset}
          onPageChange={load}
          onRowClick={openEdit}
          emptyMessage="No inbound numbers configured yet."
        />
      )}
    </div>
  )
}
