import { useQuery, type UseQueryOptions } from '@tanstack/react-query'
import { useShallow } from 'zustand/react/shallow'
import { fetchCurrentUser } from '@/lib/api/auth'
import type { AuthUser } from '@/types/auth'
import type { ApiError } from '@/lib/api/http'
import { useAuthStore } from '@/store/auth-store'

export const CURRENT_USER_QUERY_KEY = ['auth', 'current-user'] as const

type CurrentUserQueryOptions = Omit<
  UseQueryOptions<AuthUser, ApiError, AuthUser>,
  'queryKey' | 'queryFn'
>

export function useCurrentUser(options?: CurrentUserQueryOptions) {
  const { status, user, setUser } = useAuthStore(
    useShallow((state) => ({
      status: state.status,
      user: state.user,
      setUser: state.setUser,
    }))
  )

  return useQuery<AuthUser, ApiError>({
    queryKey: CURRENT_USER_QUERY_KEY,
    queryFn: fetchCurrentUser,
    enabled: status === 'authenticated',
    initialData: user ?? undefined,
    staleTime: 5 * 60 * 1000,
    refetchOnMount: false,
    refetchOnWindowFocus: false,
    ...options,
    onSuccess: (data) => {
      setUser(data)
      options?.onSuccess?.(data)
    },
    onError: (error) => {
      if (error.code === 'auth.unauthorized') {
        setUser(null)
      }
      options?.onError?.(error)
    },
  })
}
