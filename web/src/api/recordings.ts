import { list, del } from './client'
import type { PaginatedResponse, Recording } from './types'

export interface RecordingListParams {
  limit?: number
  offset?: number
  search?: string
  start_date?: string
  end_date?: string
}

/** List recordings (CDRs with recording files) with pagination and optional filters. */
export function listRecordings(params?: RecordingListParams): Promise<PaginatedResponse<Recording>> {
  return list<Recording>('/recordings', params as Record<string, string | number | undefined>)
}

/** Delete a recording by CDR ID. */
export function deleteRecording(id: number): Promise<null> {
  return del(`/recordings/${id}`)
}

/** Build the download URL for a recording. */
export function recordingDownloadURL(id: number): string {
  return `/api/v1/recordings/${id}/download`
}
