import type { ComponentType } from 'react'
import {
  Box,
  Cloud,
  Database,
  Folder,
  FolderGit,
  FolderKanban,
  FolderOpen,
  FolderTree,
  Layers,
  Package,
  Server,
} from 'lucide-react'

export interface FolderIconOption {
  id: string
  label: string
  icon: ComponentType<{ className?: string }>
}

export interface FolderColorOption {
  id: string
  label: string
  value: string
}

export const FOLDER_ICON_OPTIONS: FolderIconOption[] = [
  { id: 'folder', label: 'Folder', icon: Folder },
  { id: 'folder-open', label: 'Open Folder', icon: FolderOpen },
  { id: 'folder-tree', label: 'Tree', icon: FolderTree },
  { id: 'folder-kanban', label: 'Kanban', icon: FolderKanban },
  { id: 'folder-git', label: 'Git', icon: FolderGit },
  { id: 'database', label: 'Database', icon: Database },
  { id: 'server', label: 'Server', icon: Server },
  { id: 'cloud', label: 'Cloud', icon: Cloud },
  { id: 'package', label: 'Package', icon: Package },
  { id: 'layers', label: 'Layers', icon: Layers },
  { id: 'box', label: 'Box', icon: Box },
]

export const FOLDER_COLOR_OPTIONS: FolderColorOption[] = [
  { id: 'blue', label: 'Blue', value: '#3b82f6' },
  { id: 'green', label: 'Green', value: '#10b981' },
  { id: 'red', label: 'Red', value: '#ef4444' },
  { id: 'yellow', label: 'Yellow', value: '#f59e0b' },
  { id: 'purple', label: 'Purple', value: '#8b5cf6' },
  { id: 'pink', label: 'Pink', value: '#ec4899' },
  { id: 'indigo', label: 'Indigo', value: '#6366f1' },
  { id: 'gray', label: 'Gray', value: '#6b7280' },
]

export const DEFAULT_FOLDER_ICON_ID = 'folder'

export function resolveFolderIcon(iconId?: string) {
  const match = FOLDER_ICON_OPTIONS.find((option) => option.id === iconId)
  return match?.icon ?? Folder
}
