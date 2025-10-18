import { useCallback, useEffect, useMemo, useState } from 'react'
import { formatDistanceToNow } from 'date-fns'
import { Download, RefreshCw, Trash2 } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { flexRender, getCoreRowModel, type ColumnDef, useReactTable } from '@tanstack/react-table'
import { useQueryClient } from '@tanstack/react-query'

import { PageHeader } from '@/components/layout/PageHeader'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/Tabs'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/Select'
import { Input } from '@/components/ui/Input'
import { Skeleton } from '@/components/ui/Skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/Table'
import { formatBytes } from '@/components/file-manager/utils'
import { useActiveConnections } from '@/hooks/useActiveConnections'
import {
  SESSION_RECORDINGS_QUERY_KEY,
  useDeleteSessionRecording,
  useSessionRecordings,
} from '@/hooks/useSessionRecordings'
import { usePermissions } from '@/hooks/usePermissions'
import { useTeams } from '@/hooks/useTeams'
import { PERMISSIONS } from '@/constants/permissions'
import { toast } from '@/lib/utils/toast'
import { downloadSessionRecording } from '@/lib/api/session-recordings'
import type { ActiveConnectionSession } from '@/types/connections'
import type { SessionRecordingScope, SessionRecordingSummary } from '@/types/session-recording'
import { getWorkspaceDescriptor } from '@/workspaces/protocolWorkspaceRegistry'

type TeamFilterValue = 'all' | 'personal' | 'custom' | string

interface TeamOption {
  label: string
  value: string
}

const RECORDINGS_PER_PAGE = 20

const activeScopeOptions: { label: string; value: SessionRecordingScope }[] = [
  { label: 'Team sessions', value: 'team' },
  { label: 'All sessions', value: 'all' },
]

const recordingScopeOptions: { label: string; value: SessionRecordingScope }[] = [
  { label: 'My sessions', value: 'personal' },
  { label: 'Team sessions', value: 'team' },
  { label: 'All sessions', value: 'all' },
]

const recordingSortOptions = [
  { label: 'Most recent', value: 'recent' as const },
  { label: 'Oldest', value: 'oldest' as const },
  { label: 'Largest', value: 'size_desc' as const },
  { label: 'Smallest', value: 'size_asc' as const },
]

function formatRelativeTime(timestamp?: string | null): string {
  if (!timestamp) {
    return '—'
  }
  try {
    return formatDistanceToNow(new Date(timestamp), { addSuffix: true })
  } catch {
    return '—'
  }
}

function resolveTeamParameter(filter: TeamFilterValue, customValue: string): string | undefined {
  if (filter === 'all') {
    return undefined
  }
  if (filter === 'personal') {
    return 'personal'
  }
  if (filter === 'custom') {
    return customValue.trim() || undefined
  }
  if (filter.startsWith('team:')) {
    return filter.slice('team:'.length)
  }
  return filter.trim() || undefined
}

function getDefaultScope(canViewAll: boolean, canViewTeam: boolean): SessionRecordingScope {
  if (canViewAll) {
    return 'all'
  }
  if (canViewTeam) {
    return 'team'
  }
  return 'personal'
}

export function Sessions() {
  const { hasPermission } = usePermissions()

  const canViewActiveAll =
    hasPermission(PERMISSIONS.SESSION.ACTIVE.VIEW_ALL) ||
    hasPermission(PERMISSIONS.CONNECTION.MANAGE) ||
    hasPermission(PERMISSIONS.PERMISSION.MANAGE)
  const canViewActiveTeam = canViewActiveAll || hasPermission(PERMISSIONS.SESSION.ACTIVE.VIEW_TEAM)

  const canViewRecordingAll =
    hasPermission(PERMISSIONS.SESSION.RECORDING.VIEW_ALL) ||
    hasPermission(PERMISSIONS.PERMISSION.MANAGE)
  const canViewRecordingTeam =
    canViewRecordingAll || hasPermission(PERMISSIONS.SESSION.RECORDING.VIEW_TEAM)
  const canViewRecordingPersonal = hasPermission(PERMISSIONS.SESSION.RECORDING.VIEW)
  const canDeleteRecordings = hasPermission(PERMISSIONS.SESSION.RECORDING.DELETE)

  const canListTeams =
    hasPermission(PERMISSIONS.TEAM.VIEW) || hasPermission(PERMISSIONS.TEAM.VIEW_ALL)
  const teamsQuery = useTeams({ enabled: canListTeams })

  const teamsData = teamsQuery.data?.data
  const teamOptions = useMemo<TeamOption[]>(() => {
    if (!teamsData?.length) {
      return []
    }
    return teamsData.map((team) => ({ label: team.name, value: `team:${team.id}` }))
  }, [teamsData])

  const hasActiveAccess = canViewActiveAll || canViewActiveTeam
  const hasRecordingAccess = canViewRecordingAll || canViewRecordingTeam || canViewRecordingPersonal

  const [activeTab, setActiveTab] = useState<'active' | 'recordings'>(
    hasActiveAccess ? 'active' : 'recordings'
  )

  useEffect(() => {
    if (!hasActiveAccess && activeTab === 'active' && hasRecordingAccess) {
      setActiveTab('recordings')
    } else if (!hasRecordingAccess && activeTab === 'recordings' && hasActiveAccess) {
      setActiveTab('active')
    }
  }, [activeTab, hasActiveAccess, hasRecordingAccess])

  return (
    <div className="space-y-6">
      <PageHeader
        title="Sessions"
        description="Monitor live sessions across the platform and review terminal recordings for auditing, investigation, or training."
      />

      <Tabs
        value={activeTab}
        onValueChange={(value) => setActiveTab(value as 'active' | 'recordings')}
      >
        <TabsList>
          {hasActiveAccess ? <TabsTrigger value="active">Active Sessions</TabsTrigger> : null}
          {hasRecordingAccess ? (
            <TabsTrigger value="recordings">Session Recordings</TabsTrigger>
          ) : null}
        </TabsList>

        {hasActiveAccess ? (
          <TabsContent value="active">
            <ActiveSessionsSection
              canViewAll={canViewActiveAll}
              canViewTeam={canViewActiveTeam}
              teamOptions={teamOptions}
              isTeamsLoading={teamsQuery.isLoading}
            />
          </TabsContent>
        ) : null}

        {hasRecordingAccess ? (
          <TabsContent value="recordings">
            <SessionRecordingsSection
              canViewAll={canViewRecordingAll}
              canViewTeam={canViewRecordingTeam}
              canDelete={canDeleteRecordings}
              teamOptions={teamOptions}
              isTeamsLoading={teamsQuery.isLoading}
            />
          </TabsContent>
        ) : null}
      </Tabs>
    </div>
  )
}

interface ActiveSessionsSectionProps {
  canViewAll: boolean
  canViewTeam: boolean
  teamOptions: TeamOption[]
  isTeamsLoading: boolean
}

function ActiveSessionsSection({
  canViewAll,
  canViewTeam,
  teamOptions,
  isTeamsLoading,
}: ActiveSessionsSectionProps) {
  const hasScopeAccess = canViewAll || canViewTeam
  const navigate = useNavigate()
  const [scope, setScope] = useState<SessionRecordingScope>(() => (canViewTeam ? 'team' : 'all'))
  const [teamFilter, setTeamFilter] = useState<TeamFilterValue>('all')
  const [customTeam, setCustomTeam] = useState('')

  useEffect(() => {
    if (teamFilter !== 'custom') {
      setCustomTeam('')
    }
  }, [teamFilter])

  useEffect(() => {
    if (scope === 'team' && !canViewTeam && canViewAll) {
      setScope('all')
    } else if (scope === 'all' && !canViewAll && canViewTeam) {
      setScope('team')
    }
  }, [scope, canViewAll, canViewTeam])

  useEffect(() => {
    if (scope === 'personal') {
      setScope(canViewTeam ? 'team' : 'all')
    }
  }, [scope, canViewTeam])

  const teamParam = useMemo(
    () => (scope === 'personal' ? undefined : resolveTeamParameter(teamFilter, customTeam)),
    [scope, teamFilter, customTeam]
  )

  const activeSessionsQuery = useActiveConnections({
    scope,
    team_id: teamParam,
    refetchInterval: 20_000,
    enabled: hasScopeAccess,
  })
  const activeSessions = activeSessionsQuery.data ?? []

  const columns = useMemo<ColumnDef<ActiveConnectionSession>[]>(
    () => [
      {
        accessorKey: 'connection',
        header: () => 'Connection',
        cell: ({ row }) => (
          <span className="font-medium text-foreground">
            {row.original.connection_name ?? row.original.connection_id}
          </span>
        ),
      },
      {
        accessorKey: 'protocol_id',
        header: () => 'Workspace',
        cell: ({ row }) => {
          const descriptor = getWorkspaceDescriptor(
            row.original.descriptor_id ?? row.original.protocol_id
          )
          return (
            <Badge variant="secondary" className="text-[11px] font-semibold">
              {descriptor.displayName}
            </Badge>
          )
        },
      },
      {
        accessorKey: 'owner',
        header: () => 'Owner',
        cell: ({ row }) => {
          const ownerName = row.original.owner_user_name ?? row.original.user_name
          const ownerId = row.original.owner_user_id ?? row.original.user_id
          return (
            <span className="text-sm text-muted-foreground">{ownerName ?? ownerId ?? '—'}</span>
          )
        },
      },
      {
        accessorKey: 'team_id',
        header: () => 'Team',
        cell: ({ row }) => <span>{row.original.team_id ?? '—'}</span>,
      },
      {
        accessorKey: 'started_at',
        header: () => 'Started',
        cell: ({ row }) => <span>{formatRelativeTime(row.original.started_at)}</span>,
      },
      {
        accessorKey: 'last_seen_at',
        header: () => 'Last seen',
        cell: ({ row }) => <span>{formatRelativeTime(row.original.last_seen_at)}</span>,
      },
      {
        accessorKey: 'template_version',
        header: () => 'Template',
        cell: ({ row }) => <span>{row.original.template?.version ?? '—'}</span>,
      },
      {
        id: 'actions',
        header: () => 'Actions',
        cell: ({ row }) => {
          const descriptor = getWorkspaceDescriptor(
            row.original.descriptor_id ?? row.original.protocol_id
          )
          return (
            <Button
              size="sm"
              variant="outline"
              onClick={() => navigate(descriptor.defaultRoute(row.original.id))}
            >
              Resume
            </Button>
          )
        },
        enableSorting: false,
        meta: {
          align: 'end',
        },
      },
    ],
    [navigate]
  )

  const table = useReactTable({
    data: activeSessions,
    columns,
    getCoreRowModel: getCoreRowModel(),
  })

  const scopeOptionsWithPermissions = useMemo(
    () =>
      activeScopeOptions.map((option) => ({
        ...option,
        disabled:
          (option.value === 'team' && !canViewTeam) || (option.value === 'all' && !canViewAll),
      })),
    [canViewAll, canViewTeam]
  )

  const columnCount = columns.length
  const isLoading = activeSessionsQuery.isLoading
  const showEmpty = !isLoading && activeSessions.length === 0

  if (!hasScopeAccess) {
    return null
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Active Sessions</CardTitle>
        <CardDescription>
          View currently connected users and their session details. Scope controls the visibility of
          sessions beyond your own connections.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="flex flex-wrap items-end gap-4">
          <div className="min-w-[200px] space-y-2">
            <label className="block text-sm font-medium text-muted-foreground">Scope</label>
            <Select
              value={scope}
              onValueChange={(value) => setScope(value as SessionRecordingScope)}
            >
              <SelectTrigger>
                <SelectValue placeholder="Select scope" />
              </SelectTrigger>
              <SelectContent>
                {scopeOptionsWithPermissions.map((option) => (
                  <SelectItem key={option.value} value={option.value} disabled={option.disabled}>
                    {option.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          {scope !== 'personal' ? (
            <div className="min-w-[220px] space-y-2">
              <label className="block text-sm font-medium text-muted-foreground">Team filter</label>
              <Select
                value={teamFilter}
                onValueChange={(value) => setTeamFilter(value as TeamFilterValue)}
                disabled={isTeamsLoading}
              >
                <SelectTrigger>
                  <SelectValue placeholder="All teams" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">All teams</SelectItem>
                  {teamOptions.map((team) => (
                    <SelectItem key={team.value} value={team.value}>
                      {team.label}
                    </SelectItem>
                  ))}
                  <SelectItem value="custom">Custom team ID…</SelectItem>
                </SelectContent>
              </Select>
            </div>
          ) : null}

          {scope !== 'personal' && teamFilter === 'custom' ? (
            <div className="min-w-[200px] space-y-2">
              <label className="block text-sm font-medium text-muted-foreground">Team ID</label>
              <Input
                value={customTeam}
                onChange={(event) => setCustomTeam(event.target.value)}
                placeholder="Enter team UUID"
              />
            </div>
          ) : null}

          <div className="flex-1" />
          <Button
            variant="outline"
            size="sm"
            onClick={() => activeSessionsQuery.refetch()}
            disabled={activeSessionsQuery.isLoading}
            className="flex items-center gap-1"
          >
            <RefreshCw
              className={activeSessionsQuery.isFetching ? 'h-4 w-4 animate-spin' : 'h-4 w-4'}
            />
            Refresh
          </Button>
        </div>

        <div className="overflow-hidden rounded-md border">
          <Table className="min-w-[720px] divide-y divide-border">
            <TableHeader className="bg-muted/50">
              {table.getHeaderGroups().map((headerGroup) => (
                <TableRow key={headerGroup.id}>
                  {headerGroup.headers.map((header) => (
                    <TableHead key={header.id} className="px-4 py-3 text-left font-medium">
                      {header.isPlaceholder
                        ? null
                        : flexRender(header.column.columnDef.header, header.getContext())}
                    </TableHead>
                  ))}
                </TableRow>
              ))}
            </TableHeader>
            <TableBody className="bg-background">
              {isLoading ? (
                <TableRow>
                  <TableCell colSpan={columnCount} className="px-4 py-6">
                    <div className="space-y-2">
                      <Skeleton className="h-6 w-full" />
                      <Skeleton className="h-6 w-3/4" />
                      <Skeleton className="h-6 w-1/2" />
                    </div>
                  </TableCell>
                </TableRow>
              ) : showEmpty ? (
                <TableRow>
                  <TableCell
                    colSpan={columnCount}
                    className="px-4 py-6 text-center text-sm text-muted-foreground"
                  >
                    No active sessions in the selected scope.
                  </TableCell>
                </TableRow>
              ) : (
                table.getRowModel().rows.map((row) => (
                  <TableRow key={row.id} className="divide-x divide-border/70">
                    {row.getVisibleCells().map((cell) => (
                      <TableCell key={cell.id} className="px-4 py-3">
                        {flexRender(cell.column.columnDef.cell, cell.getContext())}
                      </TableCell>
                    ))}
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </div>
      </CardContent>
    </Card>
  )
}

interface SessionRecordingsSectionProps {
  canViewAll: boolean
  canViewTeam: boolean
  canDelete: boolean
  teamOptions: TeamOption[]
  isTeamsLoading: boolean
}

function SessionRecordingsSection({
  canViewAll,
  canViewTeam,
  canDelete,
  teamOptions,
  isTeamsLoading,
}: SessionRecordingsSectionProps) {
  const queryClient = useQueryClient()
  const [scope, setScope] = useState<SessionRecordingScope>(
    getDefaultScope(canViewAll, canViewTeam)
  )
  const [teamFilter, setTeamFilter] = useState<TeamFilterValue>('all')
  const [customTeam, setCustomTeam] = useState('')
  const [sort, setSort] = useState<'recent' | 'oldest' | 'size_desc' | 'size_asc'>('recent')
  const [page, setPage] = useState(1)

  useEffect(() => {
    if (scope === 'personal') {
      setTeamFilter('all')
    }
  }, [scope])

  useEffect(() => {
    if (teamFilter !== 'custom') {
      setCustomTeam('')
    }
  }, [teamFilter])

  useEffect(() => {
    setPage(1)
  }, [scope, teamFilter, customTeam, sort])

  const teamParam = useMemo(
    () => (scope === 'personal' ? undefined : resolveTeamParameter(teamFilter, customTeam)),
    [scope, teamFilter, customTeam]
  )

  const recordingsQuery = useSessionRecordings(
    {
      scope,
      team_id: teamParam,
      sort: sort === 'recent' ? undefined : sort,
      page,
      per_page: RECORDINGS_PER_PAGE,
    },
    {
      placeholderData: (previous) => previous,
    }
  )

  const deleteRecording = useDeleteSessionRecording({
    onSuccess: () => {
      toast.success('Recording deleted')
      queryClient.invalidateQueries({ queryKey: SESSION_RECORDINGS_QUERY_KEY })
    },
    onError: (error) => {
      toast.error('Failed to delete recording', {
        description: error.message,
      })
    },
  })

  const recordings = recordingsQuery.data?.data ?? []
  const recordingsMeta = recordingsQuery.data?.meta

  const totalPages = recordingsMeta?.total
    ? Math.max(1, Math.ceil(recordingsMeta.total / RECORDINGS_PER_PAGE))
    : 1
  const totalsCount = recordingsMeta?.total ?? recordings.length
  const showingFrom = totalsCount === 0 ? 0 : (page - 1) * RECORDINGS_PER_PAGE + 1
  const showingTo = Math.min(page * RECORDINGS_PER_PAGE, totalsCount)

  const handleDownload = useCallback(async (record: SessionRecordingSummary) => {
    try {
      const blob = await downloadSessionRecording(record.record_id)
      const url = URL.createObjectURL(blob)
      const anchor = document.createElement('a')
      anchor.href = url
      anchor.download = `${record.session_id}-${record.record_id}.cast.gz`
      anchor.click()
      setTimeout(() => URL.revokeObjectURL(url), 1000)
      toast.success('Recording download started')
    } catch (error) {
      toast.error('Unable to download recording', {
        description: error instanceof Error ? error.message : undefined,
      })
    }
  }, [])

  const handleDelete = useCallback(
    async (record: SessionRecordingSummary) => {
      await deleteRecording.mutateAsync(record.record_id)
    },
    [deleteRecording]
  )

  const columns = useMemo<ColumnDef<SessionRecordingSummary>[]>(
    () => [
      {
        accessorKey: 'connection',
        header: () => 'Connection',
        cell: ({ row }) => (
          <span className="font-medium text-foreground">
            {row.original.connection_name ?? row.original.connection_id}
          </span>
        ),
      },
      {
        accessorKey: 'owner',
        header: () => 'Owner',
        cell: ({ row }) => (
          <span className="text-sm text-muted-foreground">
            {row.original.owner_user_name ?? row.original.owner_user_id}
          </span>
        ),
      },
      {
        accessorKey: 'created_by',
        header: () => 'Captured by',
        cell: ({ row }) => (
          <span className="text-sm text-muted-foreground">
            {row.original.created_by_user_name ?? row.original.created_by_user_id}
          </span>
        ),
      },
      {
        accessorKey: 'team_id',
        header: () => 'Team',
        cell: ({ row }) => <span>{row.original.team_id ?? '—'}</span>,
      },
      {
        accessorKey: 'size_bytes',
        header: () => 'Size',
        cell: ({ row }) => <span>{formatBytes(row.original.size_bytes)}</span>,
      },
      {
        accessorKey: 'created_at',
        header: () => 'Created',
        cell: ({ row }) => <span>{formatRelativeTime(row.original.created_at)}</span>,
      },
      {
        accessorKey: 'retention_until',
        header: () => 'Retention',
        cell: ({ row }) => <span>{formatRelativeTime(row.original.retention_until)}</span>,
      },
      {
        id: 'actions',
        header: () => <span className="sr-only">Actions</span>,
        cell: ({ row }) => {
          const isDeleting =
            deleteRecording.isPending && deleteRecording.variables === row.original.record_id
          return (
            <div className="flex justify-end gap-2">
              <Button
                variant="ghost"
                size="icon"
                onClick={() => handleDownload(row.original)}
                title="Download recording"
              >
                <Download className="h-4 w-4" />
              </Button>
              {canDelete ? (
                <Button
                  variant="ghost"
                  size="icon"
                  onClick={() => handleDelete(row.original)}
                  disabled={isDeleting}
                  title="Delete recording"
                >
                  {isDeleting ? (
                    <RefreshCw className="h-4 w-4 animate-spin" />
                  ) : (
                    <Trash2 className="h-4 w-4" />
                  )}
                </Button>
              ) : null}
            </div>
          )
        },
        size: 90,
      },
    ],
    [canDelete, deleteRecording.isPending, deleteRecording.variables, handleDelete, handleDownload]
  )

  const table = useReactTable({
    data: recordings,
    columns,
    getCoreRowModel: getCoreRowModel(),
  })

  const scopeOptionsWithPermissions = useMemo(
    () =>
      recordingScopeOptions.map((option) => ({
        ...option,
        disabled:
          (option.value === 'team' && !canViewTeam) || (option.value === 'all' && !canViewAll),
      })),
    [canViewAll, canViewTeam]
  )

  const columnCount = columns.length
  const isLoading = recordingsQuery.isLoading
  const showEmpty = !isLoading && recordings.length === 0

  return (
    <Card>
      <CardHeader>
        <CardTitle>Session Recordings</CardTitle>
        <CardDescription>
          Browse archived terminal recordings. Filter by scope or team, download archives, and
          delete obsolete captures when permitted.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="flex flex-wrap items-end gap-4">
          <div className="min-w-[200px] space-y-2">
            <label className="block text-sm font-medium text-muted-foreground">Scope</label>
            <Select
              value={scope}
              onValueChange={(value) => setScope(value as SessionRecordingScope)}
            >
              <SelectTrigger>
                <SelectValue placeholder="Select scope" />
              </SelectTrigger>
              <SelectContent>
                {scopeOptionsWithPermissions.map((option) => (
                  <SelectItem key={option.value} value={option.value} disabled={option.disabled}>
                    {option.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          {scope !== 'personal' ? (
            <div className="min-w-[220px] space-y-2">
              <label className="block text-sm font-medium text-muted-foreground">Team filter</label>
              <Select
                value={teamFilter}
                onValueChange={(value) => setTeamFilter(value as TeamFilterValue)}
                disabled={isTeamsLoading}
              >
                <SelectTrigger>
                  <SelectValue placeholder="All teams" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">All teams</SelectItem>
                  <SelectItem value="personal">Only personal sessions</SelectItem>
                  {teamOptions.map((team) => (
                    <SelectItem key={team.value} value={team.value}>
                      {team.label}
                    </SelectItem>
                  ))}
                  <SelectItem value="custom">Custom team ID…</SelectItem>
                </SelectContent>
              </Select>
            </div>
          ) : null}

          {scope !== 'personal' && teamFilter === 'custom' ? (
            <div className="min-w-[200px] space-y-2">
              <label className="block text-sm font-medium text-muted-foreground">Team ID</label>
              <Input
                value={customTeam}
                onChange={(event) => setCustomTeam(event.target.value)}
                placeholder="Enter team UUID"
              />
            </div>
          ) : null}

          <div className="min-w-[200px] space-y-2">
            <label className="block text-sm font-medium text-muted-foreground">Sort by</label>
            <Select value={sort} onValueChange={(value) => setSort(value as typeof sort)}>
              <SelectTrigger>
                <SelectValue placeholder="Sort order" />
              </SelectTrigger>
              <SelectContent>
                {recordingSortOptions.map((option) => (
                  <SelectItem key={option.value} value={option.value}>
                    {option.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="flex-1" />
        </div>

        <div className="overflow-hidden rounded-md border">
          <Table className="min-w-[960px] divide-y divide-border">
            <TableHeader className="bg-muted/50">
              {table.getHeaderGroups().map((headerGroup) => (
                <TableRow key={headerGroup.id}>
                  {headerGroup.headers.map((header) => (
                    <TableHead
                      key={header.id}
                      className={`px-4 py-3 text-left font-medium ${
                        header.column.id === 'actions' ? 'text-right' : ''
                      }`}
                    >
                      {header.isPlaceholder
                        ? null
                        : flexRender(header.column.columnDef.header, header.getContext())}
                    </TableHead>
                  ))}
                </TableRow>
              ))}
            </TableHeader>
            <TableBody className="bg-background">
              {isLoading ? (
                <TableRow>
                  <TableCell colSpan={columnCount} className="px-4 py-6">
                    <div className="space-y-2">
                      <Skeleton className="h-6 w-full" />
                      <Skeleton className="h-6 w-4/5" />
                      <Skeleton className="h-6 w-3/5" />
                    </div>
                  </TableCell>
                </TableRow>
              ) : showEmpty ? (
                <TableRow>
                  <TableCell
                    colSpan={columnCount}
                    className="px-4 py-6 text-center text-sm text-muted-foreground"
                  >
                    No recordings found for the selected filters.
                  </TableCell>
                </TableRow>
              ) : (
                table.getRowModel().rows.map((row) => (
                  <TableRow key={row.id} className="divide-x divide-border/70">
                    {row.getVisibleCells().map((cell) => (
                      <TableCell
                        key={cell.id}
                        className={`px-4 py-3 ${cell.column.id === 'actions' ? 'text-right' : ''}`}
                      >
                        {flexRender(cell.column.columnDef.cell, cell.getContext())}
                      </TableCell>
                    ))}
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </div>

        <div className="flex flex-wrap items-center justify-between gap-2 text-sm text-muted-foreground">
          <span>
            Showing {totalsCount === 0 ? 0 : showingFrom} – {showingTo} of {totalsCount}
          </span>
          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={() => setPage((current) => Math.max(1, current - 1))}
              disabled={page <= 1}
            >
              Previous
            </Button>
            <span>
              Page {page} of {totalPages}
            </span>
            <Button
              variant="outline"
              size="sm"
              onClick={() => setPage((current) => Math.min(totalPages, current + 1))}
              disabled={page >= totalPages}
            >
              Next
            </Button>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}

export default Sessions
