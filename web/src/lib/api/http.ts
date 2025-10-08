import type { AxiosResponse } from 'axios'
import type { ApiErrorPayload, ApiResponse } from '@/types/api'
import { isApiSuccess } from '@/types/api'

export class ApiError extends Error {
  readonly code: string
  readonly status?: number
  readonly details?: Record<string, unknown>

  constructor(payload: ApiErrorPayload, status?: number) {
    super(payload.message)
    this.name = 'ApiError'
    this.code = payload.code
    this.status = status
    this.details = payload.details
  }
}

export function ensureSuccess<T>(payload: ApiResponse<T>, status?: number): T {
  if (isApiSuccess(payload)) {
    return payload.data
  }

  throw new ApiError(payload.error, status)
}

export function unwrapResponse<T>(response: AxiosResponse<ApiResponse<T>>): T {
  return ensureSuccess(response.data, response.status)
}
