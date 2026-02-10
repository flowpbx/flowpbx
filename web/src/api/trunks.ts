import { get, post, put, del, list } from './client'
import type { Trunk, TrunkRequest, TrunkStatusEntry, PaginatedResponse, PaginationParams } from './types'

/** List trunks with pagination. */
export function listTrunks(params?: PaginationParams): Promise<PaginatedResponse<Trunk>> {
  return list<Trunk>('/trunks', params)
}

/** Get a single trunk by ID. */
export function getTrunk(id: number): Promise<Trunk> {
  return get<Trunk>(`/trunks/${id}`)
}

/** Create a new trunk. */
export function createTrunk(data: TrunkRequest): Promise<Trunk> {
  return post<Trunk>('/trunks', data)
}

/** Update an existing trunk. */
export function updateTrunk(id: number, data: TrunkRequest): Promise<Trunk> {
  return put<Trunk>(`/trunks/${id}`, data)
}

/** Delete a trunk. */
export function deleteTrunk(id: number): Promise<null> {
  return del(`/trunks/${id}`)
}

/** List all trunk registration statuses. */
export function listTrunkStatuses(): Promise<TrunkStatusEntry[]> {
  return get<TrunkStatusEntry[]>('/trunks/status')
}
