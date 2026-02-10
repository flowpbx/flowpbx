import { get, post, put, del } from './client'
import type { ConferenceBridge, ConferenceBridgeRequest, ConferenceParticipant } from './types'

/** List all conference bridges. */
export function listConferenceBridges(): Promise<ConferenceBridge[]> {
  return get<ConferenceBridge[]>('/conferences')
}

/** Get a single conference bridge by ID. */
export function getConferenceBridge(id: number): Promise<ConferenceBridge> {
  return get<ConferenceBridge>(`/conferences/${id}`)
}

/** Create a new conference bridge. */
export function createConferenceBridge(data: ConferenceBridgeRequest): Promise<ConferenceBridge> {
  return post<ConferenceBridge>('/conferences', data)
}

/** Update an existing conference bridge. */
export function updateConferenceBridge(id: number, data: ConferenceBridgeRequest): Promise<ConferenceBridge> {
  return put<ConferenceBridge>(`/conferences/${id}`, data)
}

/** Delete a conference bridge. */
export function deleteConferenceBridge(id: number): Promise<null> {
  return del(`/conferences/${id}`)
}

/** List active participants in a conference bridge. */
export function listConferenceParticipants(bridgeId: number): Promise<ConferenceParticipant[]> {
  return get<ConferenceParticipant[]>(`/conferences/${bridgeId}/participants`)
}

/** Mute or unmute a conference participant. */
export function muteConferenceParticipant(bridgeId: number, participantId: string, muted: boolean): Promise<{ participant_id: string; muted: boolean }> {
  return put<{ participant_id: string; muted: boolean }>(`/conferences/${bridgeId}/participants/${participantId}/mute`, { muted })
}

/** Kick a participant from a conference. */
export function kickConferenceParticipant(bridgeId: number, participantId: string): Promise<null> {
  return del(`/conferences/${bridgeId}/participants/${participantId}`)
}
