/* eslint-disable react-refresh/only-export-components */
import { TerminalSquare } from 'lucide-react'

import { SshWorkspace } from '@/pages/sessions/SshWorkspace'

import type { WorkspaceDescriptor } from '../types'

function SshWorkspaceMount() {
  return <SshWorkspace />
}

export const SSH_WORKSPACE_DESCRIPTOR: WorkspaceDescriptor = {
  id: 'workspace.ssh',
  protocolId: 'ssh',
  displayName: 'SSH Workspace',
  description: 'Full-featured SSH terminal and file manager.',
  icon: TerminalSquare,
  defaultRoute: (sessionId: string) => `/active-sessions/${sessionId}`,
  mount: SshWorkspaceMount,
  features: {
    supportsSftp: true,
    supportsRecording: true,
    supportsSharing: true,
    supportsSnippets: true,
  },
}
