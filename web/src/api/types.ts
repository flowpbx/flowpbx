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
