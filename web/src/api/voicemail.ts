import { get, post, put, del, list } from './client'
import type { VoicemailBox, VoicemailBoxRequest, PaginatedResponse, PaginationParams } from './types'

/** List voicemail boxes with pagination. */
export function listVoicemailBoxes(params?: PaginationParams): Promise<PaginatedResponse<VoicemailBox>> {
  return list<VoicemailBox>('/voicemail-boxes', params)
}

/** Get a single voicemail box by ID. */
export function getVoicemailBox(id: number): Promise<VoicemailBox> {
  return get<VoicemailBox>(`/voicemail-boxes/${id}`)
}

/** Create a new voicemail box. */
export function createVoicemailBox(data: VoicemailBoxRequest): Promise<VoicemailBox> {
  return post<VoicemailBox>('/voicemail-boxes', data)
}

/** Update an existing voicemail box. */
export function updateVoicemailBox(id: number, data: VoicemailBoxRequest): Promise<VoicemailBox> {
  return put<VoicemailBox>(`/voicemail-boxes/${id}`, data)
}

/** Delete a voicemail box. */
export function deleteVoicemailBox(id: number): Promise<null> {
  return del(`/voicemail-boxes/${id}`)
}
