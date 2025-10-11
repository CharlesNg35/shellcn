import type { ComponentType } from 'react'
import {
  Box,
  Cloud,
  Database,
  Folder,
  HardDrive,
  Layers,
  Monitor,
  Network,
  Package,
  Server,
  Terminal,
} from 'lucide-react'

export interface ConnectionIconOption {
  id: string
  label: string
  icon: ComponentType<{ className?: string }>
}

export interface ConnectionColorOption {
  id: string
  label: string
  value: string
}

export const CONNECTION_ICON_OPTIONS: ConnectionIconOption[] = [
  { id: 'server', label: 'Server', icon: Server },
  { id: 'terminal', label: 'Terminal', icon: Terminal },
  { id: 'database', label: 'Database', icon: Database },
  { id: 'cloud', label: 'Cloud', icon: Cloud },
  { id: 'container', label: 'Container', icon: Package },
  { id: 'layers', label: 'Layers', icon: Layers },
  { id: 'harddrive', label: 'VM / Disk', icon: HardDrive },
  { id: 'network', label: 'Network', icon: Network },
  { id: 'folder', label: 'Workspace', icon: Folder },
  { id: 'box', label: 'Appliance', icon: Box },
  { id: 'monitor', label: 'Desktop', icon: Monitor },
]

export const CONNECTION_COLOR_OPTIONS: ConnectionColorOption[] = [
  { id: 'blue', label: 'Blue', value: '#2563eb' },
  { id: 'green', label: 'Green', value: '#16a34a' },
  { id: 'red', label: 'Red', value: '#dc2626' },
  { id: 'yellow', label: 'Yellow', value: '#f59e0b' },
  { id: 'purple', label: 'Purple', value: '#7c3aed' },
  { id: 'teal', label: 'Teal', value: '#0d9488' },
  { id: 'pink', label: 'Pink', value: '#db2777' },
  { id: 'gray', label: 'Gray', value: '#4b5563' },
]

export const DEFAULT_CONNECTION_ICON_ID = 'server'

export function resolveConnectionIcon(iconId?: string) {
  const match = CONNECTION_ICON_OPTIONS.find((option) => option.id === iconId)
  return match?.icon ?? Server
}
