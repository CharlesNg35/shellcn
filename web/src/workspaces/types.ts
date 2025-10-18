import type { ComponentType } from 'react'
import type { LucideIcon } from 'lucide-react'

import type { ActiveConnectionSession } from '@/types/connections'
import type { WorkspaceViewType } from '@/store/protocol-workspace-tabs-store'

export interface WorkspaceDescriptorFeatures {
  supportsSftp?: boolean
  supportsRecording?: boolean
  supportsSharing?: boolean
  supportsSnippets?: boolean
  [key: string]: boolean | undefined
}

export interface WorkspaceLaunchOptionDefinition {
  id: string
  label: string
  description?: string
  defaultEnabled?: boolean
  requiresFeature?: keyof WorkspaceDescriptorFeatures
}

export interface WorkspaceTabDefinition {
  type: WorkspaceViewType
  title?: string
  closable?: boolean
}

export interface WorkspaceMountProps {
  sessionId: string
  session?: ActiveConnectionSession
  allSessions: ActiveConnectionSession[]
  descriptor: WorkspaceDescriptor
  isLoading: boolean
  isError: boolean
}

export interface WorkspaceDescriptor {
  id: string
  protocolId: string
  displayName: string
  icon: LucideIcon
  description?: string
  defaultRoute: (sessionId: string) => string
  mount: ComponentType<WorkspaceMountProps>
  features: WorkspaceDescriptorFeatures
  defaultTabs?: WorkspaceTabDefinition[]
  launchOptions?: WorkspaceLaunchOptionDefinition[]
}
