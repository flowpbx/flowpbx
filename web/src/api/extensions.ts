import { get, post, put, del, list } from './client'
import type { Extension, ExtensionRequest, PaginatedResponse, PaginationParams } from './types'

/** List extensions with pagination. */
export function listExtensions(params?: PaginationParams): Promise<PaginatedResponse<Extension>> {
  return list<Extension>('/extensions', params)
}

/** Get a single extension by ID. */
export function getExtension(id: number): Promise<Extension> {
  return get<Extension>(`/extensions/${id}`)
}

/** Create a new extension. */
export function createExtension(data: ExtensionRequest): Promise<Extension> {
  return post<Extension>('/extensions', data)
}

/** Update an existing extension. */
export function updateExtension(id: number, data: ExtensionRequest): Promise<Extension> {
  return put<Extension>(`/extensions/${id}`, data)
}

/** Delete an extension. */
export function deleteExtension(id: number): Promise<null> {
  return del(`/extensions/${id}`)
}
