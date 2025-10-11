import { useEffect, useMemo, useState } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import { Plus } from 'lucide-react'
import { Button } from '@/components/ui/Button'
import { Modal } from '@/components/ui/Modal'
import { PermissionGuard } from '@/components/permissions/PermissionGuard'
import { PageHeader } from '@/components/layout/PageHeader'
import { TeamTable } from '@/components/teams/TeamTable'
import { TeamForm, type TeamFormMode } from '@/components/teams/TeamForm'
import { useTeamMutations, useTeams } from '@/hooks/useTeams'
import type { TeamRecord } from '@/types/teams'
import { PERMISSIONS } from '@/constants/permissions'
import { toast } from '@/lib/utils/toast'

export function Teams() {
  const location = useLocation()
  const navigate = useNavigate()
  const [formMode, setFormMode] = useState<TeamFormMode>('create')
  const [teamForForm, setTeamForForm] = useState<TeamRecord | undefined>()
  const [isModalOpen, setIsModalOpen] = useState(false)
  const [memberCounts, setMemberCounts] = useState<Record<string, number>>({})

  const { data: teamsResult, isLoading: isTeamsLoading } = useTeams()
  const teams = useMemo(() => teamsResult?.data ?? [], [teamsResult?.data])

  const teamMutations = useTeamMutations()

  useEffect(() => {
    // Initialize member counts from teams data
    if (teams.length > 0) {
      const counts: Record<string, number> = {}
      teams.forEach((team) => {
        if (team.members) {
          counts[team.id] = team.members.length
        }
      })
      setMemberCounts((prev) => ({ ...prev, ...counts }))
    }
  }, [teams])

  const handleOpenCreateModal = () => {
    setFormMode('create')
    setTeamForForm(undefined)
    setIsModalOpen(true)
  }

  const handleOpenEditModal = (team: TeamRecord) => {
    if (isExternalTeam(team)) {
      toast.warning('This team is managed by an external provider and cannot be edited.')
      return
    }
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
    } catch (error) {
      console.error(error)
    }
  }

  const handleSelectTeam = (teamId: string) => {
    navigate(`/settings/teams/${teamId}`)
  }

  const handleManageTeamResources = (team: TeamRecord) => {
    const search = new URLSearchParams()
    search.set('team', team.id)
    navigate(`/connections?${search.toString()}`)
  }

  const modalTitle = formMode === 'create' ? 'Create team' : 'Edit team'
  const modalDescription =
    formMode === 'create'
      ? 'Define a new team to group users and manage permissions collectively.'
      : 'Update the team name or description. Changes apply immediately.'

  return (
    <div key={location.pathname} className="space-y-6">
      <PageHeader
        title="Teams"
        description="Organize users into teams to streamline permission assignment and access control. Teams can be nested by using slash-separated names (for example, Security/Incident Response)."
        action={
          <PermissionGuard permission={PERMISSIONS.TEAM.MANAGE}>
            <Button onClick={handleOpenCreateModal}>
              <Plus className="mr-2 h-4 w-4" />
              Create Team
            </Button>
          </PermissionGuard>
        }
      />

      <TeamTable
        teams={teams}
        isLoading={isTeamsLoading}
        memberCounts={memberCounts}
        onSelectTeam={handleSelectTeam}
        onEditTeam={handleOpenEditModal}
        onDeleteTeam={handleDeleteTeam}
        onManageResources={handleManageTeamResources}
        emptyAction={
          <PermissionGuard permission={PERMISSIONS.TEAM.MANAGE}>
            <Button onClick={handleOpenCreateModal}>
              <Plus className="mr-2 h-4 w-4" />
              Create Team
            </Button>
          </PermissionGuard>
        }
      />

      <Modal
        open={isModalOpen}
        onClose={handleCloseModal}
        title={modalTitle}
        description={modalDescription}
      >
        <TeamForm
          mode={formMode}
          team={teamForForm}
          onClose={handleCloseModal}
          onSuccess={handleFormSuccess}
        />
      </Modal>
    </div>
  )
}
const isExternalTeam = (team?: TeamRecord) =>
  Boolean(team?.source && team.source.toLowerCase() !== 'local')
