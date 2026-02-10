import { useState, useEffect, useRef, type FormEvent } from 'react'
import {
  listVoicemailBoxes, createVoicemailBox, updateVoicemailBox, deleteVoicemailBox,
  listVoicemailMessages, deleteVoicemailMessage, markVoicemailMessageRead, voicemailAudioURL,
  ApiError,
} from '../api'
import type { VoicemailBox, VoicemailBoxRequest, VoicemailMessage } from '../api'
import DataTable, { type Column } from '../components/DataTable'
import { TextInput, NumberInput, SelectField, Toggle } from '../components/FormFields'

const PAGE_SIZE = 20

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString(undefined, {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}

function formatDuration(seconds: number): string {
  const m = Math.floor(seconds / 60)
  const s = seconds % 60
  return m > 0 ? `${m}m ${s}s` : `${s}s`
}

/** Message browser for a single voicemail box. */
function MessageBrowser({ box, onBack }: { box: VoicemailBox; onBack: () => void }) {
  const [messages, setMessages] = useState<VoicemailMessage[]>([])
  const [loading, setLoading] = useState(true)
  const [playingId, setPlayingId] = useState<number | null>(null)
  const audioRef = useRef<HTMLAudioElement | null>(null)

  function load() {
    setLoading(true)
    listVoicemailMessages(box.id)
      .then((items) => setMessages(items))
      .catch(() => setMessages([]))
      .finally(() => setLoading(false))
  }

  useEffect(() => {
    load()
    return () => stopPlayback()
  }, [box.id])

  function togglePlay(msg: VoicemailMessage) {
    if (playingId === msg.id) {
      stopPlayback()
      return
    }

    stopPlayback()
    const audio = new Audio(voicemailAudioURL(box.id, msg.id))
    audio.addEventListener('ended', () => setPlayingId(null))
    audio.addEventListener('error', () => setPlayingId(null))
    audio.play()
    audioRef.current = audio
    setPlayingId(msg.id)
  }

  function stopPlayback() {
    if (audioRef.current) {
      audioRef.current.pause()
      audioRef.current = null
    }
    setPlayingId(null)
  }

  async function handleMarkRead(msg: VoicemailMessage) {
    try {
      const updated = await markVoicemailMessageRead(box.id, msg.id)
      setMessages((prev) => prev.map((m) => (m.id === msg.id ? updated : m)))
    } catch (err) {
      alert(err instanceof ApiError ? err.message : 'unable to mark message as read')
    }
  }

  async function handleDelete(msg: VoicemailMessage) {
    if (!confirm('Delete this voicemail message?')) return
    stopPlayback()
    try {
      await deleteVoicemailMessage(box.id, msg.id)
      setMessages((prev) => prev.filter((m) => m.id !== msg.id))
    } catch (err) {
      alert(err instanceof ApiError ? err.message : 'unable to delete message')
    }
  }

  const columns: Column<VoicemailMessage>[] = [
    {
      key: 'status',
      header: '',
      className: 'w-8',
      render: (r) => (
        <span className={`inline-block h-2 w-2 rounded-full ${r.read ? 'bg-gray-300' : 'bg-blue-500'}`} title={r.read ? 'Read' : 'Unread'} />
      ),
    },
    {
      key: 'caller',
      header: 'Caller',
      render: (r) => (
        <div>
          <span className={`${r.read ? 'text-gray-700' : 'font-medium text-gray-900'}`}>
            {r.caller_id_name || 'Unknown'}
          </span>
          {r.caller_id_num && (
            <span className="ml-2 text-gray-400 text-xs">{r.caller_id_num}</span>
          )}
        </div>
      ),
    },
    { key: 'timestamp', header: 'Date', render: (r) => formatDate(r.timestamp) },
    { key: 'duration', header: 'Duration', render: (r) => formatDuration(r.duration) },
    {
      key: 'actions',
      header: '',
      className: 'w-48',
      render: (r) => (
        <div className="flex gap-2">
          <button
            type="button"
            onClick={(e) => { e.stopPropagation(); togglePlay(r) }}
            className="text-sm text-blue-600 hover:text-blue-800"
          >
            {playingId === r.id ? 'Stop' : 'Play'}
          </button>
          <a
            href={voicemailAudioURL(box.id, r.id)}
            download
            onClick={(e) => e.stopPropagation()}
            className="text-sm text-blue-600 hover:text-blue-800"
          >
            Download
          </a>
          {!r.read && (
            <button
              type="button"
              onClick={(e) => { e.stopPropagation(); handleMarkRead(r) }}
              className="text-sm text-gray-500 hover:text-gray-700"
            >
              Mark Read
            </button>
          )}
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

  const unreadCount = messages.filter((m) => !m.read).length

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <div>
          <div className="flex items-center gap-3">
            <button
              type="button"
              onClick={onBack}
              className="text-sm text-gray-500 hover:text-gray-700"
            >
              &larr; Back
            </button>
            <h1 className="text-2xl font-bold text-gray-900">{box.name}</h1>
          </div>
          <p className="mt-1 text-sm text-gray-500">
            {messages.length} message{messages.length !== 1 ? 's' : ''}
            {unreadCount > 0 && <span className="ml-1 text-blue-600">({unreadCount} unread)</span>}
          </p>
        </div>
      </div>

      {loading ? (
        <p className="text-sm text-gray-400">Loading messages...</p>
      ) : (
        <DataTable
          columns={columns}
          rows={messages}
          keyFn={(r) => r.id}
          total={messages.length}
          limit={messages.length || 1}
          offset={0}
          onPageChange={() => {}}
          emptyMessage="No voicemail messages in this box."
        />
      )}
    </div>
  )
}

export default function VoicemailBoxes() {
  const [boxes, setBoxes] = useState<VoicemailBox[]>([])
  const [total, setTotal] = useState(0)
  const [offset, setOffset] = useState(0)
  const [loading, setLoading] = useState(true)
  const [editing, setEditing] = useState<VoicemailBox | null>(null)
  const [creating, setCreating] = useState(false)
  const [browsing, setBrowsing] = useState<VoicemailBox | null>(null)
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)

  const [form, setForm] = useState<VoicemailBoxRequest>(emptyForm())

  function emptyForm(): VoicemailBoxRequest {
    return {
      name: '',
      mailbox_number: '',
      pin: '',
      greeting_type: 'default',
      email_notify: false,
      email_address: '',
      email_attach_audio: true,
      max_message_duration: 120,
      max_messages: 50,
      retention_days: 90,
      notify_extension_id: null,
    }
  }

  function load(newOffset: number) {
    setLoading(true)
    listVoicemailBoxes({ limit: PAGE_SIZE, offset: newOffset })
      .then((res) => {
        setBoxes(res.items)
        setTotal(res.total)
        setOffset(newOffset)
      })
      .catch(() => {
        setBoxes([])
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

  function openEdit(box: VoicemailBox) {
    setForm({
      name: box.name,
      mailbox_number: box.mailbox_number,
      pin: '',
      greeting_type: box.greeting_type,
      email_notify: box.email_notify,
      email_address: box.email_address,
      email_attach_audio: box.email_attach_audio,
      max_message_duration: box.max_message_duration,
      max_messages: box.max_messages,
      retention_days: box.retention_days,
      notify_extension_id: box.notify_extension_id,
    })
    setEditing(box)
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
        await updateVoicemailBox(editing.id, form)
      } else {
        await createVoicemailBox(form)
      }
      closeForm()
      load(editing ? offset : 0)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'unable to save voicemail box')
    } finally {
      setSaving(false)
    }
  }

  async function handleDelete(box: VoicemailBox) {
    if (!confirm(`Delete voicemail box "${box.name}"?`)) return
    try {
      await deleteVoicemailBox(box.id)
      load(offset)
    } catch (err) {
      alert(err instanceof ApiError ? err.message : 'unable to delete voicemail box')
    }
  }

  const columns: Column<VoicemailBox>[] = [
    { key: 'name', header: 'Name', render: (r) => r.name },
    { key: 'mailbox_number', header: 'Mailbox #', render: (r) => r.mailbox_number || '—' },
    { key: 'greeting_type', header: 'Greeting', render: (r) => r.greeting_type },
    {
      key: 'email_notify',
      header: 'Email Notify',
      render: (r) => (
        <span className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${r.email_notify ? 'bg-green-50 text-green-700' : 'bg-gray-100 text-gray-500'}`}>
          {r.email_notify ? 'On' : 'Off'}
        </span>
      ),
    },
    { key: 'max_messages', header: 'Max Msgs', render: (r) => r.max_messages },
    {
      key: 'retention',
      header: 'Retention',
      render: (r) => r.retention_days === 0 ? 'Forever' : `${r.retention_days}d`,
    },
    {
      key: 'actions',
      header: '',
      className: 'w-36',
      render: (r) => (
        <div className="flex gap-2">
          <button
            type="button"
            onClick={(e) => { e.stopPropagation(); setBrowsing(r) }}
            className="text-sm text-blue-600 hover:text-blue-800"
          >
            Messages
          </button>
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

  // Message browser view
  if (browsing) {
    return <MessageBrowser box={browsing} onBack={() => setBrowsing(null)} />
  }

  // Create/edit form view
  if (creating) {
    return (
      <div>
        <div className="flex items-center justify-between mb-6">
          <h1 className="text-2xl font-bold text-gray-900">
            {editing ? 'Edit Voicemail Box' : 'New Voicemail Box'}
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
            label="Name"
            id="vm_name"
            required
            value={form.name}
            onChange={(e) => setForm({ ...form, name: e.currentTarget.value })}
            placeholder="Sales Voicemail"
          />

          <div className="grid grid-cols-2 gap-4">
            <TextInput
              label="Mailbox Number (optional)"
              id="vm_number"
              value={form.mailbox_number ?? ''}
              onChange={(e) => setForm({ ...form, mailbox_number: e.currentTarget.value })}
              placeholder="901"
            />
            <TextInput
              label={editing ? 'PIN (leave blank to keep)' : 'PIN (optional)'}
              id="vm_pin"
              type="password"
              value={form.pin ?? ''}
              onChange={(e) => setForm({ ...form, pin: e.currentTarget.value })}
              autoComplete="off"
            />
          </div>

          <SelectField
            label="Greeting Type"
            id="vm_greeting_type"
            value={form.greeting_type ?? 'default'}
            onChange={(e) => setForm({ ...form, greeting_type: e.currentTarget.value })}
          >
            <option value="default">Default</option>
            <option value="custom">Custom</option>
            <option value="name_only">Name Only</option>
          </SelectField>

          <div className="space-y-3">
            <Toggle
              label="Email Notifications"
              checked={form.email_notify ?? false}
              onChange={(v) => setForm({ ...form, email_notify: v })}
            />
            {form.email_notify && (
              <>
                <TextInput
                  label="Notification Email"
                  id="vm_email"
                  type="email"
                  value={form.email_address ?? ''}
                  onChange={(e) => setForm({ ...form, email_address: e.currentTarget.value })}
                  placeholder="user@example.com"
                />
                <Toggle
                  label="Attach Audio to Email"
                  checked={form.email_attach_audio ?? true}
                  onChange={(v) => setForm({ ...form, email_attach_audio: v })}
                />
              </>
            )}
          </div>

          <div className="grid grid-cols-3 gap-4">
            <NumberInput
              label="Max Duration (s)"
              id="vm_max_duration"
              min={10}
              max={600}
              value={form.max_message_duration ?? 120}
              onChange={(e) => setForm({ ...form, max_message_duration: Number(e.currentTarget.value) })}
            />
            <NumberInput
              label="Max Messages"
              id="vm_max_messages"
              min={1}
              max={999}
              value={form.max_messages ?? 50}
              onChange={(e) => setForm({ ...form, max_messages: Number(e.currentTarget.value) })}
            />
            <NumberInput
              label="Retention (days)"
              id="vm_retention"
              min={0}
              value={form.retention_days ?? 90}
              onChange={(e) => setForm({ ...form, retention_days: Number(e.currentTarget.value) })}
            />
          </div>
          <p className="text-xs text-gray-400">Set retention to 0 for no automatic deletion.</p>

          <div className="pt-4 border-t border-gray-100">
            <button
              type="submit"
              disabled={saving}
              className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
            >
              {saving ? 'Saving...' : editing ? 'Update Voicemail Box' : 'Create Voicemail Box'}
            </button>
          </div>
        </form>
      </div>
    )
  }

  // Main list view
  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Voicemail Boxes</h1>
          <p className="mt-1 text-sm text-gray-500">Manage voicemail boxes and message settings.</p>
        </div>
        <button
          type="button"
          onClick={openCreate}
          className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 transition-colors"
        >
          Add Voicemail Box
        </button>
      </div>

      {loading ? (
        <p className="text-sm text-gray-400">Loading...</p>
      ) : (
        <DataTable
          columns={columns}
          rows={boxes}
          keyFn={(r) => r.id}
          total={total}
          limit={PAGE_SIZE}
          offset={offset}
          onPageChange={load}
          onRowClick={openEdit}
          emptyMessage="No voicemail boxes configured yet."
        />
      )}
    </div>
  )
}
