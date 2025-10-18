import { LifeBuoy } from 'lucide-react'
import { EmptyState } from '@/components/ui/EmptyState'

import { SSH_WORKSPACE_DESCRIPTOR } from './ssh/SshWorkspaceDescriptor'
import { SFTP_WORKSPACE_DESCRIPTOR } from './sftp/SftpWorkspaceDescriptor'
import type { WorkspaceDescriptor, WorkspaceMountProps } from './types'

const descriptorsById = new Map<string, WorkspaceDescriptor>()
const descriptorsByProtocol = new Map<string, WorkspaceDescriptor>()
const descriptorOrder: WorkspaceDescriptor[] = []

const FALLBACK_DESCRIPTOR: WorkspaceDescriptor = {
  id: 'fallback',
  protocolId: 'unknown',
  displayName: 'Workspace coming soon',
  description: 'This protocol does not have a workspace yet. Check back soon.',
  icon: LifeBuoy,
  defaultRoute: (sessionId: string) => `/active-sessions/${sessionId}`,
  mount: function FallbackWorkspace({ descriptor }: WorkspaceMountProps) {
    return (
      <div className="flex h-full items-center justify-center">
        <EmptyState
          title="Workspace not available"
          description={
            descriptor.description ??
            'The requested workspace is not available yet. Please contact your administrator.'
          }
        />
      </div>
    )
  },
  features: {},
}

export function registerWorkspaceDescriptor(descriptor: WorkspaceDescriptor) {
  if (descriptorsById.has(descriptor.id)) {
    descriptorsById.set(descriptor.id, descriptor)
  } else {
    descriptorOrder.push(descriptor)
    descriptorsById.set(descriptor.id, descriptor)
  }
  descriptorsByProtocol.set(descriptor.protocolId, descriptor)
}

export function listWorkspaceDescriptors(): WorkspaceDescriptor[] {
  return [...descriptorOrder]
}

export function getWorkspaceDescriptor(id?: string | null): WorkspaceDescriptor {
  if (!id) {
    return FALLBACK_DESCRIPTOR
  }
  return descriptorsById.get(id) ?? FALLBACK_DESCRIPTOR
}

export function getWorkspaceDescriptorForProtocol(protocolId?: string | null): WorkspaceDescriptor {
  if (!protocolId) {
    return FALLBACK_DESCRIPTOR
  }
  return descriptorsByProtocol.get(protocolId) ?? FALLBACK_DESCRIPTOR
}

export { FALLBACK_DESCRIPTOR }

registerWorkspaceDescriptor(SSH_WORKSPACE_DESCRIPTOR)
registerWorkspaceDescriptor(SFTP_WORKSPACE_DESCRIPTOR)
