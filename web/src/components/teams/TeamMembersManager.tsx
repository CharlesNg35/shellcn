import { useEffect, useMemo, useState, type FormEvent } from 'react'
import { Loader2, Search, UserMinus, UserPlus } from 'lucide-react'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { PermissionGuard } from '@/components/permissions/PermissionGuard'
import { useUsers } from '@/hooks/useUsers'
import type { TeamMember, TeamRecord } from '@/types/teams'
import type { UserRecord } from '@/types/users'
import { cn } from '@/lib/utils/cn'
import type { UseMutationResult } from '@tanstack/react-query'
import { PERMISSIONS } from '@/constants/permissions'

type AddMemberMutation = UseMutationResult<boolean, unknown, { teamId: string; userId: string }>
type RemoveMemberMutation = UseMutationResult<boolean, unknown, { teamId: string; userId: string }>

interface TeamMembersManagerProps {
  team: TeamRecord
  members?: TeamMember[]
  isLoadingMembers?: boolean
  addMemberMutation: AddMemberMutation
  removeMemberMutation: RemoveMemberMutation
}

function filterAvailableUsers(users: UserRecord[], members: TeamMember[] | undefined) {
  if (!members?.length) {
    return users
  }
  const memberIds = new Set(members.map((member) => member.id))
  return users.filter((user) => !memberIds.has(user.id))
}

export function TeamMembersManager({
  team,
  members,
  isLoadingMembers,
  addMemberMutation,
  removeMemberMutation,
}: TeamMembersManagerProps) {
  const [searchTerm, setSearchTerm] = useState('')
  const [selectedUserId, setSelectedUserId] = useState<string>('')
  const [pendingRemovalId, setPendingRemovalId] = useState<string | null>(null)
  const [isAdding, setIsAdding] = useState(false)

  const userQueryParams = useMemo(() => {
    return {
      per_page: 25,
      status: 'all' as const,
      search: searchTerm.trim() ? searchTerm.trim() : undefined,
    }
  }, [searchTerm])

  const usersQuery = useUsers(userQueryParams, {
    enabled: Boolean(team.id),
    placeholderData: (previous) => previous ?? undefined,
  })

  const usersResult = usersQuery.data
  const isUsersLoading = usersQuery.isLoading

  const users = useMemo<UserRecord[]>(() => usersResult?.data ?? [], [usersResult?.data])

  const availableUsers = useMemo(() => filterAvailableUsers(users, members), [members, users])

  useEffect(() => {
    if (!availableUsers.length) {
      setSelectedUserId('')
      return
    }
    if (!selectedUserId || !availableUsers.some((user) => user.id === selectedUserId)) {
      setSelectedUserId(availableUsers[0]?.id ?? '')
    }
  }, [availableUsers, selectedUserId])

  const handleAddMember = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (!selectedUserId) {
      return
    }
    try {
      setIsAdding(true)
      await addMemberMutation.mutateAsync({
        teamId: team.id,
        userId: selectedUserId,
      })
    } finally {
      setIsAdding(false)
    }
  }

  const handleRemoveMember = async (userId: string) => {
    try {
      setPendingRemovalId(userId)
      await removeMemberMutation.mutateAsync({
        teamId: team.id,
        userId,
      })
    } finally {
      setPendingRemovalId(null)
    }
  }

  return (
    <div className="rounded-lg border border-border bg-card p-4 shadow-sm">
      <div className="flex flex-col gap-1 border-b border-border/70 pb-3">
        <div className="flex items-center justify-between">
          <h3 className="text-base font-semibold text-foreground">Members</h3>
          <Badge variant="outline" className="text-xs font-medium">
            {members?.length ?? 0} member{(members?.length ?? 0) === 1 ? '' : 's'}
          </Badge>
        </div>
        <p className="text-xs text-muted-foreground">
          Add or remove users from <span className="font-medium">{team.name}</span>
        </p>
      </div>

      <div className="mt-4 space-y-4">
        <PermissionGuard permission={PERMISSIONS.TEAM.MANAGE}>
          <form
            className="flex flex-col gap-3 rounded-lg border border-dashed border-border/80 bg-muted/30 p-3"
            onSubmit={handleAddMember}
          >
            <div className="flex items-center justify-between">
              <span className="text-sm font-medium text-foreground">Add member</span>
              <Badge variant="secondary" className="text-[10px] uppercase tracking-wide">
                Requires {PERMISSIONS.TEAM.MANAGE}
              </Badge>
            </div>

            <div className="grid gap-3 md:grid-cols-[1fr_auto]">
              <div className="flex flex-col gap-2">
                <label
                  className="text-xs font-medium text-muted-foreground"
                  htmlFor="member-search"
                >
                  Search users
                </label>
                <div className="relative">
                  <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                  <input
                    id="member-search"
                    type="text"
                    value={searchTerm}
                    placeholder="Search by name, username, or email"
                    onChange={(event) => setSearchTerm(event.target.value)}
                    className="h-10 w-full rounded-lg border border-input bg-background pl-10 pr-3 text-sm text-foreground transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                  />
                </div>
              </div>

              <div className="flex flex-col gap-2">
                <label
                  className="text-xs font-medium text-muted-foreground"
                  htmlFor="member-select"
                >
                  Select user
                </label>
                <select
                  id="member-select"
                  value={selectedUserId}
                  onChange={(event) => setSelectedUserId(event.target.value)}
                  className="h-10 rounded-lg border border-input bg-background px-3 text-sm text-foreground transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                  disabled={!availableUsers.length || isUsersLoading}
                >
                  {!availableUsers.length ? (
                    <option value="" disabled>
                      {isUsersLoading ? 'Loading users…' : 'No matching users available'}
                    </option>
                  ) : null}
                  {availableUsers.map((user) => (
                    <option key={user.id} value={user.id}>
                      {user.username} — {user.email}
                    </option>
                  ))}
                </select>
              </div>
            </div>

            <div className="flex justify-end">
              <Button
                type="submit"
                size="sm"
                className="flex items-center gap-2"
                disabled={!selectedUserId || isAdding}
              >
                {isAdding ? (
                  <Loader2 className="h-4 w-4 animate-spin" />
                ) : (
                  <UserPlus className="h-4 w-4" />
                )}
                Add member
              </Button>
            </div>
          </form>
        </PermissionGuard>

        <div className="space-y-2">
          {isLoadingMembers ? (
            <div className="flex items-center justify-center py-6 text-sm text-muted-foreground">
              <Loader2 className="mr-2 h-4 w-4 animate-spin" />
              Loading members…
            </div>
          ) : !members?.length ? (
            <div className="rounded-lg border border-dashed border-border/70 bg-muted/20 p-4 text-center text-sm text-muted-foreground">
              No members assigned yet.
            </div>
          ) : (
            members.map((member) => (
              <div
                key={member.id}
                className="flex items-center justify-between rounded-lg border border-border/70 bg-background px-3 py-2"
              >
                <div className="min-w-0">
                  <p className="truncate text-sm font-medium text-foreground">{member.username}</p>
                  <p className="truncate text-xs text-muted-foreground">{member.email}</p>
                </div>
                <div className="flex shrink-0 items-center gap-2">
                  <Badge
                    variant={member.is_active ? 'success' : 'secondary'}
                    className="text-[10px]"
                  >
                    {member.is_active ? 'Active' : 'Inactive'}
                  </Badge>
                  <PermissionGuard permission={PERMISSIONS.TEAM.MANAGE}>
                    <Button
                      type="button"
                      variant="ghost"
                      size="sm"
                      className={cn(
                        'text-destructive hover:text-destructive',
                        pendingRemovalId === member.id && 'pointer-events-none opacity-60'
                      )}
                      aria-label={`Remove ${member.username}`}
                      onClick={() => handleRemoveMember(member.id)}
                    >
                      {pendingRemovalId === member.id ? (
                        <Loader2 className="h-4 w-4 animate-spin" />
                      ) : (
                        <UserMinus className="h-4 w-4" />
                      )}
                    </Button>
                  </PermissionGuard>
                </div>
              </div>
            ))
          )}
        </div>
      </div>
    </div>
  )
}
