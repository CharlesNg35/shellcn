import { useCallback, useMemo } from 'react'
import {
  useMutation,
  useQuery,
  useQueryClient,
  type UseQueryOptions,
  type UseQueryResult,
} from '@tanstack/react-query'
import type { ApiError } from '@/lib/api/http'
import { toast } from '@/lib/utils/toast'
import {
  addTeamMember,
  createTeam,
  deleteTeam,
  fetchTeamById,
  fetchTeamMembers,
  fetchTeams,
  removeTeamMember,
  updateTeam,
} from '@/lib/api/teams'
import type {
  TeamCreatePayload,
  TeamListResult,
  TeamMember,
  TeamRecord,
  TeamUpdatePayload,
} from '@/types/teams'

export const TEAMS_LIST_QUERY_KEY = ['teams', 'list'] as const

export function getTeamsQueryKey() {
  return TEAMS_LIST_QUERY_KEY
}

export const TEAM_DETAIL_QUERY_KEY = ['teams', 'detail'] as const

export function getTeamDetailQueryKey(teamId?: string) {
  return [...TEAM_DETAIL_QUERY_KEY, teamId ?? ''] as const
}

export const TEAM_MEMBERS_QUERY_KEY = ['teams', 'members'] as const

export function getTeamMembersQueryKey(teamId?: string) {
  return [...TEAM_MEMBERS_QUERY_KEY, teamId ?? ''] as const
}

type TeamsQueryOptions = Omit<
  UseQueryOptions<TeamListResult, ApiError, TeamListResult, readonly unknown[]>,
  'queryKey' | 'queryFn'
>

type TeamDetailQueryOptions = Omit<
  UseQueryOptions<TeamRecord, ApiError, TeamRecord, readonly unknown[]>,
  'queryKey' | 'queryFn'
>

type TeamMembersQueryOptions = Omit<
  UseQueryOptions<TeamMember[], ApiError, TeamMember[], readonly unknown[]>,
  'queryKey' | 'queryFn'
>

export function useTeams(options?: TeamsQueryOptions): UseQueryResult<TeamListResult, ApiError> {
  const queryKey = useMemo(() => getTeamsQueryKey(), [])

  return useQuery<TeamListResult, ApiError, TeamListResult, readonly unknown[]>({
    queryKey,
    queryFn: fetchTeams,
    placeholderData: (previous) => previous ?? undefined,
    staleTime: 60_000,
    ...(options ?? {}),
  })
}

export function useTeam(
  teamId: string | undefined,
  options?: TeamDetailQueryOptions
): UseQueryResult<TeamRecord, ApiError> {
  const queryKey = useMemo(() => getTeamDetailQueryKey(teamId), [teamId])

  return useQuery<TeamRecord, ApiError, TeamRecord, readonly unknown[]>({
    queryKey,
    queryFn: () => fetchTeamById(teamId as string),
    enabled: Boolean(teamId),
    staleTime: 60_000,
    ...(options ?? {}),
  })
}

export function useTeamMembers(
  teamId: string | undefined,
  options?: TeamMembersQueryOptions
): UseQueryResult<TeamMember[], ApiError> {
  const queryKey = useMemo(() => getTeamMembersQueryKey(teamId), [teamId])

  return useQuery<TeamMember[], ApiError, TeamMember[], readonly unknown[]>({
    queryKey,
    queryFn: () => fetchTeamMembers(teamId as string),
    enabled: Boolean(teamId),
    staleTime: 30_000,
    ...(options ?? {}),
  })
}

export function useTeamMutations() {
  const queryClient = useQueryClient()

  const invalidateTeams = useCallback(async () => {
    await queryClient.invalidateQueries({ queryKey: TEAMS_LIST_QUERY_KEY })
  }, [queryClient])

  const invalidateTeam = useCallback(
    async (teamId?: string) => {
      if (!teamId) {
        return
      }
      await queryClient.invalidateQueries({ queryKey: getTeamDetailQueryKey(teamId) })
      await queryClient.invalidateQueries({ queryKey: getTeamMembersQueryKey(teamId) })
    },
    [queryClient]
  )

  const create = useMutation({
    mutationFn: (payload: TeamCreatePayload) => createTeam(payload),
    onSuccess: async (team) => {
      await invalidateTeams()
      toast.success('Team created', {
        description: `${team.name} is now available for member assignment`,
      })
    },
    onError: (error: ApiError) => {
      toast.error('Failed to create team', {
        description: error.message || 'Please try again later',
      })
    },
  })

  const update = useMutation({
    mutationFn: ({ teamId, payload }: { teamId: string; payload: TeamUpdatePayload }) =>
      updateTeam(teamId, payload),
    onSuccess: async (team) => {
      await invalidateTeams()
      await invalidateTeam(team.id)
      toast.success('Team updated', {
        description: `${team.name} saved successfully`,
      })
    },
    onError: (error: ApiError) => {
      toast.error('Failed to update team', {
        description: error.message || 'Please try again later',
      })
    },
  })

  const remove = useMutation({
    mutationFn: (teamId: string) => deleteTeam(teamId),
    onSuccess: async (_, teamId) => {
      await invalidateTeams()
      await invalidateTeam(teamId)
      toast.success('Team deleted')
    },
    onError: (error: ApiError) => {
      toast.error('Failed to delete team', {
        description: error.message || 'Please try again later',
      })
    },
  })

  const addMemberMutation = useMutation({
    mutationFn: ({ teamId, userId }: { teamId: string; userId: string }) =>
      addTeamMember(teamId, userId),
    onSuccess: async (_, variables) => {
      await invalidateTeam(variables.teamId)
      toast.success('Member added to team')
    },
    onError: (error: ApiError) => {
      toast.error('Failed to add member', {
        description: error.message || 'Please try again later',
      })
    },
  })

  const removeMemberMutation = useMutation({
    mutationFn: ({ teamId, userId }: { teamId: string; userId: string }) =>
      removeTeamMember(teamId, userId),
    onSuccess: async (_, variables) => {
      await invalidateTeam(variables.teamId)
      toast.success('Member removed from team')
    },
    onError: (error: ApiError) => {
      toast.error('Failed to remove member', {
        description: error.message || 'Please try again later',
      })
    },
  })

  return {
    create,
    update,
    remove,
    addMember: addMemberMutation,
    removeMember: removeMemberMutation,
  }
}
