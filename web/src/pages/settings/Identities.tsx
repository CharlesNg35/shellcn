import { useMemo, useState } from 'react'
import { Plus } from 'lucide-react'
import { PageHeader } from '@/components/layout/PageHeader'
import { Button } from '@/components/ui/Button'
import { PermissionGuard } from '@/components/permissions/PermissionGuard'
import { IdentityFilters, type IdentityScopeFilter } from '@/components/vault/IdentityFilters'
import { IdentityTable } from '@/components/vault/IdentityTable'
import { IdentityFormModal } from '@/components/vault/IdentityFormModal'
import { IdentityDetailModal } from '@/components/vault/IdentityDetailModal'
import { IdentityShareModal } from '@/components/vault/IdentityShareModal'
import { useCredentialTemplates, useIdentities, useIdentityMutations } from '@/hooks/useIdentities'
import type { IdentityRecord } from '@/types/vault'
import { PERMISSIONS } from '@/constants/permissions'
import { toast } from '@/lib/utils/toast'

interface IdentityFiltersState {
  search: string
  scope: IdentityScopeFilter
  protocolId: string | 'all'
  includeConnectionScoped: boolean
}

const INITIAL_FILTERS: IdentityFiltersState = {
  search: '',
  scope: 'all',
  protocolId: 'all',
  includeConnectionScoped: false,
}

export function Identities() {
  const [filters, setFilters] = useState<IdentityFiltersState>(INITIAL_FILTERS)
  const [activeIdentityId, setActiveIdentityId] = useState<string | undefined>(undefined)
  const [formState, setFormState] = useState<{
    mode: 'create' | 'edit'
    identityId?: string
  } | null>(null)
  const [shareIdentityId, setShareIdentityId] = useState<string | undefined>(undefined)

  const identitiesQuery = useIdentities({
    scope: filters.scope === 'all' ? undefined : filters.scope,
    protocol_id: filters.protocolId === 'all' ? undefined : filters.protocolId,
    include_connection_scoped: filters.includeConnectionScoped,
  })
  const templatesQuery = useCredentialTemplates()
  const { remove } = useIdentityMutations()

  const templateNameMap = useMemo(() => {
    const map: Record<string, string> = {}
    templatesQuery.data?.forEach((template) => {
      map[template.id] = template.display_name
    })
    return map
  }, [templatesQuery.data])

  const protocolOptions = useMemo(() => {
    if (!templatesQuery.data?.length) {
      return []
    }
    const unique = new Map<string, string>()
    templatesQuery.data.forEach((template) => {
      if (!unique.has(template.driver_id)) {
        unique.set(template.driver_id, `${template.display_name} (${template.driver_id})`)
      }
    })
    return Array.from(unique.entries()).map(([id, name]) => ({ id, name }))
  }, [templatesQuery.data])

  const identities = useMemo(() => identitiesQuery.data ?? [], [identitiesQuery.data])
  const filteredIdentities = useMemo(() => {
    if (!filters.search) {
      return identities
    }
    const term = filters.search.toLowerCase().trim()
    return identities.filter((identity) => {
      const haystack = [identity.name, identity.description, identity.owner_user_id]
        .filter(Boolean)
        .join(' ')
        .toLowerCase()
      return haystack.includes(term)
    })
  }, [identities, filters.search])

  const handleDeleteIdentity = async (identity: IdentityRecord) => {
    const confirmed = window.confirm(`Delete identity “${identity.name}”? This cannot be undone.`)
    if (!confirmed) {
      return
    }
    try {
      await remove.mutateAsync(identity.id)
    } catch (error) {
      const message = error instanceof Error ? error.message : 'Unable to delete identity'
      toast.error('Failed to delete identity', { description: message })
    }
  }

  const handleFormSuccess = () => {
    setFormState(null)
  }

  return (
    <div className="space-y-6">
      <PageHeader
        title="Identities"
        description="Manage reusable credentials in the vault. Create, share, and monitor usage across protocols and teams."
        action={
          <PermissionGuard permission={PERMISSIONS.VAULT.CREATE}>
            <Button onClick={() => setFormState({ mode: 'create' })}>
              <Plus className="mr-1 h-4 w-4" />
              New identity
            </Button>
          </PermissionGuard>
        }
      />

      <IdentityFilters
        search={filters.search}
        scope={filters.scope}
        protocolId={filters.protocolId}
        includeConnectionScoped={filters.includeConnectionScoped}
        onSearchChange={(value) => setFilters((prev) => ({ ...prev, search: value }))}
        onScopeChange={(value) => setFilters((prev) => ({ ...prev, scope: value }))}
        onProtocolChange={(value) => setFilters((prev) => ({ ...prev, protocolId: value }))}
        onIncludeConnectionScopedChange={(value) =>
          setFilters((prev) => ({ ...prev, includeConnectionScoped: value }))
        }
        protocolOptions={protocolOptions}
      />

      <IdentityTable
        identities={filteredIdentities}
        isLoading={identitiesQuery.isLoading}
        templateNames={templateNameMap}
        onViewIdentity={(identity) => setActiveIdentityId(identity.id)}
        onEditIdentity={(identity) => setFormState({ mode: 'edit', identityId: identity.id })}
        onShareIdentity={(identity) => setShareIdentityId(identity.id)}
        onDeleteIdentity={handleDeleteIdentity}
      />

      <IdentityFormModal
        open={Boolean(formState)}
        mode={formState?.mode ?? 'create'}
        identityId={formState?.identityId}
        onClose={() => setFormState(null)}
        onSuccess={handleFormSuccess}
      />

      <IdentityDetailModal
        identityId={activeIdentityId}
        open={Boolean(activeIdentityId)}
        onClose={() => setActiveIdentityId(undefined)}
        onEditIdentity={(identity) => setFormState({ mode: 'edit', identityId: identity.id })}
        onShareIdentity={(identity) => setShareIdentityId(identity.id)}
      />

      <IdentityShareModal
        identityId={shareIdentityId}
        open={Boolean(shareIdentityId)}
        onClose={() => setShareIdentityId(undefined)}
      />
    </div>
  )
}
