import axios, { type AxiosError, type AxiosResponse } from 'axios'
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

function isApiEnvelope(value: unknown): value is ApiResponse<unknown> {
  return typeof value === 'object' && value !== null && 'success' in value
}

function apiErrorFromAxios(error: AxiosError): ApiError {
  const response = error.response as AxiosResponse<ApiResponse<unknown>> | undefined

  if (!response) {
    const message = error.message || 'Unable to reach the server'
    return new ApiError(
      {
        code: 'NETWORK_ERROR',
        message,
      },
      undefined
    )
  }

  const { status, statusText, data } = response

  if (isApiEnvelope(data) && !data.success) {
    return new ApiError(data.error, status)
  }

  const message = statusText || error.message || `Request failed with status ${status}`
  return new ApiError(
    {
      code: status ? `HTTP_${status}` : 'HTTP_ERROR',
      message,
    },
    status
  )
}

export function toApiError(error: unknown): ApiError {
  if (error instanceof ApiError) {
    return error
  }

  if (axios.isAxiosError(error)) {
    return apiErrorFromAxios(error)
  }

  if (error instanceof Error) {
    return new ApiError({
      code: 'UNKNOWN_ERROR',
      message: error.message,
    })
  }

  return new ApiError({
    code: 'UNKNOWN_ERROR',
    message: 'An unexpected error occurred',
  })
}
