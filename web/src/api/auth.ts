import { get, post } from './client'
import type { AuthUser, HealthResponse, LoginRequest, LoginResponse, SetupRequest } from './types'

/** Check system health and whether setup is needed. */
export function getHealth(): Promise<HealthResponse> {
  return get<HealthResponse>('/health')
}

/** Log in with username and password. */
export function login(credentials: LoginRequest): Promise<LoginResponse> {
  return post<LoginResponse>('/auth/login', credentials)
}

/** Log out the current session. */
export function logout(): Promise<null> {
  return post<null>('/auth/logout')
}

/** Get the currently authenticated user. */
export function getMe(): Promise<AuthUser> {
  return get<AuthUser>('/auth/me')
}

/** Run first-boot setup. */
export function setup(data: SetupRequest): Promise<null> {
  return post<null>('/setup', data)
}
