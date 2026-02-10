import { get, post, put, del } from './client'
import type { RingGroup, RingGroupRequest } from './types'

/** List all ring groups. */
export function listRingGroups(): Promise<RingGroup[]> {
  return get<RingGroup[]>('/ring-groups')
}

/** Get a single ring group by ID. */
export function getRingGroup(id: number): Promise<RingGroup> {
  return get<RingGroup>(`/ring-groups/${id}`)
}

/** Create a new ring group. */
export function createRingGroup(data: RingGroupRequest): Promise<RingGroup> {
  return post<RingGroup>('/ring-groups', data)
}

/** Update an existing ring group. */
export function updateRingGroup(id: number, data: RingGroupRequest): Promise<RingGroup> {
  return put<RingGroup>(`/ring-groups/${id}`, data)
}

/** Delete a ring group. */
export function deleteRingGroup(id: number): Promise<null> {
  return del(`/ring-groups/${id}`)
}
