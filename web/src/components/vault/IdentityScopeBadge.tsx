import { Badge } from '@/components/ui/Badge'
import type { IdentityScope } from '@/types/vault'

interface IdentityScopeBadgeProps {
  scope: IdentityScope
}

const LABELS: Record<IdentityScope, string> = {
  global: 'Global',
  team: 'Team',
  connection: 'Connection',
}

const VARIANTS: Record<IdentityScope, 'outline' | 'secondary' | 'default'> = {
  global: 'outline',
  team: 'secondary',
  connection: 'default',
}

export function IdentityScopeBadge({ scope }: IdentityScopeBadgeProps) {
  return (
    <Badge variant={VARIANTS[scope]} className="text-xs uppercase tracking-wide">
      {LABELS[scope]}
    </Badge>
  )
}
