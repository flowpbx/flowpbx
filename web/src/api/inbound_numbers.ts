import { get, post, put, del, list } from './client'
import type { InboundNumber, InboundNumberRequest, PaginatedResponse, PaginationParams } from './types'

/** List inbound numbers with pagination. */
export function listInboundNumbers(params?: PaginationParams): Promise<PaginatedResponse<InboundNumber>> {
  return list<InboundNumber>('/numbers', params)
}

/** Get a single inbound number by ID. */
export function getInboundNumber(id: number): Promise<InboundNumber> {
  return get<InboundNumber>(`/numbers/${id}`)
}

/** Create a new inbound number. */
export function createInboundNumber(data: InboundNumberRequest): Promise<InboundNumber> {
  return post<InboundNumber>('/numbers', data)
}

/** Update an existing inbound number. */
export function updateInboundNumber(id: number, data: InboundNumberRequest): Promise<InboundNumber> {
  return put<InboundNumber>(`/numbers/${id}`, data)
}

/** Delete an inbound number. */
export function deleteInboundNumber(id: number): Promise<null> {
  return del(`/numbers/${id}`)
}
