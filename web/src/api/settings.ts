import { get, put } from './client'

/** SMTP configuration returned by the API. */
export interface SMTPSettings {
  host: string
  port: string
  from: string
  username: string
  tls: string
  has_password: boolean
}

/** Full settings response from GET /settings. */
export interface SystemSettings {
  smtp: SMTPSettings
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

/** Settings update request for PUT /settings. */
export interface SystemSettingsRequest {
  smtp?: SMTPSettingsRequest
}

/** Fetch current system settings. */
export function getSettings(): Promise<SystemSettings> {
  return get<SystemSettings>('/settings')
}

/** Update system settings. */
export function updateSettings(data: SystemSettingsRequest): Promise<SystemSettings> {
  return put<SystemSettings>('/settings', data)
}
