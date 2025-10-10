import { useEffect, useMemo, useState } from 'react'
import { CalendarClock, Loader2, PencilLine, Plus, Trash2 } from 'lucide-react'
import { Button } from '@/components/ui/Button'
import { Modal } from '@/components/ui/Modal'
import { PermissionGuard } from '@/components/permissions/PermissionGuard'
import { TeamTable } from '@/components/teams/TeamTable'
import { TeamForm, type TeamFormMode } from '@/components/teams/TeamForm'
import { TeamMembersManager } from '@/components/teams/TeamMembersManager'
import { TeamHierarchy } from '@/components/teams/TeamHierarchy'
import { useTeam, useTeamMembers, useTeamMutations, useTeams } from '@/hooks/useTeams'
import type { TeamRecord } from '@/types/teams'
import { Badge } from '@/components/ui/Badge'

function formatDate(value?: string) {
  if (!value) {
    return '—'
  }
  try {
    const date = new Date(value)
    return date.toLocaleString()
  } catch {
    return value
  }
}

export function Teams() {
  const [selectedTeamId, setSelectedTeamId] = useState<string | null>(null)
  const [formMode, setFormMode] = useState<TeamFormMode>('create')
  const [teamForForm, setTeamForForm] = useState<TeamRecord | undefined>()
  const [isModalOpen, setIsModalOpen] = useState(false)
  const [memberCounts, setMemberCounts] = useState<Record<string, number>>({})

  const { data: teamsResult, isLoading: isTeamsLoading } = useTeams()
  const teams = useMemo(() => teamsResult?.data ?? [], [teamsResult?.data])

  const { data: teamDetail, isLoading: isTeamDetailLoading } = useTeam(
    selectedTeamId ?? undefined,
    {
      enabled: Boolean(selectedTeamId),
    }
  )

  const { data: members, isLoading: isMembersLoading } = useTeamMembers(
    selectedTeamId ?? undefined,
    {
      enabled: Boolean(selectedTeamId),
    }
  )

  const teamMutations = useTeamMutations()

  useEffect(() => {
    if (!teams.length) {
      setSelectedTeamId(null)
      return
    }

    setSelectedTeamId((current) => {
      if (current && teams.some((team) => team.id === current)) {
        return current
      }
      return teams[0]?.id ?? null
    })
  }, [teams])

  useEffect(() => {
    if (!selectedTeamId) {
      return
    }
    const detailMembers = teamDetail?.members
    if (!detailMembers) {
      return
    }

    setMemberCounts((prev) => {
      if (typeof prev[selectedTeamId] !== 'undefined') {
        return prev
      }
      return {
        ...prev,
        [selectedTeamId]: detailMembers.length,
      }
    })
  }, [selectedTeamId, teamDetail])

  useEffect(() => {
    if (selectedTeamId && members) {
      setMemberCounts((prev) => ({
        ...prev,
        [selectedTeamId]: members.length,
      }))
    }
  }, [selectedTeamId, members])

  const selectedTeam = useMemo(() => {
    if (!selectedTeamId) {
      return undefined
    }
    return teams.find((team) => team.id === selectedTeamId)
  }, [teams, selectedTeamId])

  const handleOpenCreateModal = () => {
    setFormMode('create')
    setTeamForForm(undefined)
    setIsModalOpen(true)
  }

  const handleOpenEditModal = (team: TeamRecord) => {
    setFormMode('edit')
    setTeamForForm(team)
    setIsModalOpen(true)
  }

  const handleCloseModal = () => {
    setIsModalOpen(false)
    setTeamForForm(undefined)
  }

  const handleFormSuccess = (team: TeamRecord) => {
    setTeamForForm(undefined)
    setSelectedTeamId(team.id)
    setMemberCounts((prev) => ({
      ...prev,
      [team.id]: team.members?.length ?? prev[team.id] ?? 0,
    }))
  }

  const handleDeleteTeam = async (team: TeamRecord) => {
    const confirmed = window.confirm(
      `Delete team "${team.name}"? This will remove team membership assignments.`
    )
    if (!confirmed) {
      return
    }

    try {
      await teamMutations.remove.mutateAsync(team.id)
      setMemberCounts((prev) => {
        if (!(team.id in prev)) {
          return prev
        }
        const next = { ...prev }
        delete next[team.id]
        return next
      })
      setSelectedTeamId((current) => {
        if (current !== team.id) {
          return current
        }
        const remainingTeams = teams.filter((item) => item.id !== team.id)
        return remainingTeams[0]?.id ?? null
      })
    } catch (error) {
      console.error(error)
    }
  }

  const modalTitle = formMode === 'create' ? 'Create team' : 'Edit team'
  const modalDescription =
    formMode === 'create'
      ? 'Define a new team to group users and manage permissions collectively.'
      : 'Update the team name or description. Changes apply immediately.'

  const teamInfo = teamDetail ?? teamForForm ?? selectedTeam

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div>
          <h1 className="text-3xl font-bold text-foreground">Teams</h1>
          <p className="mt-2 max-w-2xl text-sm text-muted-foreground">
            Organize users into teams to streamline permission assignment and access control. Teams
            can be nested by using “/” separated names (for example, Security/Incident Response).
          </p>
        </div>
        <PermissionGuard permission="team.manage">
          <Button onClick={handleOpenCreateModal}>
            <Plus className="mr-2 h-4 w-4" />
            Create Team
          </Button>
        </PermissionGuard>
      </div>

      <div className="grid gap-6 xl:grid-cols-[2fr,3fr]">
        <div className="space-y-6">
          <TeamTable
            teams={teams}
            selectedTeamId={selectedTeamId ?? undefined}
            isLoading={isTeamsLoading}
            memberCounts={memberCounts}
            onSelectTeam={(teamId) => setSelectedTeamId(teamId)}
            onEditTeam={(team) => handleOpenEditModal(team)}
            onDeleteTeam={(team) => handleDeleteTeam(team)}
            emptyAction={
              <PermissionGuard permission="team.manage">
                <Button onClick={handleOpenCreateModal}>
                  <Plus className="mr-2 h-4 w-4" />
                  Create Team
                </Button>
              </PermissionGuard>
            }
          />

          <TeamHierarchy
            teams={teams}
            selectedTeamId={selectedTeamId ?? undefined}
            memberCounts={memberCounts}
            onSelectTeam={(teamId) => setSelectedTeamId(teamId)}
          />
        </div>

        <div className="space-y-6">
          {!selectedTeamId ? (
            <div className="rounded-lg border border-dashed border-border bg-muted/20 p-8 text-center text-sm text-muted-foreground">
              Select a team to view details and manage memberships.
            </div>
          ) : (
            <>
              <div className="rounded-lg border border-border bg-card p-5 shadow-sm">
                <div className="flex flex-col gap-4 md:flex-row md:items-start md:justify-between">
                  <div className="space-y-1.5">
                    <div className="flex items-center gap-2">
                      <h2 className="text-2xl font-semibold text-foreground">
                        {teamInfo?.name ?? 'Team details'}
                      </h2>
                      {teamInfo?.members?.some((member) => member.is_root) ? (
                        <Badge variant="destructive" className="text-[10px]">
                          Root member
                        </Badge>
                      ) : null}
                    </div>
                    <p className="text-sm text-muted-foreground">
                      {teamInfo?.description?.trim()?.length
                        ? teamInfo.description
                        : 'No description provided for this team.'}
                    </p>
                  </div>

                  <PermissionGuard permission="team.manage">
                    <div className="flex flex-wrap gap-2">
                      {(teamDetail ?? selectedTeam) ? (
                        <>
                          <Button
                            type="button"
                            variant="outline"
                            size="sm"
                            onClick={() => {
                              const editable = teamDetail ?? selectedTeam
                              if (editable) {
                                handleOpenEditModal(editable)
                              }
                            }}
                          >
                            <PencilLine className="mr-2 h-4 w-4" />
                            Edit
                          </Button>
                          <Button
                            type="button"
                            variant="outline"
                            size="sm"
                            className="text-destructive hover:text-destructive"
                            onClick={() => {
                              const removable = teamDetail ?? selectedTeam
                              if (removable) {
                                void handleDeleteTeam(removable)
                              }
                            }}
                            disabled={teamMutations.remove.isPending}
                          >
                            {teamMutations.remove.isPending ? (
                              <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                            ) : (
                              <Trash2 className="mr-2 h-4 w-4" />
                            )}
                            Delete
                          </Button>
                        </>
                      ) : null}
                    </div>
                  </PermissionGuard>
                </div>

                <div className="mt-5 grid gap-3 rounded-lg bg-muted/20 p-4 text-sm text-muted-foreground md:grid-cols-2">
                  <div className="flex items-center gap-2">
                    <CalendarClock className="h-4 w-4" />
                    <div>
                      <p className="text-xs uppercase tracking-wide text-muted-foreground/80">
                        Created
                      </p>
                      <p className="font-medium text-foreground">
                        {formatDate(teamInfo?.created_at)}
                      </p>
                    </div>
                  </div>
                  <div className="flex items-center gap-2">
                    <CalendarClock className="h-4 w-4" />
                    <div>
                      <p className="text-xs uppercase tracking-wide text-muted-foreground/80">
                        Last updated
                      </p>
                      <p className="font-medium text-foreground">
                        {formatDate(teamInfo?.updated_at)}
                      </p>
                    </div>
                  </div>
                </div>

                {isTeamDetailLoading ? (
                  <div className="mt-4 flex items-center gap-2 rounded-lg border border-border/70 bg-muted/30 px-3 py-2 text-sm text-muted-foreground">
                    <Loader2 className="h-4 w-4 animate-spin" />
                    Loading team details…
                  </div>
                ) : null}
              </div>

              {(teamDetail ?? selectedTeam) ? (
                <TeamMembersManager
                  team={(teamDetail ?? selectedTeam) as TeamRecord}
                  members={members}
                  isLoadingMembers={isMembersLoading}
                  addMemberMutation={teamMutations.addMember}
                  removeMemberMutation={teamMutations.removeMember}
                />
              ) : null}
            </>
          )}
        </div>
      </div>

      <Modal
        open={isModalOpen}
        onClose={handleCloseModal}
        title={modalTitle}
        description={modalDescription}
      >
        <TeamForm
          mode={formMode}
          team={teamForForm ?? teamDetail ?? undefined}
          onClose={handleCloseModal}
          onSuccess={handleFormSuccess}
        />
      </Modal>
    </div>
  )
}
