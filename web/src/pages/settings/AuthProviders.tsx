import { useMemo, useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Building, Key, Mail, Shield, ShieldCheck } from 'lucide-react'
import { PageHeader } from '@/components/layout/PageHeader'
import { ProviderCard } from '@/components/auth-providers/ProviderCard'
import { LocalSettingsForm } from '@/components/auth-providers/LocalSettingsForm'
import { InviteSettingsForm } from '@/components/auth-providers/InviteSettingsForm'
import { OIDCConfigForm } from '@/components/auth-providers/OIDCConfigForm'
import { SAMLConfigForm } from '@/components/auth-providers/SAMLConfigForm'
import { LDAPConfigForm } from '@/components/auth-providers/LDAPConfigForm'
import { Modal } from '@/components/ui/Modal'
import { PermissionGuard } from '@/components/permissions/PermissionGuard'
import { Skeleton } from '@/components/ui/Skeleton'
import { PERMISSIONS } from '@/constants/permissions'
import {
  AUTH_PROVIDERS_QUERY_KEY,
  getAuthProviderDetailQueryKey,
  useAuthProviders,
} from '@/hooks/useAuthProviders'
import type { AuthProviderRecord, AuthProviderType } from '@/types/auth-providers'
import { authProvidersApi } from '@/lib/api/auth-providers'
import { toApiError } from '@/lib/api/http'
import { toast } from '@/lib/utils/toast'

type ProviderDefinition = {
  type: AuthProviderType
  name: string
  description: string
  icon: typeof Key
  supportsTest?: boolean
  disableToggle?: boolean
}

const PROVIDER_DEFINITIONS: ProviderDefinition[] = [
  {
    type: 'local',
    name: 'Local Authentication',
    description: 'Built-in username and password authentication managed by ShellCN.',
    icon: Key,
    disableToggle: true,
  },
  {
    type: 'invite',
    name: 'Email Invitations',
    description: 'Provision new users via invitation links with optional verification.',
    icon: Mail,
  },
  {
    type: 'oidc',
    name: 'OpenID Connect',
    description: 'Standards-based single sign-on with providers like Azure AD or Okta.',
    icon: ShieldCheck,
    supportsTest: true,
  },
  {
    type: 'saml',
    name: 'SAML 2.0',
    description: 'Enterprise SAML federation for corporate identity providers.',
    icon: Shield,
  },
  {
    type: 'ldap',
    name: 'LDAP / Active Directory',
    description: 'Authenticate against LDAP directories and Active Directory forests.',
    icon: Building,
    supportsTest: true,
  },
]

function getProviderLabel(type: AuthProviderType): string {
  return PROVIDER_DEFINITIONS.find((definition) => definition.type === type)?.name ?? type
}

export function AuthProviders() {
  const queryClient = useQueryClient()
  const [activeModal, setActiveModal] = useState<AuthProviderType | null>(null)
  const [activeToggle, setActiveToggle] = useState<AuthProviderType | null>(null)
  const [activeTest, setActiveTest] = useState<AuthProviderType | null>(null)

  const providersQuery = useAuthProviders()

  const providerMap = useMemo(() => {
    const map = new Map<AuthProviderType, AuthProviderRecord>()
    for (const provider of providersQuery.data ?? []) {
      map.set(provider.type, provider)
    }
    return map
  }, [providersQuery.data])

  const toggleProviderMutation = useMutation({
    mutationFn: async ({ type, enabled }: { type: AuthProviderType; enabled: boolean }) => {
      await authProvidersApi.setEnabled(type, enabled)
      return { type, enabled }
    },
    onMutate: (variables) => {
      setActiveToggle(variables.type)
    },
    onSuccess: async (_, variables) => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: AUTH_PROVIDERS_QUERY_KEY }),
        queryClient.invalidateQueries({
          queryKey: getAuthProviderDetailQueryKey(variables.type),
        }),
      ])
      toast.success(
        `${getProviderLabel(variables.type)} ${variables.enabled ? 'enabled' : 'disabled'}`
      )
    },
    onError: (error, variables) => {
      const apiError = toApiError(error)
      toast.error(
        `Failed to ${variables.enabled ? 'enable' : 'disable'} ${getProviderLabel(variables.type)}`,
        {
          description: apiError.message,
        }
      )
    },
    onSettled: () => {
      setActiveToggle(null)
    },
  })

  const testProviderMutation = useMutation({
    mutationFn: async (type: AuthProviderType) => {
      await authProvidersApi.testConnection(type)
      return type
    },
    onMutate: (type) => {
      setActiveTest(type)
    },
    onSuccess: (type) => {
      toast.success(`${getProviderLabel(type)} connection succeeded`)
    },
    onError: (error, type) => {
      const apiError = toApiError(error)
      toast.error(`Test failed for ${getProviderLabel(type)}`, {
        description: apiError.message,
      })
    },
    onSettled: () => {
      setActiveTest(null)
    },
  })

  const handleToggle = (type: AuthProviderType, enabled: boolean) => {
    toggleProviderMutation.mutate({ type, enabled })
  }

  const handleTest = (type: AuthProviderType) => {
    testProviderMutation.mutate(type)
  }

  const loading = providersQuery.isLoading
  const loadError = providersQuery.isError ? providersQuery.error : null

  return (
    <PermissionGuard permission={PERMISSIONS.PERMISSION.MANAGE}>
      <div className="space-y-6">
        <PageHeader
          title="Authentication Providers"
          description="Enable and configure authentication options for your organization. Manage local login policies, invitation workflows, and external identity providers."
        />

        {loadError ? (
          <div className="rounded-lg border border-destructive/30 bg-destructive/10 px-4 py-3 text-sm text-destructive">
            Unable to load providers: {loadError.message}
          </div>
        ) : null}

        <div className="grid grid-cols-1 gap-6 md:grid-cols-2 xl:grid-cols-3">
          {loading
            ? PROVIDER_DEFINITIONS.map((definition) => (
                <Skeleton key={definition.type} className="h-56 rounded-lg" />
              ))
            : PROVIDER_DEFINITIONS.map((definition) => {
                const provider = providerMap.get(definition.type)
                const isToggleLoading = activeToggle === definition.type
                const isTestLoading = activeTest === definition.type

                return (
                  <ProviderCard
                    key={definition.type}
                    type={definition.type}
                    name={definition.name}
                    description={definition.description}
                    icon={definition.icon}
                    provider={provider}
                    onConfigure={() => setActiveModal(definition.type)}
                    onToggleEnabled={
                      definition.disableToggle
                        ? undefined
                        : (enabled) => handleToggle(definition.type, enabled)
                    }
                    toggleDisabled={!provider && definition.type !== 'invite'}
                    toggleLoading={isToggleLoading}
                    onTestConnection={
                      definition.supportsTest ? () => handleTest(definition.type) : undefined
                    }
                    testDisabled={!definition.supportsTest || !provider}
                    testLoading={isTestLoading}
                  />
                )
              })}
        </div>

        <Modal
          open={activeModal === 'local'}
          onClose={() => setActiveModal(null)}
          title="Local Authentication Settings"
          description="Control self-service registration, verification, and password recovery for local accounts."
        >
          <LocalSettingsForm
            provider={providerMap.get('local')}
            onCancel={() => setActiveModal(null)}
            onSuccess={() => setActiveModal(null)}
          />
        </Modal>

        <Modal
          open={activeModal === 'invite'}
          onClose={() => setActiveModal(null)}
          title="Invitation Settings"
          description="Customize the invitation experience for administrators inviting new users."
        >
          <InviteSettingsForm
            provider={providerMap.get('invite')}
            onCancel={() => setActiveModal(null)}
            onSuccess={() => setActiveModal(null)}
          />
        </Modal>

        <Modal
          open={activeModal === 'oidc'}
          onClose={() => setActiveModal(null)}
          title="Configure OpenID Connect"
          description="Connect ShellCN to an OpenID Connect identity provider."
          size="lg"
        >
          <OIDCConfigForm
            provider={providerMap.get('oidc')}
            onCancel={() => setActiveModal(null)}
            onSuccess={() => setActiveModal(null)}
          />
        </Modal>

        <Modal
          open={activeModal === 'saml'}
          onClose={() => setActiveModal(null)}
          title="Configure SAML 2.0"
          description="Set up SAML federation for enterprise identity providers."
          size="lg"
        >
          <SAMLConfigForm
            provider={providerMap.get('saml')}
            onCancel={() => setActiveModal(null)}
            onSuccess={() => setActiveModal(null)}
          />
        </Modal>

        <Modal
          open={activeModal === 'ldap'}
          onClose={() => setActiveModal(null)}
          title="Configure LDAP / Active Directory"
          description="Connect to LDAP directories or Active Directory for authentication."
          size="lg"
        >
          <LDAPConfigForm
            provider={providerMap.get('ldap')}
            onCancel={() => setActiveModal(null)}
            onSuccess={() => setActiveModal(null)}
          />
        </Modal>
      </div>
    </PermissionGuard>
  )
}
