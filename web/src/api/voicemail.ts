import { get, post, put, del, list } from './client'
import type { VoicemailBox, VoicemailBoxRequest, VoicemailMessage, PaginatedResponse, PaginationParams } from './types'

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

/** List voicemail messages for a box. */
export function listVoicemailMessages(boxId: number): Promise<VoicemailMessage[]> {
  return get<VoicemailMessage[]>(`/voicemail-boxes/${boxId}/messages`)
}

/** Delete a voicemail message. */
export function deleteVoicemailMessage(boxId: number, msgId: number): Promise<null> {
  return del(`/voicemail-boxes/${boxId}/messages/${msgId}`)
}

/** Mark a voicemail message as read. */
export function markVoicemailMessageRead(boxId: number, msgId: number): Promise<VoicemailMessage> {
  return put<VoicemailMessage>(`/voicemail-boxes/${boxId}/messages/${msgId}/read`)
}

/** Build the audio URL for a voicemail message. */
export function voicemailAudioURL(boxId: number, msgId: number): string {
  return `/api/v1/voicemail-boxes/${boxId}/messages/${msgId}/audio`
}
