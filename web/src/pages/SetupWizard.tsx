import { useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { setup } from '../api/auth'
import { ApiError } from '../api/client'

const STEPS = [
  'Admin Account',
  'Hostname',
  'SIP Ports',
  'First Trunk',
  'First Extension',
  'License Key',
  'Review',
] as const

interface WizardData {
  admin_username: string
  admin_password: string
  admin_password_confirm: string
  hostname: string
  sip_port: number
  sip_tls_port: number
  rtp_port_min: number
  rtp_port_max: number
  // Optional trunk
  trunk_name: string
  trunk_host: string
  trunk_port: number
  trunk_username: string
  trunk_password: string
  trunk_transport: string
  skip_trunk: boolean
  // Optional extension
  ext_number: string
  ext_name: string
  ext_password: string
  skip_extension: boolean
  // License
  license_key: string
  skip_license: boolean
}

const initialData: WizardData = {
  admin_username: 'admin',
  admin_password: '',
  admin_password_confirm: '',
  hostname: '',
  sip_port: 5060,
  sip_tls_port: 5061,
  rtp_port_min: 10000,
  rtp_port_max: 20000,
  trunk_name: '',
  trunk_host: '',
  trunk_port: 5060,
  trunk_username: '',
  trunk_password: '',
  trunk_transport: 'udp',
  skip_trunk: false,
  ext_number: '100',
  ext_name: '',
  ext_password: '',
  skip_extension: false,
  license_key: '',
  skip_license: false,
}

const inputClass =
  'block w-full rounded-md border border-gray-300 px-3 py-2 text-sm text-gray-900 placeholder-gray-400 focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500'
const labelClass = 'block text-sm font-medium text-gray-700 mb-1'

export default function SetupWizard() {
  const navigate = useNavigate()
  const [step, setStep] = useState(0)
  const [data, setData] = useState<WizardData>(initialData)
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)

  function set<K extends keyof WizardData>(key: K, value: WizardData[K]) {
    setData((prev) => ({ ...prev, [key]: value }))
  }

  function validateStep(): string | null {
    switch (step) {
      case 0: {
        if (!data.admin_username.trim()) return 'Username is required'
        if (data.admin_password.length < 8) return 'Password must be at least 8 characters'
        if (data.admin_password !== data.admin_password_confirm) return 'Passwords do not match'
        return null
      }
      case 1: {
        if (!data.hostname.trim()) return 'Hostname or IP address is required'
        return null
      }
      case 2: {
        if (data.sip_port < 1 || data.sip_port > 65535) return 'SIP port must be 1-65535'
        if (data.sip_tls_port < 1 || data.sip_tls_port > 65535) return 'SIP TLS port must be 1-65535'
        if (data.rtp_port_min < 1024) return 'RTP port min must be at least 1024'
        if (data.rtp_port_max > 65535) return 'RTP port max must be at most 65535'
        if (data.rtp_port_min >= data.rtp_port_max) return 'RTP port min must be less than max'
        return null
      }
      case 3: {
        if (!data.skip_trunk) {
          if (!data.trunk_name.trim()) return 'Trunk name is required'
          if (!data.trunk_host.trim()) return 'Trunk host is required'
        }
        return null
      }
      case 4: {
        if (!data.skip_extension) {
          if (!data.ext_number.trim()) return 'Extension number is required'
          if (!data.ext_name.trim()) return 'Display name is required'
          if (data.ext_password.length < 8) return 'SIP password must be at least 8 characters'
        }
        return null
      }
      default:
        return null
    }
  }

  function handleNext(e: FormEvent) {
    e.preventDefault()
    const err = validateStep()
    if (err) {
      setError(err)
      return
    }
    setError('')
    setStep((s) => s + 1)
  }

  function handleBack() {
    setError('')
    setStep((s) => s - 1)
  }

  async function handleFinish(e: FormEvent) {
    e.preventDefault()
    setError('')
    setSubmitting(true)

    try {
      await setup({
        admin_username: data.admin_username.trim(),
        admin_password: data.admin_password,
        hostname: data.hostname.trim(),
        sip_port: data.sip_port,
        sip_tls_port: data.sip_tls_port,
        rtp_port_min: data.rtp_port_min,
        rtp_port_max: data.rtp_port_max,
      })
      navigate('/login', { replace: true })
    } catch (err) {
      if (err instanceof ApiError) {
        setError(err.message)
      } else {
        setError('unable to connect to server')
      }
    } finally {
      setSubmitting(false)
    }
  }

  function renderStepContent() {
    switch (step) {
      case 0:
        return (
          <div className="space-y-4">
            <p className="text-sm text-gray-500">Create the initial administrator account.</p>
            <div>
              <label htmlFor="admin_username" className={labelClass}>Username</label>
              <input
                id="admin_username"
                type="text"
                required
                autoComplete="username"
                autoFocus
                value={data.admin_username}
                onChange={(e) => set('admin_username', e.target.value)}
                className={inputClass}
              />
            </div>
            <div>
              <label htmlFor="admin_password" className={labelClass}>Password</label>
              <input
                id="admin_password"
                type="password"
                required
                autoComplete="new-password"
                value={data.admin_password}
                onChange={(e) => set('admin_password', e.target.value)}
                className={inputClass}
                placeholder="Minimum 8 characters"
              />
            </div>
            <div>
              <label htmlFor="admin_password_confirm" className={labelClass}>Confirm Password</label>
              <input
                id="admin_password_confirm"
                type="password"
                required
                autoComplete="new-password"
                value={data.admin_password_confirm}
                onChange={(e) => set('admin_password_confirm', e.target.value)}
                className={inputClass}
              />
            </div>
          </div>
        )

      case 1:
        return (
          <div className="space-y-4">
            <p className="text-sm text-gray-500">
              Enter the hostname or public IP address that SIP devices and trunks will use to reach this PBX.
            </p>
            <div>
              <label htmlFor="hostname" className={labelClass}>Hostname / External IP</label>
              <input
                id="hostname"
                type="text"
                required
                autoFocus
                value={data.hostname}
                onChange={(e) => set('hostname', e.target.value)}
                className={inputClass}
                placeholder="pbx.example.com or 203.0.113.10"
              />
            </div>
          </div>
        )

      case 2:
        return (
          <div className="space-y-4">
            <p className="text-sm text-gray-500">
              Configure SIP signaling and RTP media port ranges. Defaults work for most installations.
            </p>
            <div className="grid grid-cols-2 gap-4">
              <div>
                <label htmlFor="sip_port" className={labelClass}>SIP UDP/TCP Port</label>
                <input
                  id="sip_port"
                  type="number"
                  required
                  autoFocus
                  min={1}
                  max={65535}
                  value={data.sip_port}
                  onChange={(e) => set('sip_port', Number(e.target.value))}
                  className={inputClass}
                />
              </div>
              <div>
                <label htmlFor="sip_tls_port" className={labelClass}>SIP TLS Port</label>
                <input
                  id="sip_tls_port"
                  type="number"
                  required
                  min={1}
                  max={65535}
                  value={data.sip_tls_port}
                  onChange={(e) => set('sip_tls_port', Number(e.target.value))}
                  className={inputClass}
                />
              </div>
            </div>
            <div className="grid grid-cols-2 gap-4">
              <div>
                <label htmlFor="rtp_port_min" className={labelClass}>RTP Port Min</label>
                <input
                  id="rtp_port_min"
                  type="number"
                  required
                  min={1024}
                  max={65535}
                  value={data.rtp_port_min}
                  onChange={(e) => set('rtp_port_min', Number(e.target.value))}
                  className={inputClass}
                />
              </div>
              <div>
                <label htmlFor="rtp_port_max" className={labelClass}>RTP Port Max</label>
                <input
                  id="rtp_port_max"
                  type="number"
                  required
                  min={1024}
                  max={65535}
                  value={data.rtp_port_max}
                  onChange={(e) => set('rtp_port_max', Number(e.target.value))}
                  className={inputClass}
                />
              </div>
            </div>
          </div>
        )

      case 3:
        return (
          <div className="space-y-4">
            <p className="text-sm text-gray-500">
              Optionally configure your first SIP trunk to connect to an upstream provider.
            </p>
            <label className="flex items-center gap-2 text-sm text-gray-700">
              <input
                type="checkbox"
                checked={data.skip_trunk}
                onChange={(e) => set('skip_trunk', e.target.checked)}
                className="rounded border-gray-300 text-blue-600 focus:ring-blue-500"
              />
              Skip — I'll add a trunk later
            </label>
            {!data.skip_trunk && (
              <div className="space-y-4">
                <div>
                  <label htmlFor="trunk_name" className={labelClass}>Trunk Name</label>
                  <input
                    id="trunk_name"
                    type="text"
                    value={data.trunk_name}
                    onChange={(e) => set('trunk_name', e.target.value)}
                    className={inputClass}
                    placeholder="My SIP Provider"
                  />
                </div>
                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <label htmlFor="trunk_host" className={labelClass}>Host</label>
                    <input
                      id="trunk_host"
                      type="text"
                      value={data.trunk_host}
                      onChange={(e) => set('trunk_host', e.target.value)}
                      className={inputClass}
                      placeholder="sip.provider.com"
                    />
                  </div>
                  <div>
                    <label htmlFor="trunk_port" className={labelClass}>Port</label>
                    <input
                      id="trunk_port"
                      type="number"
                      min={1}
                      max={65535}
                      value={data.trunk_port}
                      onChange={(e) => set('trunk_port', Number(e.target.value))}
                      className={inputClass}
                    />
                  </div>
                </div>
                <div>
                  <label htmlFor="trunk_transport" className={labelClass}>Transport</label>
                  <select
                    id="trunk_transport"
                    value={data.trunk_transport}
                    onChange={(e) => set('trunk_transport', e.target.value)}
                    className={inputClass}
                  >
                    <option value="udp">UDP</option>
                    <option value="tcp">TCP</option>
                    <option value="tls">TLS</option>
                  </select>
                </div>
                <div>
                  <label htmlFor="trunk_username" className={labelClass}>Username</label>
                  <input
                    id="trunk_username"
                    type="text"
                    value={data.trunk_username}
                    onChange={(e) => set('trunk_username', e.target.value)}
                    className={inputClass}
                    autoComplete="off"
                  />
                </div>
                <div>
                  <label htmlFor="trunk_password" className={labelClass}>Password</label>
                  <input
                    id="trunk_password"
                    type="password"
                    value={data.trunk_password}
                    onChange={(e) => set('trunk_password', e.target.value)}
                    className={inputClass}
                    autoComplete="off"
                  />
                </div>
              </div>
            )}
          </div>
        )

      case 4:
        return (
          <div className="space-y-4">
            <p className="text-sm text-gray-500">
              Optionally create your first extension so a phone can register immediately.
            </p>
            <label className="flex items-center gap-2 text-sm text-gray-700">
              <input
                type="checkbox"
                checked={data.skip_extension}
                onChange={(e) => set('skip_extension', e.target.checked)}
                className="rounded border-gray-300 text-blue-600 focus:ring-blue-500"
              />
              Skip — I'll add extensions later
            </label>
            {!data.skip_extension && (
              <div className="space-y-4">
                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <label htmlFor="ext_number" className={labelClass}>Extension Number</label>
                    <input
                      id="ext_number"
                      type="text"
                      value={data.ext_number}
                      onChange={(e) => set('ext_number', e.target.value)}
                      className={inputClass}
                      placeholder="100"
                    />
                  </div>
                  <div>
                    <label htmlFor="ext_name" className={labelClass}>Display Name</label>
                    <input
                      id="ext_name"
                      type="text"
                      value={data.ext_name}
                      onChange={(e) => set('ext_name', e.target.value)}
                      className={inputClass}
                      placeholder="John Smith"
                    />
                  </div>
                </div>
                <div>
                  <label htmlFor="ext_password" className={labelClass}>SIP Password</label>
                  <input
                    id="ext_password"
                    type="password"
                    value={data.ext_password}
                    onChange={(e) => set('ext_password', e.target.value)}
                    className={inputClass}
                    placeholder="Minimum 8 characters"
                    autoComplete="off"
                  />
                </div>
              </div>
            )}
          </div>
        )

      case 5:
        return (
          <div className="space-y-4">
            <p className="text-sm text-gray-500">
              Enter your FlowPBX license key, or skip to use the free tier.
            </p>
            <label className="flex items-center gap-2 text-sm text-gray-700">
              <input
                type="checkbox"
                checked={data.skip_license}
                onChange={(e) => set('skip_license', e.target.checked)}
                className="rounded border-gray-300 text-blue-600 focus:ring-blue-500"
              />
              Skip — use free tier
            </label>
            {!data.skip_license && (
              <div>
                <label htmlFor="license_key" className={labelClass}>License Key</label>
                <input
                  id="license_key"
                  type="text"
                  value={data.license_key}
                  onChange={(e) => set('license_key', e.target.value)}
                  className={inputClass}
                  placeholder="XXXX-XXXX-XXXX-XXXX"
                  autoComplete="off"
                />
              </div>
            )}
          </div>
        )

      case 6:
        return (
          <div className="space-y-4">
            <p className="text-sm text-gray-500">
              Review your settings before completing setup.
            </p>
            <dl className="divide-y divide-gray-100 text-sm">
              <ReviewRow label="Admin Username" value={data.admin_username} />
              <ReviewRow label="Hostname" value={data.hostname} />
              <ReviewRow label="SIP Port" value={String(data.sip_port)} />
              <ReviewRow label="SIP TLS Port" value={String(data.sip_tls_port)} />
              <ReviewRow label="RTP Ports" value={`${data.rtp_port_min} – ${data.rtp_port_max}`} />
              <ReviewRow
                label="First Trunk"
                value={data.skip_trunk ? 'Skipped' : `${data.trunk_name} (${data.trunk_host})`}
              />
              <ReviewRow
                label="First Extension"
                value={data.skip_extension ? 'Skipped' : `${data.ext_number} — ${data.ext_name}`}
              />
              <ReviewRow
                label="License"
                value={data.skip_license ? 'Free tier' : data.license_key || '(empty)'}
              />
            </dl>
          </div>
        )
    }
  }

  const isLastStep = step === STEPS.length - 1
  const onSubmit = isLastStep ? handleFinish : handleNext

  return (
    <div className="min-h-screen bg-gray-50 flex items-center justify-center px-4">
      <div className="w-full max-w-lg">
        {/* Brand */}
        <div className="text-center mb-8">
          <h1 className="text-2xl font-bold text-gray-900 tracking-tight">FlowPBX Setup</h1>
          <p className="mt-1 text-sm text-gray-500">Step {step + 1} of {STEPS.length}</p>
        </div>

        {/* Progress bar */}
        <div className="mb-6">
          <div className="flex gap-1">
            {STEPS.map((_, i) => (
              <div
                key={i}
                className={`h-1 flex-1 rounded-full transition-colors ${
                  i <= step ? 'bg-blue-600' : 'bg-gray-200'
                }`}
              />
            ))}
          </div>
        </div>

        {/* Card */}
        <form
          onSubmit={onSubmit}
          className="bg-white rounded-lg shadow-sm border border-gray-200 p-6"
        >
          <h2 className="text-lg font-semibold text-gray-900 mb-4">{STEPS[step]}</h2>

          {error && (
            <div className="rounded-md bg-red-50 border border-red-200 px-3 py-2 mb-4">
              <p className="text-sm text-red-700">{error}</p>
            </div>
          )}

          {renderStepContent()}

          {/* Navigation */}
          <div className="flex justify-between mt-6 pt-4 border-t border-gray-100">
            <button
              type="button"
              onClick={handleBack}
              disabled={step === 0}
              className="rounded-md border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
            >
              Back
            </button>
            <button
              type="submit"
              disabled={submitting}
              className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
            >
              {isLastStep ? (submitting ? 'Finishing…' : 'Finish Setup') : 'Next'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}

function ReviewRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex justify-between py-2">
      <dt className="text-gray-500">{label}</dt>
      <dd className="font-medium text-gray-900">{value}</dd>
    </div>
  )
}
