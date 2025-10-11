import { useMemo, useState } from 'react'
import { Search, Loader2 } from 'lucide-react'
import { Modal } from '@/components/ui/Modal'
import { Input } from '@/components/ui/Input'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import type { Protocol } from '@/types/protocols'
import { resolveProtocolIcon } from '@/lib/utils/protocolIcons'
import { cn } from '@/lib/utils/cn'

interface ResourceSelectionModalProps {
  open: boolean
  onClose: () => void
  protocols: Protocol[]
  isLoading?: boolean
  onSelectProtocol: (protocolId: string) => void
}

interface ProtocolGroup {
  category: string
  items: Protocol[]
}

export function ResourceSelectionModal({
  open,
  onClose,
  protocols,
  isLoading,
  onSelectProtocol,
}: ResourceSelectionModalProps) {
  const [searchTerm, setSearchTerm] = useState('')

  const filteredProtocols = useMemo(() => {
    const term = searchTerm.trim().toLowerCase()

    return protocols
      .filter((protocol) => protocol.available)
      .filter((protocol) => {
        if (!term) {
          return true
        }
        return (
          protocol.name.toLowerCase().includes(term) ||
          protocol.description?.toLowerCase().includes(term) ||
          protocol.category.toLowerCase().includes(term)
        )
      })
      .sort((a, b) => a.name.localeCompare(b.name))
  }, [protocols, searchTerm])

  const grouped = useMemo<ProtocolGroup[]>(() => {
    const map = new Map<string, Protocol[]>()
    for (const protocol of filteredProtocols) {
      const key = protocol.category ? protocol.category.toLowerCase() : 'other'
      if (!map.has(key)) {
        map.set(key, [])
      }
      map.get(key)?.push(protocol)
    }

    return Array.from(map.entries())
      .sort((a, b) => a[0].localeCompare(b[0]))
      .map(([category, items]) => ({ category, items }))
  }, [filteredProtocols])

  const handleSelect = (protocol: Protocol) => {
    onSelectProtocol(protocol.id)
  }

  return (
    <Modal
      open={open}
      onClose={() => {
        setSearchTerm('')
        onClose()
      }}
      title="Select Resource Type"
      description="Choose the type of resource to connect to."
      size="xl"
    >
      <div className="flex flex-col gap-6">
        <div className="relative">
          <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            placeholder="Search by name, capability, or category"
            value={searchTerm}
            onChange={(event) => setSearchTerm(event.target.value)}
            className="h-10 pl-9"
          />
        </div>

        {isLoading ? (
          <div className="flex items-center justify-center py-10 text-muted-foreground">
            <Loader2 className="h-5 w-5 animate-spin" />
            <span className="ml-2 text-sm">Loading protocols...</span>
          </div>
        ) : grouped.length === 0 ? (
          <div className="rounded-lg border border-dashed border-border/70 bg-muted/20 p-6 text-center">
            <p className="text-sm font-medium text-foreground">No protocols available</p>
            <p className="mt-1 text-sm text-muted-foreground">
              Adjust your search or check with an administrator about permissions.
            </p>
          </div>
        ) : (
          <div className="space-y-6">
            {grouped.map((group) => (
              <div key={group.category} className="space-y-3">
                <div className="flex items-center gap-2">
                  <h3 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                    {formatCategoryLabel(group.category)}
                  </h3>
                  <span className="text-[11px] font-medium text-muted-foreground/70">
                    {group.items.length} option{group.items.length === 1 ? '' : 's'}
                  </span>
                </div>
                <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
                  {group.items.map((protocol) => (
                    <ProtocolCard key={protocol.id} protocol={protocol} onSelect={handleSelect} />
                  ))}
                </div>
              </div>
            ))}
          </div>
        )}

        <div className="flex justify-end">
          <Button variant="outline" onClick={onClose}>
            Cancel
          </Button>
        </div>
      </div>
    </Modal>
  )
}

function ProtocolCard({
  protocol,
  onSelect,
}: {
  protocol: Protocol
  onSelect: (protocol: Protocol) => void
}) {
  const Icon = resolveProtocolIcon(protocol)
  return (
    <button
      type="button"
      onClick={() => onSelect(protocol)}
      className={cn(
        'group flex h-full flex-col gap-3 rounded-lg border border-border/70 bg-card p-4 text-left shadow-sm transition hover:-translate-y-[2px] hover:border-border hover:shadow-md'
      )}
    >
      <div className="flex items-center gap-3">
        <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-primary/10 text-primary">
          <Icon className="h-5 w-5" />
        </div>
        <div className="flex-1">
          <p className="text-sm font-semibold text-foreground">{protocol.name}</p>
          <p className="text-xs uppercase tracking-wide text-muted-foreground">
            {formatCategoryLabel(protocol.category)}
          </p>
        </div>
      </div>
      {protocol.description ? (
        <p className="text-sm text-muted-foreground">{protocol.description}</p>
      ) : (
        <p className="text-sm text-muted-foreground">No description provided.</p>
      )}
      {protocol.features?.length ? (
        <div className="flex flex-wrap gap-1.5">
          {protocol.features.slice(0, 3).map((feature) => (
            <Badge
              key={feature}
              variant="secondary"
              className="text-[10px] uppercase tracking-wide"
            >
              {feature.replace(/_/g, ' ')}
            </Badge>
          ))}
        </div>
      ) : null}
    </button>
  )
}

function formatCategoryLabel(category: string) {
  return category.replace(/[_-]/g, ' ').replace(/\b\w/g, (char) => char.toUpperCase())
}
