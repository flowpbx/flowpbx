import { get, post, put, del } from './client'
import type { IVRMenu, IVRMenuRequest } from './types'

/** List all IVR menus. */
export function listIVRMenus(): Promise<IVRMenu[]> {
  return get<IVRMenu[]>('/ivr-menus')
}

/** Get a single IVR menu by ID. */
export function getIVRMenu(id: number): Promise<IVRMenu> {
  return get<IVRMenu>(`/ivr-menus/${id}`)
}

/** Create a new IVR menu. */
export function createIVRMenu(data: IVRMenuRequest): Promise<IVRMenu> {
  return post<IVRMenu>('/ivr-menus', data)
}

/** Update an existing IVR menu. */
export function updateIVRMenu(id: number, data: IVRMenuRequest): Promise<IVRMenu> {
  return put<IVRMenu>(`/ivr-menus/${id}`, data)
}

/** Delete an IVR menu. */
export function deleteIVRMenu(id: number): Promise<null> {
  return del(`/ivr-menus/${id}`)
}
