import { useEffect, useMemo, useState, type FormEvent } from 'react'
import { Loader2, ShieldCheck, Crown, Key, Users } from 'lucide-react'

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
import {
  useSessionParticipants,
  useSessionParticipantMutations,
} from '@/hooks/useSessionParticipants'
import type { ActiveConnectionSession, ActiveSessionParticipant } from '@/types/connections'

interface SessionShareDialogProps {
  sessionId: string
  open: boolean
  onClose: () => void
  session?: ActiveConnectionSession
  currentUserId?: string
  canShare: boolean
  canGrantWrite: boolean
}

export function SessionShareDialog({
  sessionId,
  open,
  onClose,
  session,
  currentUserId,
  canShare,
  canGrantWrite,
}: SessionShareDialogProps) {
  const [search, setSearch] = useState('')
  const [selectedUserId, setSelectedUserId] = useState('')

  const participantsQuery = useSessionParticipants(sessionId, { enabled: open })
  const mutations = useSessionParticipantMutations(sessionId)

  const isOwner = useMemo(
    () => (session && currentUserId ? session.owner_user_id === currentUserId : false),
    [session, currentUserId]
  )

  useEffect(() => {
    if (!open) {
      setSearch('')
      setSelectedUserId('')
    }
  }, [open])

  const usersQuery = useUsers(
    { search: search.trim() || undefined, status: 'active', per_page: 25 },
    { enabled: open && (canShare || isOwner), placeholderData: (prev) => prev ?? undefined }
  )

  const users = usersQuery.data?.data

  useEffect(() => {
    if (!open) {
      return
    }
    if (!selectedUserId && users && users.length > 0) {
      setSelectedUserId(users[0]!.id)
    }
    if (selectedUserId && users && users.every((user) => user.id !== selectedUserId)) {
      setSelectedUserId(users[0]?.id ?? '')
    }
  }, [open, selectedUserId, users])

  const participants = participantsQuery.data?.participants ?? []
  const writeHolder = participants.find((participant) => participant.is_write_holder)

  const canManageParticipants = canShare || isOwner
  const canAssignWrite = canGrantWrite || isOwner

  const handleInvite = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (!selectedUserId || !canManageParticipants) {
      return
    }
    await mutations.invite.mutateAsync({ user_id: selectedUserId })
  }

  const handleGrantWrite = (participant: ActiveSessionParticipant) => {
    mutations.grantWrite.mutate({ userId: participant.user_id })
  }

  const handleRelinquish = (participant: ActiveSessionParticipant) => {
    mutations.relinquishWrite.mutate({ userId: participant.user_id })
  }

  const handleRemove = (participant: ActiveSessionParticipant) => {
    mutations.remove.mutate({ userId: participant.user_id })
  }

  const isLoading = participantsQuery.isLoading

  return (
    <Modal
      open={open}
      onClose={() => {
        if (
          !mutations.invite.isPending &&
          !mutations.remove.isPending &&
          !mutations.grantWrite.isPending &&
          !mutations.relinquishWrite.isPending
        ) {
          onClose()
        }
      }}
      title="Session participants"
      description="Manage who can observe or control this session."
      size="lg"
    >
      <div className="flex flex-col gap-6">
        {isLoading ? (
          <div className="flex items-center justify-center gap-2 text-muted-foreground">
            <Loader2 className="h-4 w-4 animate-spin" aria-hidden />
            Loading participants…
          </div>
        ) : participants.length === 0 ? (
          <div className="rounded-md border border-dashed border-border p-4 text-sm text-muted-foreground">
            No participants yet. Invite a teammate to collaborate in real time.
          </div>
        ) : (
          <div className="space-y-3">
            {participants.map((participant) => {
              const isCurrentUser = currentUserId === participant.user_id
              const canRemoveParticipant = canManageParticipants || isCurrentUser
              const canRelinquish = participant.is_write_holder && (isCurrentUser || canAssignWrite)
              const canGrant =
                !participant.is_write_holder &&
                canAssignWrite &&
                participant.user_id !== session?.owner_user_id

              return (
                <div
                  key={participant.user_id}
                  className="flex flex-wrap items-center justify-between gap-3 rounded-md border border-border/60 bg-muted/40 px-4 py-3"
                >
                  <div className="flex min-w-0 flex-auto items-center gap-3">
                    <div className="flex h-10 w-10 items-center justify-center rounded-full bg-primary/10 text-sm font-semibold text-primary">
                      {initials(participant.user_name ?? participant.user_id)}
                    </div>
                    <div className="min-w-0">
                      <p className="truncate font-medium text-foreground">
                        {participant.user_name ?? participant.user_id}
                      </p>
                      <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
                        <span className="uppercase tracking-wide">{participant.access_mode}</span>
                        {participant.is_owner && (
                          <Badge variant="secondary" className="flex items-center gap-1">
                            <Crown className="h-3 w-3" /> Owner
                          </Badge>
                        )}
                        {participant.is_write_holder && (
                          <Badge variant="default" className="flex items-center gap-1">
                            <Key className="h-3 w-3" /> Write access
                          </Badge>
                        )}
                        {isCurrentUser && (
                          <Badge variant="outline" className="flex items-center gap-1">
                            <ShieldCheck className="h-3 w-3" /> You
                          </Badge>
                        )}
                      </div>
                    </div>
                  </div>
                  <div className="flex flex-wrap items-center gap-2">
                    {canGrant && (
                      <Button
                        size="sm"
                        variant="ghost"
                        disabled={mutations.grantWrite.isPending}
                        onClick={() => handleGrantWrite(participant)}
                      >
                        Grant write
                      </Button>
                    )}
                    {canRelinquish && (
                      <Button
                        size="sm"
                        variant="ghost"
                        disabled={mutations.relinquishWrite.isPending}
                        onClick={() => handleRelinquish(participant)}
                      >
                        Relinquish
                      </Button>
                    )}
                    {canRemoveParticipant && !participant.is_owner && (
                      <Button
                        size="sm"
                        variant="ghost"
                        disabled={mutations.remove.isPending}
                        onClick={() => handleRemove(participant)}
                      >
                        Remove
                      </Button>
                    )}
                  </div>
                </div>
              )
            })}
          </div>
        )}

        {(canManageParticipants || isOwner) && (
          <form onSubmit={handleInvite} className="space-y-3">
            <div className="flex items-center gap-2 text-sm text-muted-foreground">
              <Users className="h-4 w-4" /> Invite teammate
            </div>
            <div className="grid gap-3 sm:grid-cols-3">
              <div className="sm:col-span-2">
                <Input
                  placeholder="Search by name or email"
                  value={search}
                  onChange={(event) => setSearch(event.target.value)}
                  disabled={usersQuery.isLoading}
                />
              </div>
              <div>
                <Select
                  value={selectedUserId}
                  onValueChange={setSelectedUserId}
                  disabled={usersQuery.isLoading}
                >
                  <SelectTrigger>
                    <SelectValue
                      placeholder={usersQuery.isLoading ? 'Loading users…' : 'Select user'}
                    />
                  </SelectTrigger>
                  <SelectContent>
                    {usersQuery.isLoading ? (
                      <SelectItem value="loading">Loading…</SelectItem>
                    ) : !users || users.length === 0 ? (
                      <SelectItem value="none" disabled>
                        No matches found
                      </SelectItem>
                    ) : (
                      users.map((user) => (
                        <SelectItem key={user.id} value={user.id}>
                          {user.first_name || user.last_name
                            ? `${user.first_name ?? ''} ${user.last_name ?? ''}`.trim() ||
                              user.username
                            : user.username}{' '}
                          <span className="text-xs text-muted-foreground">({user.email})</span>
                        </SelectItem>
                      ))
                    )}
                  </SelectContent>
                </Select>
              </div>
            </div>
            <div className="flex justify-end">
              <Button
                type="submit"
                disabled={
                  mutations.invite.isPending || !selectedUserId || selectedUserId === 'loading'
                }
              >
                {mutations.invite.isPending ? (
                  <>
                    <Loader2 className="mr-2 h-4 w-4 animate-spin" /> Adding…
                  </>
                ) : (
                  'Invite participant'
                )}
              </Button>
            </div>
          </form>
        )}

        {writeHolder && (
          <div className="rounded-md border border-border/80 bg-muted/40 px-4 py-3 text-sm text-muted-foreground">
            <span className="font-medium text-foreground">Current write access:</span>{' '}
            {writeHolder.user_name ?? writeHolder.user_id}
          </div>
        )}
      </div>
    </Modal>
  )
}

function initials(label: string) {
  const trimmed = label.trim()
  if (!trimmed) {
    return '?'
  }
  const parts = trimmed.split(/\s+/)
  const letters = parts.slice(0, 2).map((part) => part.charAt(0).toUpperCase())
  return letters.join('') || trimmed.charAt(0).toUpperCase()
}
