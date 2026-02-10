import { get, post, put, del } from './client'
import type { CallFlow, CallFlowRequest, FlowValidationResult } from './types'

/** List all call flows. */
export function listFlows(): Promise<CallFlow[]> {
  return get<CallFlow[]>('/flows')
}

/** Get a single call flow by ID. */
export function getFlow(id: number): Promise<CallFlow> {
  return get<CallFlow>(`/flows/${id}`)
}

/** Create a new call flow. */
export function createFlow(data: CallFlowRequest): Promise<CallFlow> {
  return post<CallFlow>('/flows', data)
}

/** Update an existing call flow. */
export function updateFlow(id: number, data: CallFlowRequest): Promise<CallFlow> {
  return put<CallFlow>(`/flows/${id}`, data)
}

/** Delete a call flow. */
export function deleteFlow(id: number): Promise<null> {
  return del(`/flows/${id}`)
}

/** Publish a call flow for live routing. */
export function publishFlow(id: number): Promise<CallFlow> {
  return post<CallFlow>(`/flows/${id}/publish`)
}

/** Validate a call flow graph structure. */
export function validateFlow(id: number): Promise<FlowValidationResult> {
  return post<FlowValidationResult>(`/flows/${id}/validate`)
}
