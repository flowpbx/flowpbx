import { list, get } from './client'
import type { PaginatedResponse, CDR } from './types'

export interface CDRListParams {
  limit?: number
  offset?: number
  search?: string
  direction?: string
  start_date?: string
  end_date?: string
}

/** List CDRs with pagination and optional filters. */
export function listCDRs(params?: CDRListParams): Promise<PaginatedResponse<CDR>> {
  return list<CDR>('/cdrs', params as Record<string, string | number | undefined>)
}

/** Get a single CDR by ID. */
export function getCDR(id: number): Promise<CDR> {
  return get<CDR>(`/cdrs/${id}`)
}

/** Build the CSV export URL with current filters. */
export function buildExportURL(params?: Omit<CDRListParams, 'limit' | 'offset'>): string {
  const url = new URL('/api/v1/cdrs/export', window.location.origin)
  if (params) {
    for (const [key, value] of Object.entries(params)) {
      if (value !== undefined && value !== '') {
        url.searchParams.set(key, String(value))
      }
    }
  }
  return url.toString()
}
