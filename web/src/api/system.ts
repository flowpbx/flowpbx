import { post } from './client'

/** Response from POST /system/reload. */
export interface ReloadResponse {
  status: string
  reloaded: boolean
  timestamp: string
}

/** Trigger a hot-reload of system configuration. */
export function reloadSystem(): Promise<ReloadResponse> {
  return post<ReloadResponse>('/system/reload')
}
