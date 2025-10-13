import { useEffect, useMemo, useState, type FormEvent } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { formatDistanceToNow } from 'date-fns'
import { Loader2, Share2, Shield, Users, Clock } from 'lucide-react'
import { Modal } from '@/components/ui/Modal'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { Input } from '@/components/ui/Input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/Select'
import { useUsers } from '@/hooks/useUsers'
import { usePermissionRegistry } from '@/hooks/usePermissionRegistry'
import {
  createConnectionShare,
  deleteConnectionShare,
  fetchConnectionShares,
  type ConnectionSharePayload,
} from '@/lib/api/connections'
import { fetchProtocolPermissions } from '@/lib/api/protocols'
import type { ConnectionRecord } from '@/types/connections'
import type { TeamRecord } from '@/types/teams'
import type { PermissionDefinition } from '@/types/permission'
import { cn } from '@/lib/utils/cn'

const CONNECTION_SHARE_QUERY_KEY = (connectionId: string) =>
  ['connections', connectionId, 'shares'] as const

type SharePrincipalType = 'user' | 'team'

interface ShareConnectionModalProps {
  open: boolean
  connection: ConnectionRecord | null
  teams: TeamRecord[]
  onClose: () => void
  onShareUpdated?: () => void
}

export function ShareConnectionModal({
  open,
  connection,
  teams,
  onClose,
  onShareUpdated,
}: ShareConnectionModalProps) {
  const queryClient = useQueryClient()
  const [principalType, setPrincipalType] = useState<SharePrincipalType>('user')
  const [selectedUserId, setSelectedUserId] = useState<string>('')
  const [selectedTeamId, setSelectedTeamId] = useState<string>('')
  const [userSearch, setUserSearch] = useState<string>('')
  const [selectedScopes, setSelectedScopes] = useState<string[]>(['connection.view'])
  const [expiresAt, setExpiresAt] = useState<string>('')
  const [errorMessage, setErrorMessage] = useState<string>('')

  const registryQuery = usePermissionRegistry({ enabled: open })

  const shareQuery = useQuery({
    queryKey: connection ? CONNECTION_SHARE_QUERY_KEY(connection.id) : ['connections', 'shares'],
    queryFn: () => fetchConnectionShares(connection!.id),
    enabled: open && Boolean(connection),
    staleTime: 30_000,
  })

  const protocolPermissionsQuery = useQuery({
    queryKey: connection
      ? ['protocols', connection.protocol_id, 'permissions']
      : ['protocols', 'permissions'],
    queryFn: () => fetchProtocolPermissions(connection!.protocol_id),
    enabled: open && Boolean(connection?.protocol_id),
    staleTime: 5 * 60 * 1000,
  })

  const baseScopeIds = useMemo(
    () => ['connection.view', 'connection.launch', 'connection.manage'],
    []
  )

  const permissionOptions = useMemo(() => {
    const registry = registryQuery.data
    const protocolPermissions = protocolPermissionsQuery.data ?? []

    const baseOptions = baseScopeIds
      .map((id) => buildPermissionOption(id, registry))
      .filter(Boolean)

    const protocolOptions = protocolPermissions
      .map((permission) => buildPermissionOption(permission.id, registry))
      .filter(Boolean)

    const uniqueById = new Map<string, ReturnType<typeof buildPermissionOption>>()
    ;[...baseOptions, ...protocolOptions].forEach((option) => {
      if (option) {
        uniqueById.set(option.id, option)
      }
    })
    return Array.from(uniqueById.values()).filter(Boolean) as PermissionOption[]
  }, [baseScopeIds, registryQuery.data, protocolPermissionsQuery.data])

  const userQueryParams = useMemo(
    () => ({
      search: userSearch.trim() || undefined,
      status: 'all' as const,
      per_page: 25,
    }),
    [userSearch]
  )

  const usersQuery = useUsers(userQueryParams, {
    enabled: open && principalType === 'user',
    placeholderData: (previous) => previous ?? undefined,
    staleTime: 30_000,
  })

  const users = useMemo(() => usersQuery.data?.data ?? [], [usersQuery.data?.data])

  useEffect(() => {
    if (!open) {
      setPrincipalType('user')
      setSelectedUserId('')
      setSelectedTeamId('')
      setSelectedScopes(['connection.view'])
      setExpiresAt('')
      setUserSearch('')
      setErrorMessage('')
      return
    }

    if (!selectedTeamId && teams.length > 0) {
      setSelectedTeamId(teams[0].id)
    }
  }, [open, teams, selectedTeamId])

  useEffect(() => {
    if (principalType === 'user') {
      if (!selectedUserId && users.length > 0) {
        setSelectedUserId(users[0].id)
      } else if (selectedUserId && !users.some((user) => user.id === selectedUserId)) {
        setSelectedUserId(users[0]?.id ?? '')
      }
    }
  }, [principalType, selectedUserId, users])

  const createShareMutation = useMutation({
    mutationFn: async (payload: ConnectionSharePayload) => {
      if (!connection) {
        throw new Error('Connection not found')
      }
      const share = await createConnectionShare(connection.id, payload)
      await queryClient.invalidateQueries({
        queryKey: CONNECTION_SHARE_QUERY_KEY(connection.id),
      })
      onShareUpdated?.()
      return share
    },
    onError: (error: unknown) => {
      setErrorMessage(error instanceof Error ? error.message : 'Failed to create share')
    },
    onSuccess: () => {
      setErrorMessage('')
      setSelectedScopes(['connection.view'])
      setExpiresAt('')
      if (principalType === 'user') {
        setUserSearch('')
      }
    },
  })

  const deleteShareMutation = useMutation({
    mutationFn: async (shareId: string) => {
      if (!connection) {
        throw new Error('Connection not found')
      }
      await deleteConnectionShare(connection.id, shareId)
      await queryClient.invalidateQueries({
        queryKey: CONNECTION_SHARE_QUERY_KEY(connection.id),
      })
      onShareUpdated?.()
    },
    onError: (error: unknown) => {
      setErrorMessage(error instanceof Error ? error.message : 'Failed to revoke share')
    },
    onSuccess: () => {
      setErrorMessage('')
    },
  })

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (!connection) {
      setErrorMessage('Connection unavailable')
      return
    }

    const scopes = ensureViewScope(selectedScopes)

    const payload: ConnectionSharePayload = {
      permission_scopes: scopes,
    }

    if (principalType === 'user') {
      if (!selectedUserId) {
        setErrorMessage('Select a user to share with')
        return
      }
      payload.user_id = selectedUserId
      payload.team_id = null
    } else {
      if (!selectedTeamId) {
        setErrorMessage('Select a team to share with')
        return
      }
      payload.team_id = selectedTeamId
      payload.user_id = null
    }

    if (expiresAt) {
      const date = new Date(expiresAt)
      if (Number.isNaN(date.getTime())) {
        setErrorMessage('Expiration date is invalid')
        return
      }
      payload.expires_at = date.toISOString()
    }

    await createShareMutation.mutateAsync(payload)
  }

  const shares = shareQuery.data ?? []

  return (
    <Modal
      open={open}
      onClose={() => {
        if (!createShareMutation.isPending && !deleteShareMutation.isPending) {
          onClose()
        }
      }}
      title={connection ? `Share ${connection.name}` : 'Share connection'}
      description="Grant temporary or persistent access to this connection."
    >
      {!connection ? (
        <div className="py-8 text-center text-sm text-muted-foreground">
          Select a connection to manage shares.
        </div>
      ) : (
        <div className="space-y-6">
          <section className="space-y-3">
            <header className="flex items-center justify-between">
              <h3 className="text-sm font-semibold text-foreground">Active shares</h3>
              <Badge variant="outline" className="text-xs font-medium">
                {shareQuery.isLoading ? '…' : shares.length}
              </Badge>
            </header>
            {shareQuery.isLoading ? (
              <div className="flex items-center gap-2 rounded-md border border-border/60 bg-muted/20 px-3 py-2 text-sm text-muted-foreground">
                <Loader2 className="h-4 w-4 animate-spin" />
                Loading shares…
              </div>
            ) : shares.length === 0 ? (
              <p className="rounded-md border border-dashed border-border/60 bg-muted/20 px-3 py-2 text-xs text-muted-foreground">
                No active shares. Create one below to grant access.
              </p>
            ) : (
              <ul className="space-y-2">
                {shares.map((share) => (
                  <li
                    key={share.share_id}
                    className="flex flex-col gap-2 rounded-md border border-border/70 bg-card px-3 py-2 shadow-sm"
                  >
                    <div className="flex flex-wrap items-center justify-between gap-2">
                      <div className="flex items-center gap-2">
                        <Badge variant="secondary" className="text-[11px] uppercase tracking-wide">
                          {share.principal.type === 'team' ? 'Team' : 'User'}
                        </Badge>
                        <span className="text-sm font-medium text-foreground">
                          {share.principal.name}
                        </span>
                      </div>
                      <div className="flex items-center gap-2">
                        {share.granted_by && (
                          <span className="text-xs text-muted-foreground">
                            Shared by {share.granted_by.name}
                          </span>
                        )}
                        <Button
                          size="sm"
                          variant="outline"
                          className="text-xs"
                          onClick={() => deleteShareMutation.mutate(share.share_id)}
                          disabled={deleteShareMutation.isPending}
                        >
                          Revoke
                        </Button>
                      </div>
                    </div>
                    <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
                      <span className="flex items-center gap-1">
                        <Shield className="h-3 w-3" />
                        {share.permission_scopes.join(', ')}
                      </span>
                      {share.expires_at ? (
                        <span className="flex items-center gap-1">
                          <ClockIcon />
                          Expires{' '}
                          {formatDistanceToNow(new Date(share.expires_at), { addSuffix: true })}
                        </span>
                      ) : (
                        <span className="flex items-center gap-1">
                          <ClockIcon />
                          No expiry
                        </span>
                      )}
                    </div>
                  </li>
                ))}
              </ul>
            )}
          </section>

          <form className="space-y-4" onSubmit={handleSubmit}>
            <div className="space-y-2">
              <h3 className="text-sm font-semibold text-foreground">Create share</h3>
              <p className="text-xs text-muted-foreground">
                Choose whether to share with a specific user or a team. Shares can be revoked at any
                time.
              </p>
              <div className="flex gap-2">
                <Button
                  type="button"
                  size="sm"
                  variant={principalType === 'user' ? 'default' : 'outline'}
                  onClick={() => setPrincipalType('user')}
                >
                  <Users className="mr-2 h-3.5 w-3.5" /> User
                </Button>
                <Button
                  type="button"
                  size="sm"
                  variant={principalType === 'team' ? 'default' : 'outline'}
                  onClick={() => setPrincipalType('team')}
                >
                  <Share2 className="mr-2 h-3.5 w-3.5" /> Team
                </Button>
              </div>
            </div>

            {principalType === 'user' ? (
              <div className="space-y-2">
                <label className="text-xs font-medium text-muted-foreground" htmlFor="share-user">
                  Select user
                </label>
                <Input
                  id="share-user-search"
                  placeholder="Search users"
                  value={userSearch}
                  onChange={(event) => setUserSearch(event.target.value)}
                />
                <Select
                  value={selectedUserId}
                  onValueChange={setSelectedUserId}
                  disabled={usersQuery.isLoading || users.length === 0}
                >
                  <SelectTrigger
                    id="share-user"
                    className="h-10 w-full justify-between"
                    aria-label="Select user"
                  >
                    <SelectValue
                      placeholder={
                        usersQuery.isLoading
                          ? 'Loading users…'
                          : users.length === 0
                          ? 'No users found'
                          : 'Choose a user'
                      }
                    />
                  </SelectTrigger>
                  <SelectContent align="start">
                    {users.length === 0 ? (
                      <SelectItem value="" disabled>
                        {usersQuery.isLoading ? 'Loading users…' : 'No users found'}
                      </SelectItem>
                    ) : (
                      users.map((user) => (
                        <SelectItem key={user.id} value={user.id}>
                          {user.username} — {user.email}
                        </SelectItem>
                      ))
                    )}
                  </SelectContent>
                </Select>
              </div>
            ) : (
              <div className="space-y-2">
                <label className="text-xs font-medium text-muted-foreground" htmlFor="share-team">
                  Select team
                </label>
                <Select value={selectedTeamId} onValueChange={setSelectedTeamId}>
                  <SelectTrigger
                    id="share-team"
                    className="h-10 w-full justify-between"
                    aria-label="Select team"
                  >
                    <SelectValue
                      placeholder={teams.length === 0 ? 'No teams available' : 'Choose a team'}
                    />
                  </SelectTrigger>
                  <SelectContent align="start">
                    {teams.length === 0 ? (
                      <SelectItem value="" disabled>
                        No teams available
                      </SelectItem>
                    ) : (
                      teams.map((team) => (
                        <SelectItem key={team.id} value={team.id}>
                          {team.name}
                        </SelectItem>
                      ))
                    )}
                  </SelectContent>
                </Select>
              </div>
            )}

            <div className="space-y-2">
              <span className="text-xs font-medium text-muted-foreground">Permission scopes</span>
              <div className="grid gap-2">
                {permissionOptions.map((option) => {
                  const checked = selectedScopes.includes(option.id)
                  const disabled = option.id === 'connection.view'
                  return (
                    <label
                      key={option.id}
                      className={cn(
                        'flex items-start gap-3 rounded-md border border-border/60 bg-background px-3 py-2 text-sm shadow-sm transition hover:border-border',
                        disabled && 'opacity-80'
                      )}
                    >
                      <input
                        type="checkbox"
                        className="mt-1 h-4 w-4"
                        checked={checked}
                        disabled={disabled}
                        onChange={(event) => {
                          const { checked: isChecked } = event.target
                          setSelectedScopes((prev) => {
                            if (isChecked) {
                              return ensureViewScope([...prev, option.id])
                            }
                            if (disabled) {
                              return ensureViewScope(prev)
                            }
                            return ensureViewScope(prev.filter((scope) => scope !== option.id))
                          })
                        }}
                      />
                      <div>
                        <div className="font-medium text-foreground">{option.label}</div>
                        {option.description && (
                          <div className="text-xs text-muted-foreground">{option.description}</div>
                        )}
                      </div>
                    </label>
                  )
                })}
              </div>
            </div>

            <div className="space-y-2">
              <label className="text-xs font-medium text-muted-foreground" htmlFor="share-expiry">
                Expiration (optional)
              </label>
              <Input
                id="share-expiry"
                type="datetime-local"
                value={expiresAt}
                onChange={(event) => setExpiresAt(event.target.value)}
              />
              <p className="text-xs text-muted-foreground">
                Leave blank for no expiration. Recipients will lose access automatically when the
                share expires.
              </p>
            </div>

            {errorMessage && (
              <div className="rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-xs text-destructive">
                {errorMessage}
              </div>
            )}

            <div className="flex items-center justify-end gap-2">
              <Button
                type="button"
                variant="outline"
                onClick={onClose}
                disabled={createShareMutation.isPending}
              >
                Cancel
              </Button>
              <Button type="submit" disabled={createShareMutation.isPending}>
                {createShareMutation.isPending ? (
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                ) : null}
                Share connection
              </Button>
            </div>
          </form>
        </div>
      )}
    </Modal>
  )
}

interface PermissionOption {
  id: string
  label: string
  description?: string
}

function buildPermissionOption(
  id: string,
  registry?: Record<string, PermissionDefinition>
): PermissionOption {
  const definition = registry?.[id]
  return {
    id,
    label: definition?.display_name ?? definition?.description ?? id,
    description: definition?.description,
  }
}

function ensureViewScope(scopes: string[]): string[] {
  if (!scopes.includes('connection.view')) {
    return ['connection.view', ...scopes]
  }
  return Array.from(new Set(scopes))
}

function ClockIcon() {
  return <Clock className="h-3 w-3" />
}
