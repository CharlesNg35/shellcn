import {
  Cloud,
  Container,
  Database,
  Folder,
  HardDrive,
  Monitor,
  Network,
  Server,
} from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import type { Protocol } from '@/types/protocols'

const CATEGORY_ICON_MAP: Record<string, LucideIcon> = {
  terminal: Server,
  desktop: Monitor,
  container: Container,
  database: Database,
  object_storage: Folder,
  vm: HardDrive,
  network: Network,
  cloud: Cloud,
}

const ICON_NAME_MAP: Record<string, LucideIcon> = {
  server: Server,
  monitor: Monitor,
  database: Database,
  container: Container,
  cloud: Cloud,
  harddrive: HardDrive,
  hard_drive: HardDrive,
  folder: Folder,
  files: Folder,
}

export const DEFAULT_PROTOCOL_ICON: LucideIcon = Server

export function resolveProtocolIcon(protocol?: Protocol): LucideIcon {
  if (protocol?.icon) {
    const iconKey = protocol.icon.toLowerCase()
    if (ICON_NAME_MAP[iconKey]) {
      return ICON_NAME_MAP[iconKey]
    }
  }

  if (protocol?.category) {
    const categoryKey = protocol.category.toLowerCase()
    if (CATEGORY_ICON_MAP[categoryKey]) {
      return CATEGORY_ICON_MAP[categoryKey]
    }
  }

  return DEFAULT_PROTOCOL_ICON
}
