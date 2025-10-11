import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { ArrowLeft, CalendarClock, Loader2, PencilLine, Trash2, Users } from 'lucide-react'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/Card'
import { PermissionGuard } from '@/components/permissions/PermissionGuard'
import { PageHeader } from '@/components/layout/PageHeader'
import { TeamMembersManager } from '@/components/teams/TeamMembersManager'
import { TeamRolesManager } from '@/components/teams/TeamRolesManager'
import { TeamCapabilitiesCard } from '@/components/teams/TeamCapabilitiesCard'
import { useTeam, useTeamMembers, useTeamMutations } from '@/hooks/useTeams'
import { useRoles } from '@/hooks/useRoles'
import { usePermissions } from '@/hooks/usePermissions'
import { useBreadcrumb } from '@/contexts/BreadcrumbContext'
import { PERMISSIONS } from '@/constants/permissions'
import { Modal } from '@/components/ui/Modal'
import { TeamForm } from '@/components/teams/TeamForm'
import { cn } from '@/lib/utils/cn'

function formatDate(value?: string) {
  if (!value) {
    return '—'
  }
  try {
    const date = new Date(value)
    return date.toLocaleDateString('en-US', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    })
  } catch {
    return value
  }
}

export function TeamDetail() {
  const { teamId } = useParams<{ teamId: string }>()
  const navigate = useNavigate()
  const teamMutations = useTeamMutations()
  const { setOverride, clearOverride } = useBreadcrumb()
  const { data: allRoles, isLoading: isRolesLoading } = useRoles()
  const { hasPermission } = usePermissions()
  const canManageRoles = hasPermission(PERMISSIONS.PERMISSION.MANAGE)
  const [isEditModalOpen, setEditModalOpen] = useState(false)

  const { data: teamDetail, isLoading: isTeamDetailLoading } = useTeam(teamId ?? '', {
    enabled: Boolean(teamId),
  })

  const { data: members, isLoading: isMembersLoading } = useTeamMembers(teamId ?? '', {
    enabled: Boolean(teamId),
  })

  // Set breadcrumb override for this team
  useEffect(() => {
    if (!teamId) {
      return
    }
    const path = `/settings/teams/${teamId}`
    if (teamDetail?.name) {
      setOverride(path, teamDetail.name)
      return () => {
        clearOverride(path)
      }
    }
    return () => {
      clearOverride(path)
    }
  }, [teamDetail?.name, teamId, setOverride, clearOverride])

  const handleBack = () => {
    navigate('/settings/teams')
  }

  const handleEdit = () => {
    if (teamDetail?.source && teamDetail.source.toLowerCase() !== 'local') {
      return
    }
    setEditModalOpen(true)
  }

  const handleDelete = async () => {
    if (!teamDetail) {
      return
    }

    const confirmed = window.confirm(
      `Delete team "${teamDetail.name}"? This will remove team membership assignments.`
    )
    if (!confirmed) {
      return
    }

    try {
      await teamMutations.remove.mutateAsync(teamDetail.id)
      navigate('/settings/teams')
    } catch (error) {
      console.error(error)
    }
  }

  if (!teamId) {
    return (
      <div className="space-y-6">
        <PageHeader
          title="Team Not Found"
          description="The requested team could not be found."
          action={
            <Button onClick={handleBack} variant="outline">
              <ArrowLeft className="mr-2 h-4 w-4" />
              Back to Teams
            </Button>
          }
        />
      </div>
    )
  }

  if (isTeamDetailLoading) {
    return (
      <div className="space-y-6">
        <PageHeader
          title="Loading..."
          description="Please wait while we load the team details."
          action={
            <Button onClick={handleBack} variant="outline">
              <ArrowLeft className="mr-2 h-4 w-4" />
              Back to Teams
            </Button>
          }
        />
        <Card>
          <CardContent className="flex min-h-[300px] items-center justify-center p-8">
            <div className="flex items-center gap-2 text-muted-foreground">
              <Loader2 className="h-5 w-5 animate-spin" />
              Loading team details...
            </div>
          </CardContent>
        </Card>
      </div>
    )
  }

  if (!teamDetail) {
    return (
      <div className="space-y-6">
        <PageHeader
          title="Team Not Found"
          description="The requested team could not be found."
          action={
            <Button onClick={handleBack} variant="outline">
              <ArrowLeft className="mr-2 h-4 w-4" />
              Back to Teams
            </Button>
          }
        />
      </div>
    )
  }

  const memberCount = members?.length ?? teamDetail.members?.length ?? 0
  const assignedRoles = teamDetail.roles ?? []
  const teamSource = teamDetail.source?.toUpperCase() ?? 'LOCAL'
  const isExternalTeam = Boolean(teamDetail.source && teamDetail.source.toLowerCase() !== 'local')
  const canEditTeam = !isExternalTeam

  return (
    <div className="space-y-6">
      <PageHeader
        title={teamDetail.name}
        description={
          teamDetail.description?.trim()?.length
            ? teamDetail.description
            : 'No description provided for this team.'
        }
        action={
          <div className="flex flex-wrap gap-2">
            <Button onClick={handleBack} variant="outline">
              <ArrowLeft className="mr-2 h-4 w-4" />
              Back to Teams
            </Button>
            <PermissionGuard permission={PERMISSIONS.TEAM.MANAGE}>
              {canEditTeam ? (
                <Button type="button" variant="outline" onClick={handleEdit}>
                  <PencilLine className="mr-2 h-4 w-4" />
                  Edit
                </Button>
              ) : null}
              <Button
                type="button"
                variant="outline"
                className="text-destructive hover:bg-destructive/10 hover:text-destructive"
                onClick={handleDelete}
                disabled={teamMutations.remove.isPending}
              >
                {teamMutations.remove.isPending ? (
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                ) : (
                  <Trash2 className="mr-2 h-4 w-4" />
                )}
                Delete
              </Button>
            </PermissionGuard>
          </div>
        }
      />

      <div className="grid gap-6 lg:grid-cols-3">
        {/* Team Info Card */}
        <Card className="lg:col-span-1">
          <CardHeader>
            <CardTitle>Team Information</CardTitle>
            <CardDescription>Basic details about this team</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="flex items-center gap-2 flex-wrap">
              {teamDetail.members?.some((member) => member.is_root) && (
                <Badge variant="destructive" className="text-xs">
                  Root member
                </Badge>
              )}
              {typeof memberCount === 'number' && (
                <Badge variant="secondary" className="text-xs">
                  <Users className="mr-1 h-3 w-3" />
                  {memberCount} {memberCount === 1 ? 'member' : 'members'}
                </Badge>
              )}
              <Badge variant="outline" className="text-xs">
                {assignedRoles.length} role{assignedRoles.length === 1 ? '' : 's'} assigned
              </Badge>
              <Badge
                variant={isExternalTeam ? 'secondary' : 'outline'}
                className={cn('text-xs', isExternalTeam ? '' : 'text-foreground')}
              >
                {teamSource}
              </Badge>
            </div>

            <div className="space-y-3 pt-2">
              {isExternalTeam ? (
                <div className="rounded-md border border-border/60 bg-muted/30 p-3 text-xs text-muted-foreground">
                  This team is synchronized from an external provider. Team metadata is read-only, but
                  you can still manage memberships, roles, and delete the team if required.
                </div>
              ) : null}
              <div className="flex items-start gap-3">
                <div className="flex h-9 w-9 items-center justify-center rounded-md bg-muted">
                  <CalendarClock className="h-4 w-4 text-muted-foreground" />
                </div>
                <div className="flex-1">
                  <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                    Created
                  </p>
                  <p className="mt-1 text-sm font-medium text-foreground">
                    {formatDate(teamDetail.created_at)}
                  </p>
                </div>
              </div>
              <div className="flex items-start gap-3">
                <div className="flex h-9 w-9 items-center justify-center rounded-md bg-muted">
                  <CalendarClock className="h-4 w-4 text-muted-foreground" />
                </div>
                <div className="flex-1">
                  <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                    Last updated
                  </p>
                  <p className="mt-1 text-sm font-medium text-foreground">
                    {formatDate(teamDetail.updated_at)}
                  </p>
                </div>
              </div>
            </div>
          </CardContent>
        </Card>

        <TeamCapabilitiesCard
          teamId={teamDetail.id}
          canManagePermissions={canManageRoles}
          className="lg:col-span-2"
        />

        {/* Team Roles & Members */}
        <div className="space-y-6 lg:col-span-2">
          <TeamRolesManager
            teamId={teamDetail.id}
            teamName={teamDetail.name}
            assignedRoles={assignedRoles}
            availableRoles={allRoles}
            isLoadingRoles={isRolesLoading}
            setRolesMutation={teamMutations.setRoles}
            canManageRoles={canManageRoles}
          />
          <TeamMembersManager
            team={teamDetail}
            members={members}
            isLoadingMembers={isMembersLoading}
            addMemberMutation={teamMutations.addMember}
            removeMemberMutation={teamMutations.removeMember}
            teamRoles={assignedRoles}
          />
        </div>
      </div>

      {canEditTeam ? (
        <Modal
          open={isEditModalOpen}
          onClose={() => setEditModalOpen(false)}
          title={`Edit ${teamDetail.name}`}
          description="Update the team name or description. Changes take effect immediately."
        >
          <TeamForm
            mode="edit"
            team={teamDetail}
            onClose={() => setEditModalOpen(false)}
            onSuccess={() => setEditModalOpen(false)}
          />
        </Modal>
      ) : null}
    </div>
  )
}
