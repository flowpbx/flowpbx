import { get, post, put, del } from './client'
import type { TimeSwitch, TimeSwitchRequest } from './types'

/** List all time switches. */
export function listTimeSwitches(): Promise<TimeSwitch[]> {
  return get<TimeSwitch[]>('/time-switches')
}

/** Get a single time switch by ID. */
export function getTimeSwitch(id: number): Promise<TimeSwitch> {
  return get<TimeSwitch>(`/time-switches/${id}`)
}

/** Create a new time switch. */
export function createTimeSwitch(data: TimeSwitchRequest): Promise<TimeSwitch> {
  return post<TimeSwitch>('/time-switches', data)
}

/** Update an existing time switch. */
export function updateTimeSwitch(id: number, data: TimeSwitchRequest): Promise<TimeSwitch> {
  return put<TimeSwitch>(`/time-switches/${id}`, data)
}

/** Delete a time switch. */
export function deleteTimeSwitch(id: number): Promise<null> {
  return del(`/time-switches/${id}`)
}
