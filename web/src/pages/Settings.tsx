import { useState, useEffect, type FormEvent } from 'react'
import { getSettings, updateSettings, ApiError } from '../api'
import type { SMTPSettingsRequest } from '../api'
import { TextInput, SelectField } from '../components/FormFields'

export default function Settings() {
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')
  const [hasPassword, setHasPassword] = useState(false)

  const [smtp, setSmtp] = useState<SMTPSettingsRequest>({
    host: '',
    port: '587',
    from: '',
    username: '',
    password: '',
    tls: 'starttls',
  })

  useEffect(() => {
    setLoading(true)
    getSettings()
      .then((res) => {
        setSmtp({
          host: res.smtp.host,
          port: res.smtp.port || '587',
          from: res.smtp.from,
          username: res.smtp.username,
          password: '',
          tls: res.smtp.tls || 'starttls',
        })
        setHasPassword(res.smtp.has_password)
      })
      .catch(() => {
        setError('Failed to load settings')
      })
      .finally(() => setLoading(false))
  }, [])

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setError('')
    setSuccess('')
    setSaving(true)

    try {
      const res = await updateSettings({ smtp })
      setHasPassword(res.smtp.has_password)
      setSmtp((prev) => ({ ...prev, password: '' }))
      setSuccess('SMTP settings saved')
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'failed to save settings')
    } finally {
      setSaving(false)
    }
  }

  if (loading) {
    return (
      <div>
        <h1 className="text-2xl font-bold text-gray-900">Settings</h1>
        <p className="mt-4 text-sm text-gray-400">Loading...</p>
      </div>
    )
  }

  return (
    <div>
      <h1 className="text-2xl font-bold text-gray-900">Settings</h1>
      <p className="mt-1 text-sm text-gray-500">System configuration.</p>

      <div className="mt-8 max-w-lg">
        <h2 className="text-lg font-semibold text-gray-900">SMTP / Email</h2>
        <p className="mt-1 text-sm text-gray-500">
          Configure the outgoing mail server used for voicemail email notifications.
        </p>

        <form onSubmit={handleSubmit} className="mt-4 space-y-4">
          {error && (
            <div className="rounded-md bg-red-50 border border-red-200 px-3 py-2">
              <p className="text-sm text-red-700">{error}</p>
            </div>
          )}
          {success && (
            <div className="rounded-md bg-green-50 border border-green-200 px-3 py-2">
              <p className="text-sm text-green-700">{success}</p>
            </div>
          )}

          <div className="grid grid-cols-2 gap-4">
            <TextInput
              label="SMTP Host"
              id="smtp_host"
              value={smtp.host}
              onChange={(e) => setSmtp({ ...smtp, host: e.currentTarget.value })}
              placeholder="smtp.example.com"
            />
            <TextInput
              label="Port"
              id="smtp_port"
              value={smtp.port}
              onChange={(e) => setSmtp({ ...smtp, port: e.currentTarget.value })}
              placeholder="587"
            />
          </div>

          <TextInput
            label="From Address"
            id="smtp_from"
            type="email"
            value={smtp.from}
            onChange={(e) => setSmtp({ ...smtp, from: e.currentTarget.value })}
            placeholder="pbx@example.com"
          />

          <div className="grid grid-cols-2 gap-4">
            <TextInput
              label="Username"
              id="smtp_username"
              value={smtp.username}
              onChange={(e) => setSmtp({ ...smtp, username: e.currentTarget.value })}
              placeholder="user@example.com"
              autoComplete="off"
            />
            <TextInput
              label={hasPassword ? 'Password (leave blank to keep)' : 'Password'}
              id="smtp_password"
              type="password"
              value={smtp.password}
              onChange={(e) => setSmtp({ ...smtp, password: e.currentTarget.value })}
              autoComplete="off"
            />
          </div>

          <SelectField
            label="Encryption"
            id="smtp_tls"
            value={smtp.tls}
            onChange={(e) => setSmtp({ ...smtp, tls: e.currentTarget.value })}
          >
            <option value="starttls">STARTTLS (port 587)</option>
            <option value="tls">Implicit TLS (port 465)</option>
            <option value="none">None (port 25)</option>
          </SelectField>

          <div className="pt-4 border-t border-gray-100">
            <button
              type="submit"
              disabled={saving}
              className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
            >
              {saving ? 'Saving...' : 'Save SMTP Settings'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
