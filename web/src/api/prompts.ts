import { get, del } from './client'
import type { AudioPrompt } from './types'

/** List all audio prompts. */
export function listPrompts(): Promise<AudioPrompt[]> {
  return get<AudioPrompt[]>('/prompts')
}

/** Upload an audio prompt via multipart form data. */
export async function uploadPrompt(file: File, name?: string): Promise<AudioPrompt> {
  const formData = new FormData()
  formData.append('file', file)
  if (name) {
    formData.append('name', name)
  }

  // Use raw fetch for multipart upload (the JSON client sets Content-Type).
  const csrf = document.cookie
    .split('; ')
    .find((row) => row.startsWith('flowpbx_csrf='))
  const csrfToken = csrf ? csrf.split('=')[1] : null

  const headers: Record<string, string> = { Accept: 'application/json' }
  if (csrfToken) {
    headers['X-CSRF-Token'] = csrfToken
  }

  const res = await fetch('/api/v1/prompts', {
    method: 'POST',
    headers,
    credentials: 'same-origin',
    body: formData,
  })

  if (res.status === 401) {
    window.location.href = '/login'
    throw new Error('authentication required')
  }

  const envelope = await res.json()

  if (!res.ok || envelope.error) {
    throw new Error(envelope.error ?? `upload failed with status ${res.status}`)
  }

  return envelope.data as AudioPrompt
}

/** Delete an audio prompt. */
export function deletePrompt(id: number): Promise<null> {
  return del(`/prompts/${id}`)
}

/** Build the audio playback URL for a prompt. */
export function promptAudioURL(id: number): string {
  return `/api/v1/prompts/${id}/audio`
}
