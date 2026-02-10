import { useState, useEffect, type FormEvent } from 'react'
import { listTrunks, createTrunk, updateTrunk, deleteTrunk, ApiError } from '../api'
import type { Trunk, TrunkRequest } from '../api'
import DataTable, { type Column } from '../components/DataTable'
import { TextInput, NumberInput, SelectField, Toggle } from '../components/FormFields'

const PAGE_SIZE = 20

export default function Trunks() {
  const [trunks, setTrunks] = useState<Trunk[]>([])
  const [total, setTotal] = useState(0)
  const [offset, setOffset] = useState(0)
  const [loading, setLoading] = useState(true)
  const [editing, setEditing] = useState<Trunk | null>(null)
  const [creating, setCreating] = useState(false)
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)

  const [form, setForm] = useState<TrunkRequest>(emptyForm())

  function emptyForm(): TrunkRequest {
    return {
      name: '',
      type: 'register',
      enabled: true,
      host: '',
      port: 5060,
      transport: 'udp',
      username: '',
      password: '',
      auth_username: '',
      register_expiry: 300,
      remote_hosts: '',
      local_host: '',
      codecs: '',
      max_channels: 0,
      caller_id_name: '',
      caller_id_num: '',
      prefix_strip: 0,
      prefix_add: '',
      priority: 10,
    }
  }

  function load(newOffset: number) {
    setLoading(true)
    listTrunks({ limit: PAGE_SIZE, offset: newOffset })
      .then((res) => {
        setTrunks(res.items)
        setTotal(res.total)
        setOffset(newOffset)
      })
      .catch(() => {
        setTrunks([])
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

  function openEdit(trunk: Trunk) {
    setForm({
      name: trunk.name,
      type: trunk.type,
      enabled: trunk.enabled,
      host: trunk.host,
      port: trunk.port,
      transport: trunk.transport,
      username: trunk.username,
      password: '',
      auth_username: trunk.auth_username,
      register_expiry: trunk.register_expiry,
      remote_hosts: trunk.remote_hosts,
      local_host: trunk.local_host,
      codecs: trunk.codecs,
      max_channels: trunk.max_channels,
      caller_id_name: trunk.caller_id_name,
      caller_id_num: trunk.caller_id_num,
      prefix_strip: trunk.prefix_strip,
      prefix_add: trunk.prefix_add,
      priority: trunk.priority,
    })
    setEditing(trunk)
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
        await updateTrunk(editing.id, form)
      } else {
        await createTrunk(form)
      }
      closeForm()
      load(editing ? offset : 0)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'unable to save trunk')
    } finally {
      setSaving(false)
    }
  }

  async function handleDelete(trunk: Trunk) {
    if (!confirm(`Delete trunk "${trunk.name}"?`)) return
    try {
      await deleteTrunk(trunk.id)
      load(offset)
    } catch (err) {
      alert(err instanceof ApiError ? err.message : 'unable to delete trunk')
    }
  }

  const columns: Column<Trunk>[] = [
    { key: 'name', header: 'Name', render: (r) => r.name },
    { key: 'type', header: 'Type', render: (r) => r.type === 'register' ? 'Registration' : 'IP Auth' },
    {
      key: 'host',
      header: 'Host',
      render: (r) => r.type === 'register' ? `${r.host}:${r.port}` : r.remote_hosts || '—',
    },
    { key: 'transport', header: 'Transport', render: (r) => (r.transport || 'udp').toUpperCase() },
    {
      key: 'enabled',
      header: 'Status',
      render: (r) => (
        <div className="flex items-center gap-1.5">
          <span className={`inline-block w-2 h-2 rounded-full ${r.enabled ? 'bg-green-500' : 'bg-gray-300'}`} />
          <span className="text-sm">{r.enabled ? 'Enabled' : 'Disabled'}</span>
        </div>
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
    const isRegister = form.type === 'register'

    return (
      <div>
        <div className="flex items-center justify-between mb-6">
          <h1 className="text-2xl font-bold text-gray-900">
            {editing ? 'Edit Trunk' : 'New Trunk'}
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
            label="Trunk Name"
            id="trunk_name"
            required
            value={form.name}
            onChange={(e) => setForm({ ...form, name: e.currentTarget.value })}
            placeholder="My SIP Provider"
          />

          <div className="grid grid-cols-2 gap-4">
            <SelectField
              label="Type"
              id="trunk_type"
              value={form.type}
              onChange={(e) => setForm({ ...form, type: e.currentTarget.value })}
            >
              <option value="register">Registration</option>
              <option value="ip">IP Auth</option>
            </SelectField>
            <SelectField
              label="Transport"
              id="trunk_transport"
              value={form.transport ?? 'udp'}
              onChange={(e) => setForm({ ...form, transport: e.currentTarget.value })}
            >
              <option value="udp">UDP</option>
              <option value="tcp">TCP</option>
              <option value="tls">TLS</option>
            </SelectField>
          </div>

          {isRegister ? (
            <>
              <div className="grid grid-cols-2 gap-4">
                <TextInput
                  label="Host"
                  id="trunk_host"
                  required
                  value={form.host ?? ''}
                  onChange={(e) => setForm({ ...form, host: e.currentTarget.value })}
                  placeholder="sip.provider.com"
                />
                <NumberInput
                  label="Port"
                  id="trunk_port"
                  min={1}
                  max={65535}
                  value={form.port ?? 5060}
                  onChange={(e) => setForm({ ...form, port: Number(e.currentTarget.value) })}
                />
              </div>
              <div className="grid grid-cols-2 gap-4">
                <TextInput
                  label="Username"
                  id="trunk_username"
                  value={form.username ?? ''}
                  onChange={(e) => setForm({ ...form, username: e.currentTarget.value })}
                  autoComplete="off"
                />
                <TextInput
                  label={editing ? 'Password (leave blank to keep)' : 'Password'}
                  id="trunk_password"
                  type="password"
                  value={form.password ?? ''}
                  onChange={(e) => setForm({ ...form, password: e.currentTarget.value })}
                  autoComplete="off"
                />
              </div>
              <TextInput
                label="Auth Username (if different)"
                id="trunk_auth_username"
                value={form.auth_username ?? ''}
                onChange={(e) => setForm({ ...form, auth_username: e.currentTarget.value })}
                autoComplete="off"
              />
              <NumberInput
                label="Register Expiry (s)"
                id="trunk_expiry"
                min={60}
                max={3600}
                value={form.register_expiry ?? 300}
                onChange={(e) => setForm({ ...form, register_expiry: Number(e.currentTarget.value) })}
              />
            </>
          ) : (
            <>
              <TextInput
                label="Remote Hosts (comma-separated IPs/CIDRs)"
                id="trunk_remote_hosts"
                value={form.remote_hosts ?? ''}
                onChange={(e) => setForm({ ...form, remote_hosts: e.currentTarget.value })}
                placeholder="203.0.113.10, 198.51.100.0/24"
              />
              <TextInput
                label="Local Bind Address (optional)"
                id="trunk_local_host"
                value={form.local_host ?? ''}
                onChange={(e) => setForm({ ...form, local_host: e.currentTarget.value })}
              />
            </>
          )}

          <div className="grid grid-cols-2 gap-4">
            <TextInput
              label="Caller ID Name"
              id="trunk_cid_name"
              value={form.caller_id_name ?? ''}
              onChange={(e) => setForm({ ...form, caller_id_name: e.currentTarget.value })}
            />
            <TextInput
              label="Caller ID Number"
              id="trunk_cid_num"
              value={form.caller_id_num ?? ''}
              onChange={(e) => setForm({ ...form, caller_id_num: e.currentTarget.value })}
            />
          </div>

          <div className="grid grid-cols-3 gap-4">
            <NumberInput
              label="Max Channels"
              id="trunk_max_channels"
              min={0}
              value={form.max_channels ?? 0}
              onChange={(e) => setForm({ ...form, max_channels: Number(e.currentTarget.value) })}
            />
            <NumberInput
              label="Priority"
              id="trunk_priority"
              min={1}
              max={100}
              value={form.priority ?? 10}
              onChange={(e) => setForm({ ...form, priority: Number(e.currentTarget.value) })}
            />
            <NumberInput
              label="Prefix Strip"
              id="trunk_prefix_strip"
              min={0}
              max={10}
              value={form.prefix_strip ?? 0}
              onChange={(e) => setForm({ ...form, prefix_strip: Number(e.currentTarget.value) })}
            />
          </div>

          <TextInput
            label="Prefix Add"
            id="trunk_prefix_add"
            value={form.prefix_add ?? ''}
            onChange={(e) => setForm({ ...form, prefix_add: e.currentTarget.value })}
          />

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
              {saving ? 'Saving...' : editing ? 'Update Trunk' : 'Create Trunk'}
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
          <h1 className="text-2xl font-bold text-gray-900">Trunks</h1>
          <p className="mt-1 text-sm text-gray-500">Manage SIP trunk connections to upstream providers.</p>
        </div>
        <button
          type="button"
          onClick={openCreate}
          className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 transition-colors"
        >
          Add Trunk
        </button>
      </div>

      {loading ? (
        <p className="text-sm text-gray-400">Loading...</p>
      ) : (
        <DataTable
          columns={columns}
          rows={trunks}
          keyFn={(r) => r.id}
          total={total}
          limit={PAGE_SIZE}
          offset={offset}
          onPageChange={load}
          onRowClick={openEdit}
          emptyMessage="No trunks configured yet."
        />
      )}
    </div>
  )
}
