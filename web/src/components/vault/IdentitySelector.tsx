import { useMemo, useState } from 'react'
import { Search } from 'lucide-react'
import { Input } from '@/components/ui/Input'
import { Button } from '@/components/ui/Button'
import { IdentityScopeBadge } from '@/components/vault/IdentityScopeBadge'
import { useIdentities } from '@/hooks/useIdentities'
import type { IdentityRecord, IdentityScope } from '@/types/vault'
import { cn } from '@/lib/utils/cn'

interface IdentitySelectorProps {
  value: string | null
  onChange: (value: string | null) => void
  protocolId?: string
  scope?: IdentityScope | 'all'
  includeConnectionScoped?: boolean
  disabled?: boolean
  placeholder?: string
  allowInlineCreate?: boolean
  onCreateIdentity?: () => void
}

function matchesSearch(identity: IdentityRecord, term: string) {
  if (!term) {
    return true
  }
  const haystack =
    `${identity.name} ${identity.description ?? ''} ${identity.owner_user_id}`.toLowerCase()
  return haystack.includes(term.toLowerCase())
}

export function IdentitySelector({
  value,
  onChange,
  protocolId,
  scope = 'all',
  includeConnectionScoped = false,
  disabled,
  placeholder = 'Select identity',
  allowInlineCreate,
  onCreateIdentity,
}: IdentitySelectorProps) {
  const [search, setSearch] = useState('')
  const identitiesQuery = useIdentities(
    {
      scope: scope === 'all' ? undefined : scope,
      protocol_id: protocolId,
      include_connection_scoped: includeConnectionScoped,
    },
    { enabled: !disabled }
  )
  const identities = useMemo(() => identitiesQuery.data ?? [], [identitiesQuery.data])

  const filteredIdentities = useMemo(() => {
    if (!search) {
      return identities
    }
    return identities.filter((identity) => matchesSearch(identity, search))
  }, [identities, search])

  const selectedIdentity = value ? identities.find((identity) => identity.id === value) : null

  return (
    <div className="space-y-3">
      <div className="flex flex-col gap-2">
        <div className="relative">
          <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            placeholder="Search identities by name, description, or owner"
            className="pl-9"
            disabled={disabled}
          />
        </div>
        <div className="max-h-56 overflow-y-auto rounded-md border border-border bg-card">
          <button
            type="button"
            className={cn(
              'flex w-full items-center justify-between gap-3 border-b border-border px-4 py-2 text-left text-sm transition hover:bg-muted',
              !value ? 'bg-muted/70' : ''
            )}
            onClick={() => onChange(null)}
            disabled={disabled}
          >
            <span>{placeholder}</span>
          </button>
          {filteredIdentities.map((identity) => (
            <button
              key={identity.id}
              type="button"
              className={cn(
                'flex w-full items-center justify-between gap-3 px-4 py-2 text-left text-sm transition hover:bg-muted',
                value === identity.id ? 'bg-muted/70 font-semibold' : ''
              )}
              onClick={() => onChange(identity.id)}
              disabled={disabled}
            >
              <div className="flex flex-col">
                <span>{identity.name}</span>
                <span className="text-xs text-muted-foreground">
                  {identity.description || identity.owner_user_id}
                </span>
              </div>
              <IdentityScopeBadge scope={identity.scope} />
            </button>
          ))}
          {!filteredIdentities.length ? (
            <div className="px-4 py-3 text-sm text-muted-foreground">No identities found.</div>
          ) : null}
        </div>
      </div>

      {allowInlineCreate ? (
        <Button type="button" variant="outline" onClick={onCreateIdentity} disabled={disabled}>
          Create new identity
        </Button>
      ) : null}

      {selectedIdentity ? (
        <div className="rounded-md border border-dashed border-border bg-muted/40 p-3 text-xs text-muted-foreground">
          <p className="font-medium text-foreground">Selected identity</p>
          <p>{selectedIdentity.name}</p>
          <p>Usage count: {selectedIdentity.usage_count}</p>
          <p>Connections: {selectedIdentity.connection_count}</p>
        </div>
      ) : null}
    </div>
  )
}
