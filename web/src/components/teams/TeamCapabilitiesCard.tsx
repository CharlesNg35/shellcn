import { useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { formatDistanceToNow } from 'date-fns'
import { AlertTriangle, Compass, Layers, RefreshCw, Share2 } from 'lucide-react'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/Card'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { Skeleton } from '@/components/ui/Skeleton'
import { cn } from '@/lib/utils/cn'
import { humanizePermissionModule } from '@/lib/utils/permissionLabels'
import { usePermissionRegistry } from '@/hooks/usePermissionRegistry'
import { useTeamCapabilities } from '@/hooks/useTeams'
import { usePermissions } from '@/hooks/usePermissions'
import { PERMISSIONS } from '@/constants/permissions'

interface TeamCapabilitiesCardProps {
  teamId: string
  canManagePermissions?: boolean
  className?: string
}

interface PermissionEntry {
  id: string
  label: string
  description?: string
  moduleLabel: string
}

interface ResourceGrantEntry {
  id: string
  resourceType: string
  resourceId: string
  displayName: string
  expiresAt?: string | null
}

export function TeamCapabilitiesCard({
  teamId,
  canManagePermissions,
  className,
}: TeamCapabilitiesCardProps) {
  const navigate = useNavigate()
  const {
    data: capabilities,
    isLoading,
    isError,
    error,
    refetch,
    isRefetching,
  } = useTeamCapabilities(teamId, {
    enabled: Boolean(teamId),
  })
  const { data: registry } = usePermissionRegistry()
  const { hasPermission } = usePermissions()
  const [showAllPermissions, setShowAllPermissions] = useState(false)

  const canShareConnections = hasPermission(PERMISSIONS.CONNECTION.SHARE)

  const permissionEntries = useMemo<PermissionEntry[]>(() => {
    if (!capabilities) {
      return []
    }
    return [...capabilities.permission_ids]
      .sort((a, b) => a.localeCompare(b))
      .map((permissionId) => {
        const definition = registry?.[permissionId]
        return {
          id: permissionId,
          label: definition?.display_name ?? permissionId,
          description: definition?.description,
          moduleLabel: humanizePermissionModule(definition?.module),
        }
      })
  }, [capabilities, registry])

  const resourceGrantEntries = useMemo<ResourceGrantEntry[]>(() => {
    if (!capabilities) {
      return []
    }
    return [...capabilities.resource_grants]
      .sort((a, b) => a.resource_id.localeCompare(b.resource_id))
      .map((grant) => {
        const definition = registry?.[grant.permission_id]
        return {
          id: `${grant.resource_type}:${grant.resource_id}:${grant.permission_id}`,
          resourceType: grant.resource_type,
          resourceId: grant.resource_id,
          displayName: definition?.display_name ?? grant.permission_id,
          expiresAt: grant.expires_at,
        }
      })
  }, [capabilities, registry])

  const totalPermissions = permissionEntries.length
  const visiblePermissionCount = showAllPermissions
    ? totalPermissions
    : Math.min(totalPermissions, 12)
  const visiblePermissions = permissionEntries.slice(0, visiblePermissionCount)
  const remainingPermissionCount = totalPermissions - visiblePermissionCount

  const handleNavigateToPermissions = () => {
    navigate('/settings/permissions')
  }

  const handleNavigateToConnections = () => {
    const params = new URLSearchParams()
    params.set('team', teamId)
    navigate(`/connections?${params.toString()}`)
  }

  const formatExpiryLabel = (expiresAt?: string | null) => {
    if (!expiresAt) {
      return 'No expiry'
    }
    const date = new Date(expiresAt)
    if (Number.isNaN(date.getTime())) {
      return 'Expiry unknown'
    }
    return `Expires ${formatDistanceToNow(date, { addSuffix: true })}`
  }

  return (
    <Card className={cn('border-border/70 shadow-sm', className)}>
      <CardHeader className="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <CardTitle className="text-lg font-semibold text-foreground">Team Capabilities</CardTitle>
          <CardDescription>
            Aggregated roles and resource grants available to this team.
          </CardDescription>
        </div>
        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => refetch()}
            disabled={isLoading || isRefetching}
            className="gap-1.5"
          >
            <RefreshCw className={cn('h-4 w-4', isRefetching ? 'animate-spin' : '')} />
            Refresh
          </Button>
          {canShareConnections ? (
            <Button
              variant="outline"
              size="sm"
              onClick={handleNavigateToConnections}
              className="gap-1.5"
            >
              <Share2 className="h-4 w-4" />
              Manage Access
            </Button>
          ) : null}
          {canManagePermissions ? (
            <Button
              variant="outline"
              size="sm"
              onClick={handleNavigateToPermissions}
              className="gap-1.5"
            >
              <Layers className="h-4 w-4" />
              Manage Roles
            </Button>
          ) : null}
        </div>
      </CardHeader>

      <CardContent className="space-y-6">
        {isLoading ? (
          <div className="space-y-4">
            <Skeleton className="h-5 w-1/3" />
            <div className="flex flex-wrap gap-2">
              {Array.from({ length: 6 }).map((_, index) => (
                <Skeleton key={index} className="h-8 w-32 rounded-full" />
              ))}
            </div>
            <Skeleton className="h-5 w-1/4" />
            <div className="space-y-3">
              {Array.from({ length: 3 }).map((_, index) => (
                <Skeleton key={index} className="h-4 w-full" />
              ))}
            </div>
          </div>
        ) : isError ? (
          <div className="flex items-start gap-3 rounded-md border border-destructive/40 bg-destructive/10 px-3 py-3 text-sm text-destructive">
            <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
            <div>
              <p className="font-medium">Failed to load capabilities</p>
              <p className="text-xs">{error?.message ?? 'Please try again later.'}</p>
            </div>
          </div>
        ) : (
          <>
            <section className="space-y-3">
              <div className="flex items-center justify-between gap-2">
                <h3 className="flex items-center gap-2 text-sm font-semibold text-foreground">
                  <Compass className="h-4 w-4 text-muted-foreground" />
                  Effective Permissions
                </h3>
                <Badge variant="outline" className="text-xs">
                  {totalPermissions} total
                </Badge>
              </div>

              {totalPermissions === 0 ? (
                <p className="rounded-md border border-dashed border-border/60 bg-muted/10 px-3 py-3 text-sm text-muted-foreground">
                  This team currently inherits no permissions. Assign roles to extend its
                  capabilities.
                </p>
              ) : (
                <div className="space-y-2">
                  <div className="flex flex-wrap gap-2">
                    {visiblePermissions.map((permission) => (
                      <div
                        key={permission.id}
                        className="flex items-start gap-2 rounded-lg border border-border/60 bg-muted/20 px-3 py-2 text-xs text-muted-foreground"
                        title={permission.description ?? permission.id}
                      >
                        <Badge variant="secondary" className="text-[10px] uppercase tracking-wide">
                          {permission.moduleLabel}
                        </Badge>
                        <div className="flex flex-col leading-tight">
                          <span className="text-sm font-medium text-foreground">
                            {permission.label}
                          </span>
                          <span className="text-[11px] text-muted-foreground">{permission.id}</span>
                        </div>
                      </div>
                    ))}
                  </div>
                  {remainingPermissionCount > 0 ? (
                    <Button
                      type="button"
                      variant="ghost"
                      size="sm"
                      onClick={() => setShowAllPermissions((value) => !value)}
                      className="text-xs text-primary"
                    >
                      {showAllPermissions
                        ? 'Show less'
                        : `Show ${remainingPermissionCount} more permission${
                            remainingPermissionCount === 1 ? '' : 's'
                          }`}
                    </Button>
                  ) : null}
                </div>
              )}
            </section>

            <section className="space-y-3">
              <div className="flex items-center gap-2">
                <h3 className="text-sm font-semibold text-foreground">Resource Grants</h3>
                <Badge variant="outline" className="text-xs">
                  {resourceGrantEntries.length}
                </Badge>
              </div>

              {resourceGrantEntries.length === 0 ? (
                <p className="rounded-md border border-dashed border-border/60 bg-muted/10 px-3 py-3 text-sm text-muted-foreground">
                  No per-resource grants detected for this team.
                </p>
              ) : (
                <div className="space-y-2">
                  {resourceGrantEntries.map((grant) => (
                    <div
                      key={grant.id}
                      className="flex flex-wrap items-center justify-between gap-2 rounded-lg border border-border/60 bg-card px-3 py-2 text-sm"
                    >
                      <div className="flex flex-col">
                        <span className="font-medium text-foreground">{grant.displayName}</span>
                        <span className="text-xs text-muted-foreground">
                          {grant.resourceType} • {grant.resourceId}
                        </span>
                      </div>
                      <div className="text-xs text-muted-foreground">
                        {formatExpiryLabel(grant.expiresAt)}
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </section>
          </>
        )}
      </CardContent>
    </Card>
  )
}
