import { useEffect } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useShallow } from 'zustand/react/shallow'
import { fetchCurrentUser } from '@/lib/api/auth'
import type { AuthUser } from '@/types/auth'
import type { ApiError } from '@/lib/api/http'
import { useAuthStore } from '@/store/auth-store'

export const CURRENT_USER_QUERY_KEY = ['auth', 'current-user'] as const

export function useCurrentUser() {
  const { status, user, setUser } = useAuthStore(
    useShallow((state) => ({
      status: state.status,
      user: state.user,
      setUser: state.setUser,
    }))
  )

  const query = useQuery<AuthUser, ApiError, AuthUser, typeof CURRENT_USER_QUERY_KEY>({
    queryKey: CURRENT_USER_QUERY_KEY,
    queryFn: fetchCurrentUser,
    enabled: status === 'authenticated',
    initialData: user ?? undefined,
    staleTime: 5 * 60 * 1000,
    refetchOnMount: false,
    refetchOnWindowFocus: false,
  })

  useEffect(() => {
    if (query.data) {
      setUser(query.data)
    }
  }, [query.data, setUser])

  useEffect(() => {
    const err = query.error
    if (err && err.code === 'auth.unauthorized') {
      setUser(null)
    }
  }, [query.error, setUser])

  return query
}
