import type { ApiEnvelope, PaginatedResponse, PaginationParams } from './types'

const BASE_URL = '/api/v1'

/** Error thrown when the API returns an error response. */
export class ApiError extends Error {
  status: number

  constructor(status: number, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

/** Read the CSRF token from the flowpbx_csrf cookie. */
function getCSRFToken(): string | null {
  const match = document.cookie
    .split('; ')
    .find((row) => row.startsWith('flowpbx_csrf='))
  return match ? match.split('=')[1] : null
}

/** Build a URL with query parameters. */
function buildURL(path: string, params?: Record<string, string | number | undefined>): string {
  const url = new URL(BASE_URL + path, window.location.origin)
  if (params) {
    for (const [key, value] of Object.entries(params)) {
      if (value !== undefined) {
        url.searchParams.set(key, String(value))
      }
    }
  }
  return url.pathname + url.search
}

/** Core fetch wrapper with envelope parsing and auth handling. */
async function request<T>(
  method: string,
  path: string,
  options?: {
    body?: unknown
    params?: Record<string, string | number | undefined>
  },
): Promise<T> {
  const url = buildURL(path, options?.params)

  const headers: Record<string, string> = {
    Accept: 'application/json',
  }

  if (options?.body !== undefined) {
    headers['Content-Type'] = 'application/json'
  }

  // Attach CSRF token for state-changing methods
  if (method !== 'GET' && method !== 'HEAD') {
    const csrf = getCSRFToken()
    if (csrf) {
      headers['X-CSRF-Token'] = csrf
    }
  }

  const res = await fetch(url, {
    method,
    headers,
    credentials: 'same-origin',
    body: options?.body !== undefined ? JSON.stringify(options.body) : undefined,
  })

  // Redirect to login on 401
  if (res.status === 401) {
    const currentPath = window.location.pathname
    if (currentPath !== '/login' && currentPath !== '/setup') {
      window.location.href = '/login'
    }
    throw new ApiError(res.status, 'authentication required')
  }

  // Parse JSON envelope
  const envelope: ApiEnvelope<T> = await res.json()

  if (!res.ok || envelope.error) {
    throw new ApiError(res.status, envelope.error ?? `request failed with status ${res.status}`)
  }

  return envelope.data as T
}

/** GET request returning parsed data. */
export function get<T>(
  path: string,
  params?: Record<string, string | number | undefined>,
): Promise<T> {
  return request<T>('GET', path, { params })
}

/** POST request returning parsed data. */
export function post<T>(path: string, body?: unknown): Promise<T> {
  return request<T>('POST', path, { body })
}

/** PUT request returning parsed data. */
export function put<T>(path: string, body?: unknown): Promise<T> {
  return request<T>('PUT', path, { body })
}

/** DELETE request returning parsed data. */
export function del<T = null>(path: string): Promise<T> {
  return request<T>('DELETE', path)
}

/** GET a paginated list endpoint. */
export function list<T>(
  path: string,
  params?: PaginationParams & Record<string, string | number | undefined>,
): Promise<PaginatedResponse<T>> {
  return get<PaginatedResponse<T>>(path, params)
}
