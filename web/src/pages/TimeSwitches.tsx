import { useState, useEffect, type FormEvent } from 'react'
import { listTimeSwitches, createTimeSwitch, updateTimeSwitch, deleteTimeSwitch, ApiError } from '../api'
import type { TimeSwitch, TimeSwitchRequest, TimeSwitchRule } from '../api'
import DataTable, { type Column } from '../components/DataTable'
import { TextInput, SelectField } from '../components/FormFields'

const TIMEZONES = [
  { group: 'Australia', zones: ['Australia/Sydney', 'Australia/Melbourne', 'Australia/Brisbane', 'Australia/Perth', 'Australia/Adelaide'] },
  { group: 'Pacific', zones: ['Pacific/Auckland'] },
  { group: 'Asia', zones: ['Asia/Tokyo', 'Asia/Shanghai', 'Asia/Singapore', 'Asia/Kolkata'] },
  { group: 'Europe', zones: ['Europe/London', 'Europe/Berlin', 'Europe/Paris'] },
  { group: 'Americas', zones: ['America/New_York', 'America/Chicago', 'America/Denver', 'America/Los_Angeles'] },
  { group: 'Other', zones: ['UTC'] },
]

const ALL_DAYS = ['mon', 'tue', 'wed', 'thu', 'fri', 'sat', 'sun'] as const
const DAY_LABELS: Record<string, string> = {
  mon: 'Mon', tue: 'Tue', wed: 'Wed', thu: 'Thu', fri: 'Fri', sat: 'Sat', sun: 'Sun',
}

function emptyRule(): TimeSwitchRule {
  return { label: '', days: ['mon', 'tue', 'wed', 'thu', 'fri'], start: '08:30', end: '17:00', dest_node: '' }
}

export default function TimeSwitches() {
  const [switches, setSwitches] = useState<TimeSwitch[]>([])
  const [loading, setLoading] = useState(true)
  const [editing, setEditing] = useState<TimeSwitch | null>(null)
  const [creating, setCreating] = useState(false)
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)

  const [form, setForm] = useState<TimeSwitchRequest>(emptyForm())

  function emptyForm(): TimeSwitchRequest {
    return {
      name: '',
      timezone: 'Australia/Sydney',
      rules: [emptyRule()],
      default_dest: '',
    }
  }

  function load() {
    setLoading(true)
    listTimeSwitches()
      .then((res) => setSwitches(res))
      .catch(() => setSwitches([]))
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

  function openEdit(ts: TimeSwitch) {
    setForm({
      name: ts.name,
      timezone: ts.timezone,
      rules: ts.rules ?? [],
      default_dest: ts.default_dest ?? '',
    })
    setEditing(ts)
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
        await updateTimeSwitch(editing.id, form)
      } else {
        await createTimeSwitch(form)
      }
      closeForm()
      load()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'unable to save time switch')
    } finally {
      setSaving(false)
    }
  }

  async function handleDelete(ts: TimeSwitch) {
    if (!confirm(`Delete time switch "${ts.name}"?`)) return
    try {
      await deleteTimeSwitch(ts.id)
      load()
    } catch (err) {
      alert(err instanceof ApiError ? err.message : 'unable to delete time switch')
    }
  }

  function updateRule(index: number, patch: Partial<TimeSwitchRule>) {
    setForm((prev) => {
      const rules = prev.rules.map((rule, i) => (i === index ? { ...rule, ...patch } : rule))
      return { ...prev, rules }
    })
  }

  function toggleDay(ruleIndex: number, day: string) {
    setForm((prev) => {
      const rules = prev.rules.map((rule, i) => {
        if (i !== ruleIndex) return rule
        const days = rule.days.includes(day)
          ? rule.days.filter((d) => d !== day)
          : [...rule.days, day]
        return { ...rule, days }
      })
      return { ...prev, rules }
    })
  }

  function addRule() {
    setForm((prev) => ({ ...prev, rules: [...prev.rules, emptyRule()] }))
  }

  function removeRule(index: number) {
    setForm((prev) => ({
      ...prev,
      rules: prev.rules.filter((_, i) => i !== index),
    }))
  }

  const columns: Column<TimeSwitch>[] = [
    { key: 'name', header: 'Name', render: (r) => r.name },
    {
      key: 'timezone',
      header: 'Timezone',
      render: (r) => (
        <span className="inline-flex items-center rounded-full bg-blue-50 px-2 py-0.5 text-xs font-medium text-blue-700">
          {r.timezone}
        </span>
      ),
    },
    {
      key: 'rules',
      header: 'Rules',
      render: (r) => {
        const count = r.rules?.length ?? 0
        return <span className="text-gray-600">{count} rule{count !== 1 ? 's' : ''}</span>
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
            {editing ? 'Edit Time Switch' : 'New Time Switch'}
          </h1>
          <button
            type="button"
            onClick={closeForm}
            className="text-sm text-gray-500 hover:text-gray-700"
          >
            Cancel
          </button>
        </div>

        <form onSubmit={handleSubmit} className="max-w-2xl space-y-4">
          {error && (
            <div className="rounded-md bg-red-50 border border-red-200 px-3 py-2">
              <p className="text-sm text-red-700">{error}</p>
            </div>
          )}

          <TextInput
            label="Name"
            id="ts_name"
            required
            value={form.name}
            onChange={(e) => setForm({ ...form, name: e.currentTarget.value })}
            placeholder="Business Hours"
          />

          <div className="grid grid-cols-2 gap-4">
            <SelectField
              label="Timezone"
              id="ts_timezone"
              value={form.timezone ?? 'Australia/Sydney'}
              onChange={(e) => setForm({ ...form, timezone: e.currentTarget.value })}
            >
              {TIMEZONES.map((group) => (
                <optgroup key={group.group} label={group.group}>
                  {group.zones.map((tz) => (
                    <option key={tz} value={tz}>{tz}</option>
                  ))}
                </optgroup>
              ))}
            </SelectField>

            <TextInput
              label="Default Destination"
              id="ts_default_dest"
              value={form.default_dest ?? ''}
              onChange={(e) => setForm({ ...form, default_dest: e.currentTarget.value })}
              placeholder="Node ID for after-hours"
            />
          </div>

          {/* Rules Editor */}
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-2">
              Rules
              <span className="font-normal text-gray-400 ml-1">(evaluated top to bottom, first match wins)</span>
            </label>

            <div className="space-y-3">
              {form.rules.map((rule, idx) => (
                <div
                  key={idx}
                  className="border border-gray-200 rounded-md bg-white p-3 space-y-3"
                >
                  <div className="flex items-start justify-between gap-3">
                    <div className="flex items-center gap-2 min-w-0 flex-1">
                      <span className="flex-shrink-0 inline-flex items-center justify-center w-5 h-5 rounded bg-gray-100 text-xs font-medium text-gray-500">
                        {idx + 1}
                      </span>
                      <input
                        type="text"
                        value={rule.label}
                        onChange={(e) => updateRule(idx, { label: e.currentTarget.value })}
                        placeholder="Rule label"
                        className="block w-full rounded-md border border-gray-300 px-3 py-1.5 text-sm text-gray-900 placeholder-gray-400 focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
                      />
                    </div>
                    {form.rules.length > 1 && (
                      <button
                        type="button"
                        onClick={() => removeRule(idx)}
                        className="flex-shrink-0 text-gray-400 hover:text-red-500 transition-colors p-0.5"
                        title="Remove rule"
                      >
                        <svg className="w-4 h-4" viewBox="0 0 20 20" fill="currentColor">
                          <path fillRule="evenodd" d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z" clipRule="evenodd" />
                        </svg>
                      </button>
                    )}
                  </div>

                  {/* Day chips */}
                  <div className="flex flex-wrap gap-1.5">
                    {ALL_DAYS.map((day) => {
                      const active = rule.days.includes(day)
                      return (
                        <button
                          key={day}
                          type="button"
                          onClick={() => toggleDay(idx, day)}
                          className={`px-2.5 py-1 rounded text-xs font-medium transition-colors ${
                            active
                              ? 'bg-blue-600 text-white'
                              : 'bg-gray-100 text-gray-500 hover:bg-gray-200'
                          }`}
                        >
                          {DAY_LABELS[day]}
                        </button>
                      )
                    })}
                  </div>

                  {/* Time range + destination */}
                  <div className="grid grid-cols-3 gap-3">
                    <div>
                      <label className="block text-xs text-gray-500 mb-0.5">Start</label>
                      <input
                        type="time"
                        value={rule.start}
                        onChange={(e) => updateRule(idx, { start: e.currentTarget.value })}
                        className="block w-full rounded-md border border-gray-300 px-3 py-1.5 text-sm text-gray-900 focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
                      />
                    </div>
                    <div>
                      <label className="block text-xs text-gray-500 mb-0.5">End</label>
                      <input
                        type="time"
                        value={rule.end}
                        onChange={(e) => updateRule(idx, { end: e.currentTarget.value })}
                        className="block w-full rounded-md border border-gray-300 px-3 py-1.5 text-sm text-gray-900 focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
                      />
                    </div>
                    <div>
                      <label className="block text-xs text-gray-500 mb-0.5">Destination</label>
                      <input
                        type="text"
                        value={rule.dest_node}
                        onChange={(e) => updateRule(idx, { dest_node: e.currentTarget.value })}
                        placeholder="Node ID"
                        className="block w-full rounded-md border border-gray-300 px-3 py-1.5 text-sm text-gray-900 placeholder-gray-400 focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
                      />
                    </div>
                  </div>
                </div>
              ))}
            </div>

            <button
              type="button"
              onClick={addRule}
              className="mt-2 inline-flex items-center gap-1 text-sm text-blue-600 hover:text-blue-800 transition-colors"
            >
              <svg className="w-4 h-4" viewBox="0 0 20 20" fill="currentColor">
                <path fillRule="evenodd" d="M10 3a1 1 0 011 1v5h5a1 1 0 110 2h-5v5a1 1 0 11-2 0v-5H4a1 1 0 110-2h5V4a1 1 0 011-1z" clipRule="evenodd" />
              </svg>
              Add Rule
            </button>
          </div>

          <div className="pt-4 border-t border-gray-100">
            <button
              type="submit"
              disabled={saving}
              className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
            >
              {saving ? 'Saving...' : editing ? 'Update Time Switch' : 'Create Time Switch'}
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
          <h1 className="text-2xl font-bold text-gray-900">Time Switches</h1>
          <p className="mt-1 text-sm text-gray-500">Time-based routing rules for business hours, holidays, and schedules.</p>
        </div>
        <button
          type="button"
          onClick={openCreate}
          className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 transition-colors"
        >
          Add Time Switch
        </button>
      </div>

      {loading ? (
        <p className="text-sm text-gray-400">Loading...</p>
      ) : (
        <DataTable
          columns={columns}
          rows={switches}
          keyFn={(r) => r.id}
          total={switches.length}
          limit={switches.length || 1}
          offset={0}
          onPageChange={() => {}}
          onRowClick={openEdit}
          emptyMessage="No time switches configured yet."
        />
      )}
    </div>
  )
}
