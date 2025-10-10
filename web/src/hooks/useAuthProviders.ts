import { useMemo } from 'react'
import { useQuery, type UseQueryResult } from '@tanstack/react-query'
import { PERMISSIONS } from '@/constants/permissions'
import { authProvidersApi } from '@/lib/api/auth-providers'
import type { ApiError } from '@/lib/api/http'
import type {
  AuthProviderDetails,
  AuthProviderRecord,
  AuthProviderType,
  LDAPProviderConfig,
  OIDCProviderConfig,
  SAMLProviderConfig,
} from '@/types/auth-providers'
import { usePermissions } from './usePermissions'

export const AUTH_PROVIDERS_QUERY_KEY = ['auth-providers'] as const
export const AUTH_PROVIDER_DETAIL_QUERY_KEY = ['auth-providers', 'detail'] as const

export type AuthProviderScope = 'all' | 'enabled'

export function getAuthProvidersQueryKey(scope: AuthProviderScope) {
  return [...AUTH_PROVIDERS_QUERY_KEY, scope] as const
}

export function getAuthProviderDetailQueryKey(providerType: AuthProviderType) {
  return [...AUTH_PROVIDER_DETAIL_QUERY_KEY, providerType] as const
}

export function useAuthProviders(
  scope: AuthProviderScope | 'auto' = 'auto'
): UseQueryResult<AuthProviderRecord[], ApiError> {
  const { hasPermission } = usePermissions()

  const resolvedScope: AuthProviderScope = useMemo(() => {
    if (scope !== 'auto') {
      return scope
    }
    return hasPermission(PERMISSIONS.PERMISSION.MANAGE) ? 'all' : 'enabled'
  }, [hasPermission, scope])

  return useQuery<AuthProviderRecord[], ApiError>({
    queryKey: getAuthProvidersQueryKey(resolvedScope),
    queryFn: resolvedScope === 'all' ? authProvidersApi.getAll : authProvidersApi.getEnabled,
    staleTime: 60_000,
  })
}

type ProviderConfigFor<T extends AuthProviderType> = T extends 'oidc'
  ? OIDCProviderConfig
  : T extends 'saml'
    ? SAMLProviderConfig
    : T extends 'ldap'
      ? LDAPProviderConfig
      : undefined

export function useAuthProviderDetails(
  providerType: 'oidc',
  options?: { enabled?: boolean }
): UseQueryResult<AuthProviderDetails<OIDCProviderConfig>, ApiError>
export function useAuthProviderDetails(
  providerType: 'saml',
  options?: { enabled?: boolean }
): UseQueryResult<AuthProviderDetails<SAMLProviderConfig>, ApiError>
export function useAuthProviderDetails(
  providerType: 'ldap',
  options?: { enabled?: boolean }
): UseQueryResult<AuthProviderDetails<LDAPProviderConfig>, ApiError>
export function useAuthProviderDetails(
  providerType: 'local' | 'invite',
  options?: { enabled?: boolean }
): UseQueryResult<AuthProviderDetails<undefined>, ApiError>
export function useAuthProviderDetails<T extends AuthProviderType>(
  providerType: T,
  options?: { enabled?: boolean }
): UseQueryResult<AuthProviderDetails<ProviderConfigFor<T>>, ApiError> {
  return useQuery<AuthProviderDetails<ProviderConfigFor<T>>, ApiError>({
    queryKey: getAuthProviderDetailQueryKey(providerType),
    queryFn: () =>
      authProvidersApi.getDetails(providerType) as Promise<
        AuthProviderDetails<ProviderConfigFor<T>>
      >,
    enabled: options?.enabled ?? true,
    retry: (failureCount, error) => {
      if (error.status === 404) {
        return false
      }
      return failureCount < 2
    },
  })
}
