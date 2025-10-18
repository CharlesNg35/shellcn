/* eslint-disable react-refresh/only-export-components */
import { Folder } from 'lucide-react'

import { SessionFileManager } from '@/pages/sessions/SessionFileManager'

import type { WorkspaceDescriptor } from '../types'

function SftpWorkspaceMount() {
  return <SessionFileManager />
}

export const SFTP_WORKSPACE_DESCRIPTOR: WorkspaceDescriptor = {
  id: 'workspace.sftp',
  protocolId: 'sftp',
  displayName: 'SFTP Workspace',
  description: 'Standalone SFTP file manager for file-transfer only connections.',
  icon: Folder,
  defaultRoute: (sessionId: string) => `/active-sessions/${sessionId}`,
  mount: SftpWorkspaceMount,
  features: {
    supportsSftp: true,
  },
}

export default SFTP_WORKSPACE_DESCRIPTOR
