import { get, post, put, del } from './client'
import type { ConferenceBridge, ConferenceBridgeRequest } from './types'

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
