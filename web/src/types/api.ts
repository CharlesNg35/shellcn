export interface ApiMeta {
  page?: number
  per_page?: number
  total?: number
  total_pages?: number
  [key: string]: unknown
}

export interface ApiErrorPayload {
  code: string
  message: string
  details?: Record<string, unknown>
}

export interface ApiSuccess<T> {
  success: true
  data: T
  meta?: ApiMeta
}

export interface ApiErrorResponse {
  success: false
  error: ApiErrorPayload
}

export type ApiResponse<T> = ApiSuccess<T> | ApiErrorResponse

export function isApiSuccess<T>(payload: ApiResponse<T>): payload is ApiSuccess<T> {
  return payload.success
}
