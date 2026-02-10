import { useState, useEffect, type FormEvent } from 'react'
import { getSettings, updateSettings, reloadSystem, ApiError } from '../api'
import type {
  SIPSettingsRequest,
  CodecsSettingsRequest,
  RecordingSettingsRequest,
  SMTPSettingsRequest,
  LicenseSettingsRequest,
  PushSettingsRequest,
} from '../api'
import { TextInput, SelectField } from '../components/FormFields'

export default function Settings() {
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')

  // Track which sections have saved passwords / keys.
  const [hasSmtpPassword, setHasSmtpPassword] = useState(false)
  const [hasLicenseKey, setHasLicenseKey] = useState(false)
  const [licenseInstanceId, setLicenseInstanceId] = useState('')

  // Section form states.
  const [sip, setSip] = useState<SIPSettingsRequest>({
    udp_port: '5060',
    tcp_port: '',
    tls_port: '5061',
    tls_cert: '',
    tls_key: '',
    external_ip: '',
    hostname: '',
  })

  const [codecs, setCodecs] = useState<CodecsSettingsRequest>({
    audio: 'g711u,g711a,opus',
  })

  const [recording, setRecording] = useState<RecordingSettingsRequest>({
    storage_path: '',
    format: 'wav',
    max_days: '',
  })

  const [smtp, setSmtp] = useState<SMTPSettingsRequest>({
    host: '',
    port: '587',
    from: '',
    username: '',
    password: '',
    tls: 'starttls',
  })

  const [license, setLicense] = useState<LicenseSettingsRequest>({
    key: '',
  })

  const [push, setPush] = useState<PushSettingsRequest>({
    gateway_url: '',
  })

  // Track which section is being saved.
  const [savingSection, setSavingSection] = useState<string | null>(null)
  const [reloading, setReloading] = useState(false)

  async function handleReload() {
    setError('')
    setSuccess('')
    setReloading(true)
    try {
      await reloadSystem()
      setSuccess('System configuration reloaded')
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'failed to reload configuration')
    } finally {
      setReloading(false)
    }
  }

  useEffect(() => {
    setLoading(true)
    getSettings()
      .then((res) => {
        setSip({
          udp_port: res.sip.udp_port || '5060',
          tcp_port: res.sip.tcp_port || '',
          tls_port: res.sip.tls_port || '5061',
          tls_cert: res.sip.tls_cert || '',
          tls_key: res.sip.tls_key || '',
          external_ip: res.sip.external_ip || '',
          hostname: res.sip.hostname || '',
        })
        setCodecs({
          audio: res.codecs.audio || 'g711u,g711a,opus',
        })
        setRecording({
          storage_path: res.recording.storage_path || '',
          format: res.recording.format || 'wav',
          max_days: res.recording.max_days || '',
        })
        setSmtp({
          host: res.smtp.host,
          port: res.smtp.port || '587',
          from: res.smtp.from,
          username: res.smtp.username,
          password: '',
          tls: res.smtp.tls || 'starttls',
        })
        setHasSmtpPassword(res.smtp.has_password)
        setHasLicenseKey(res.license.has_key)
        setLicenseInstanceId(res.license.instance_id)
        setLicense({ key: '' })
        setPush({
          gateway_url: res.push.gateway_url || '',
        })
      })
      .catch(() => {
        setError('Failed to load settings')
      })
      .finally(() => setLoading(false))
  }, [])

  async function saveSection(section: string, data: Record<string, unknown>) {
    setError('')
    setSuccess('')
    setSavingSection(section)

    try {
      const res = await updateSettings(data)
      setHasSmtpPassword(res.smtp.has_password)
      setHasLicenseKey(res.license.has_key)
      setLicenseInstanceId(res.license.instance_id)
      // Clear sensitive fields after save.
      setSmtp((prev) => ({ ...prev, password: '' }))
      setLicense({ key: '' })
      setSuccess(`${section} settings saved`)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'failed to save settings')
    } finally {
      setSavingSection(null)
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
      <p className="mt-1 text-sm text-gray-500">System configuration. Changes to SIP and codec settings require a reload to take effect.</p>

      <div className="mt-4 flex items-center gap-3">
        <button
          onClick={handleReload}
          disabled={reloading}
          className="rounded-md bg-amber-600 px-4 py-2 text-sm font-medium text-white hover:bg-amber-700 focus:outline-none focus:ring-2 focus:ring-amber-500 focus:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
        >
          {reloading ? 'Reloading...' : 'Reload Configuration'}
        </button>
        <span className="text-sm text-gray-500">Apply saved changes to the running system without restarting.</span>
      </div>

      {error && (
        <div className="mt-4 max-w-lg rounded-md bg-red-50 border border-red-200 px-3 py-2">
          <p className="text-sm text-red-700">{error}</p>
        </div>
      )}
      {success && (
        <div className="mt-4 max-w-lg rounded-md bg-green-50 border border-green-200 px-3 py-2">
          <p className="text-sm text-green-700">{success}</p>
        </div>
      )}

      {/* SIP Settings */}
      <Section
        title="SIP"
        description="SIP transport ports and TLS certificate paths."
        saving={savingSection === 'SIP'}
        onSubmit={() => saveSection('SIP', { sip })}
      >
        <div className="grid grid-cols-3 gap-4">
          <TextInput
            label="UDP Port"
            id="sip_udp_port"
            value={sip.udp_port}
            onChange={(e) => setSip({ ...sip, udp_port: e.currentTarget.value })}
            placeholder="5060"
          />
          <TextInput
            label="TCP Port"
            id="sip_tcp_port"
            value={sip.tcp_port}
            onChange={(e) => setSip({ ...sip, tcp_port: e.currentTarget.value })}
            placeholder="5060"
          />
          <TextInput
            label="TLS Port"
            id="sip_tls_port"
            value={sip.tls_port}
            onChange={(e) => setSip({ ...sip, tls_port: e.currentTarget.value })}
            placeholder="5061"
          />
        </div>
        <TextInput
          label="Hostname"
          id="sip_hostname"
          value={sip.hostname}
          onChange={(e) => setSip({ ...sip, hostname: e.currentTarget.value })}
          placeholder="pbx.example.com"
        />
        <TextInput
          label="External IP"
          id="sip_external_ip"
          value={sip.external_ip}
          onChange={(e) => setSip({ ...sip, external_ip: e.currentTarget.value })}
          placeholder="Auto-detected if empty"
        />
        <TextInput
          label="TLS Certificate Path"
          id="sip_tls_cert"
          value={sip.tls_cert}
          onChange={(e) => setSip({ ...sip, tls_cert: e.currentTarget.value })}
          placeholder="/etc/flowpbx/tls/cert.pem"
        />
        <TextInput
          label="TLS Key Path"
          id="sip_tls_key"
          value={sip.tls_key}
          onChange={(e) => setSip({ ...sip, tls_key: e.currentTarget.value })}
          placeholder="/etc/flowpbx/tls/key.pem"
        />
      </Section>

      {/* Codecs */}
      <Section
        title="Codecs"
        description="Audio codecs offered in SDP, in preference order. Supported: g711u, g711a, opus."
        saving={savingSection === 'Codecs'}
        onSubmit={() => saveSection('Codecs', { codecs })}
      >
        <TextInput
          label="Audio Codecs"
          id="codecs_audio"
          value={codecs.audio}
          onChange={(e) => setCodecs({ audio: e.currentTarget.value })}
          placeholder="g711u,g711a,opus"
        />
      </Section>

      {/* Recording Storage */}
      <Section
        title="Recording"
        description="Call recording storage settings."
        saving={savingSection === 'Recording'}
        onSubmit={() => saveSection('Recording', { recording })}
      >
        <TextInput
          label="Storage Path"
          id="recording_storage_path"
          value={recording.storage_path}
          onChange={(e) => setRecording({ ...recording, storage_path: e.currentTarget.value })}
          placeholder="Default: data/recordings"
        />
        <div className="grid grid-cols-2 gap-4">
          <SelectField
            label="Format"
            id="recording_format"
            value={recording.format}
            onChange={(e) => setRecording({ ...recording, format: e.currentTarget.value })}
          >
            <option value="wav">WAV</option>
            <option value="mp3">MP3</option>
          </SelectField>
          <TextInput
            label="Retention (days)"
            id="recording_max_days"
            value={recording.max_days}
            onChange={(e) => setRecording({ ...recording, max_days: e.currentTarget.value })}
            placeholder="0 = keep forever"
          />
        </div>
      </Section>

      {/* SMTP */}
      <Section
        title="SMTP / Email"
        description="Outgoing mail server for voicemail email notifications."
        saving={savingSection === 'SMTP'}
        onSubmit={() => saveSection('SMTP', { smtp })}
      >
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
            label={hasSmtpPassword ? 'Password (leave blank to keep)' : 'Password'}
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
      </Section>

      {/* License */}
      <Section
        title="License"
        description="FlowPBX license key for push notifications and premium features."
        saving={savingSection === 'License'}
        onSubmit={() => saveSection('License', { license })}
      >
        <TextInput
          label={hasLicenseKey ? 'License Key (leave blank to keep)' : 'License Key'}
          id="license_key"
          value={license.key}
          onChange={(e) => setLicense({ key: e.currentTarget.value })}
          placeholder="XXXX-XXXX-XXXX-XXXX"
          autoComplete="off"
        />
        {licenseInstanceId && (
          <p className="text-xs text-gray-400">Instance ID: {licenseInstanceId}</p>
        )}
      </Section>

      {/* Push Gateway */}
      <Section
        title="Push Gateway"
        description="URL of the push notification gateway for mobile app wake-up."
        saving={savingSection === 'Push'}
        onSubmit={() => saveSection('Push', { push })}
      >
        <TextInput
          label="Gateway URL"
          id="push_gateway_url"
          value={push.gateway_url}
          onChange={(e) => setPush({ gateway_url: e.currentTarget.value })}
          placeholder="https://push.flowpbx.com"
        />
      </Section>
    </div>
  )
}

/** Reusable settings section with its own save button. */
function Section({
  title,
  description,
  saving,
  onSubmit,
  children,
}: {
  title: string
  description: string
  saving: boolean
  onSubmit: () => void
  children: React.ReactNode
}) {
  function handleSubmit(e: FormEvent) {
    e.preventDefault()
    onSubmit()
  }

  return (
    <div className="mt-8 max-w-lg">
      <h2 className="text-lg font-semibold text-gray-900">{title}</h2>
      <p className="mt-1 text-sm text-gray-500">{description}</p>

      <form onSubmit={handleSubmit} className="mt-4 space-y-4">
        {children}

        <div className="pt-4 border-t border-gray-100">
          <button
            type="submit"
            disabled={saving}
            className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
          >
            {saving ? 'Saving...' : `Save ${title} Settings`}
          </button>
        </div>
      </form>
    </div>
  )
}
