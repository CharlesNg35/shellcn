import type { ApiResponse } from '@/types/api'
import { apiClient } from './client'
import { unwrapResponse } from './http'
import type {
  TeamCreatePayload,
  TeamListResult,
  TeamMember,
  TeamRecord,
  TeamUpdatePayload,
} from '@/types/teams'

const TEAMS_ENDPOINT = '/teams'

interface TeamMemberResponse {
  id: string
  username: string
  email: string
  first_name?: string
  last_name?: string
  avatar?: string
  is_active: boolean
  is_root?: boolean
  last_login_at?: string | null
  roles?: Array<{
    id: string
    name: string
    description?: string
  }>
}

interface TeamResponse {
  id: string
  name: string
  description?: string
  created_at?: string
  updated_at?: string
  users?: TeamMemberResponse[]
}

function transformTeamMember(raw: TeamMemberResponse): TeamMember {
  return {
    id: raw.id,
    username: raw.username,
    email: raw.email,
    first_name: raw.first_name,
    last_name: raw.last_name,
    avatar: raw.avatar,
    is_active: raw.is_active,
    is_root: raw.is_root,
    last_login_at: raw.last_login_at,
    roles: raw.roles ?? [],
  }
}

function transformTeam(raw: TeamResponse): TeamRecord {
  return {
    id: raw.id,
    name: raw.name,
    description: raw.description || undefined,
    created_at: raw.created_at,
    updated_at: raw.updated_at,
    members: raw.users ? raw.users.map(transformTeamMember) : undefined,
  }
}

export async function fetchTeams(): Promise<TeamListResult> {
  const response = await apiClient.get<ApiResponse<TeamResponse[]>>(TEAMS_ENDPOINT)
  const data = unwrapResponse(response)
  return {
    data: data.map(transformTeam),
  }
}

export async function fetchTeamById(teamId: string): Promise<TeamRecord> {
  const response = await apiClient.get<ApiResponse<TeamResponse>>(`${TEAMS_ENDPOINT}/${teamId}`)
  const data = unwrapResponse(response)
  return transformTeam(data)
}

export async function fetchTeamMembers(teamId: string): Promise<TeamMember[]> {
  const response = await apiClient.get<ApiResponse<TeamMemberResponse[]>>(
    `${TEAMS_ENDPOINT}/${teamId}/members`
  )
  const data = unwrapResponse(response)
  return data.map(transformTeamMember)
}

export async function createTeam(payload: TeamCreatePayload): Promise<TeamRecord> {
  const response = await apiClient.post<ApiResponse<TeamResponse>>(TEAMS_ENDPOINT, payload)
  const data = unwrapResponse(response)
  return transformTeam(data)
}

export async function updateTeam(teamId: string, payload: TeamUpdatePayload): Promise<TeamRecord> {
  const response = await apiClient.patch<ApiResponse<TeamResponse>>(
    `${TEAMS_ENDPOINT}/${teamId}`,
    payload
  )
  const data = unwrapResponse(response)
  return transformTeam(data)
}

export async function deleteTeam(teamId: string): Promise<boolean> {
  const response = await apiClient.delete<ApiResponse<{ deleted: boolean }>>(
    `${TEAMS_ENDPOINT}/${teamId}`
  )
  const data = unwrapResponse(response)
  return Boolean(data?.deleted)
}

export async function addTeamMember(teamId: string, userId: string): Promise<boolean> {
  const response = await apiClient.post<ApiResponse<{ added: boolean }>>(
    `${TEAMS_ENDPOINT}/${teamId}/members`,
    {
      user_id: userId,
    }
  )
  const data = unwrapResponse(response)
  return Boolean(data?.added)
}

export async function removeTeamMember(teamId: string, userId: string): Promise<boolean> {
  const response = await apiClient.delete<ApiResponse<{ removed: boolean }>>(
    `${TEAMS_ENDPOINT}/${teamId}/members/${userId}`
  )
  const data = unwrapResponse(response)
  return Boolean(data?.removed)
}

export const teamsApi = {
  list: fetchTeams,
  get: fetchTeamById,
  members: fetchTeamMembers,
  create: createTeam,
  update: updateTeam,
  delete: deleteTeam,
  addMember: addTeamMember,
  removeMember: removeTeamMember,
}
