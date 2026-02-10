/** JSON envelope returned by all API endpoints. */
export interface ApiEnvelope<T> {
  data?: T
  error?: string
}

/** Paginated list response from list endpoints. */
export interface PaginatedResponse<T> {
  items: T[]
  total: number
  limit: number
  offset: number
}

/** Pagination query parameters. */
export interface PaginationParams {
  limit?: number
  offset?: number
}

/** Login request body. */
export interface LoginRequest {
  username: string
  password: string
}

/** Login response data. */
export interface LoginResponse {
  user_id: number
  username: string
}

/** Current authenticated user. */
export interface AuthUser {
  user_id: number
  username: string
}

/** Setup wizard request body. */
export interface SetupRequest {
  admin_username: string
  admin_password: string
  hostname: string
  sip_port: number
  sip_tls_port: number
  rtp_port_min: number
  rtp_port_max: number
}

/** Health check response. */
export interface HealthResponse {
  status: string
  needs_setup: boolean
}

/** Extension resource. */
export interface Extension {
  id: number
  extension: string
  name: string
  email: string
  sip_username: string
  ring_timeout: number
  dnd: boolean
  follow_me_enabled: boolean
  recording_mode: string
  max_registrations: number
  created_at: string
  updated_at: string
}

/** Extension create/update request. */
export interface ExtensionRequest {
  extension: string
  name: string
  email?: string
  sip_username: string
  sip_password?: string
  ring_timeout?: number
  dnd?: boolean
  follow_me_enabled?: boolean
  recording_mode?: string
  max_registrations?: number
}

/** Trunk resource. */
export interface Trunk {
  id: number
  name: string
  type: string
  enabled: boolean
  host: string
  port: number
  transport: string
  username: string
  auth_username: string
  register_expiry: number
  remote_hosts: string
  local_host: string
  codecs: string
  max_channels: number
  caller_id_name: string
  caller_id_num: string
  prefix_strip: number
  prefix_add: string
  priority: number
  status?: string
  created_at: string
  updated_at: string
}

/** Trunk create/update request. */
export interface TrunkRequest {
  name: string
  type: string
  enabled?: boolean
  host?: string
  port?: number
  transport?: string
  username?: string
  password?: string
  auth_username?: string
  register_expiry?: number
  remote_hosts?: string
  local_host?: string
  codecs?: string
  max_channels?: number
  caller_id_name?: string
  caller_id_num?: string
  prefix_strip?: number
  prefix_add?: string
  priority?: number
}

/** Voicemail box resource. */
export interface VoicemailBox {
  id: number
  name: string
  mailbox_number: string
  greeting_type: string
  email_notify: boolean
  email_address: string
  email_attach_audio: boolean
  max_message_duration: number
  max_messages: number
  retention_days: number
  notify_extension_id: number | null
  created_at: string
  updated_at: string
}

/** Voicemail box create/update request. */
export interface VoicemailBoxRequest {
  name: string
  mailbox_number?: string
  pin?: string
  greeting_type?: string
  email_notify?: boolean
  email_address?: string
  email_attach_audio?: boolean
  max_message_duration?: number
  max_messages?: number
  retention_days?: number
  notify_extension_id?: number | null
}
