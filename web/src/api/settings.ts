import { get, put } from './client'

/** SIP configuration returned by the API. */
export interface SIPSettings {
  udp_port: string
  tcp_port: string
  tls_port: string
  tls_cert: string
  tls_key: string
  external_ip: string
  hostname: string
}

/** Codecs configuration returned by the API. */
export interface CodecsSettings {
  audio: string
}

/** Recording storage configuration returned by the API. */
export interface RecordingSettings {
  storage_path: string
  format: string
  max_days: string
}

/** SMTP configuration returned by the API. */
export interface SMTPSettings {
  host: string
  port: string
  from: string
  username: string
  tls: string
  has_password: boolean
}

/** License configuration returned by the API. */
export interface LicenseSettings {
  has_key: boolean
  instance_id: string
}

/** Push gateway configuration returned by the API. */
export interface PushSettings {
  gateway_url: string
}

/** Full settings response from GET /settings. */
export interface SystemSettings {
  sip: SIPSettings
  codecs: CodecsSettings
  recording: RecordingSettings
  smtp: SMTPSettings
  license: LicenseSettings
  push: PushSettings
}

/** SIP configuration sent to the API for update. */
export interface SIPSettingsRequest {
  udp_port: string
  tcp_port: string
  tls_port: string
  tls_cert: string
  tls_key: string
  external_ip: string
  hostname: string
}

/** Codecs configuration sent to the API for update. */
export interface CodecsSettingsRequest {
  audio: string
}

/** Recording storage configuration sent to the API for update. */
export interface RecordingSettingsRequest {
  storage_path: string
  format: string
  max_days: string
}

/** SMTP configuration sent to the API for update. */
export interface SMTPSettingsRequest {
  host: string
  port: string
  from: string
  username: string
  password: string
  tls: string
}

/** License configuration sent to the API for update. */
export interface LicenseSettingsRequest {
  key: string
}

/** Push gateway configuration sent to the API for update. */
export interface PushSettingsRequest {
  gateway_url: string
}

/** Settings update request for PUT /settings. */
export interface SystemSettingsRequest {
  sip?: SIPSettingsRequest
  codecs?: CodecsSettingsRequest
  recording?: RecordingSettingsRequest
  smtp?: SMTPSettingsRequest
  license?: LicenseSettingsRequest
  push?: PushSettingsRequest
}

/** Fetch current system settings. */
export function getSettings(): Promise<SystemSettings> {
  return get<SystemSettings>('/settings')
}

/** Update system settings. */
export function updateSettings(data: SystemSettingsRequest): Promise<SystemSettings> {
  return put<SystemSettings>('/settings', data)
}
