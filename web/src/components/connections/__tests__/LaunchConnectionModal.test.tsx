import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'

import { LaunchConnectionModal } from '../LaunchConnectionModal'
import type { ConnectionRecord } from '@/types/connections'
import type { ConnectionTemplate } from '@/types/protocols'
import type { WorkspaceDescriptor } from '@/workspaces/types'

const mockConnection: ConnectionRecord = {
  id: 'conn-123',
  name: 'Staging SSH',
  description: 'Primary staging server',
  protocol_id: 'ssh',
  team_id: null,
  owner_user_id: 'user-1',
  folder_id: null,
  metadata: {
    tags: ['staging', 'critical'],
    connection_template: {
      version: '1.0.0',
      driver_id: 'ssh-driver',
      fields: {
        host: 'staging.example.com',
        port: 22,
        username: 'deploy',
      },
    },
  },
  settings: {
    host: 'staging.example.com',
    port: 22,
  },
  identity_id: 'identity-1',
  last_used_at: '2024-01-01T00:00:00Z',
  targets: [
    {
      id: 'target-1',
      host: 'staging.example.com',
      port: 22,
    },
  ],
  shares: [],
  share_summary: undefined,
  folder: undefined,
}

const mockTemplate: ConnectionTemplate = {
  driverId: 'ssh-driver',
  version: '1.0.0',
  displayName: 'SSH Connection',
  description: 'SSH template',
  sections: [
    {
      id: 'general',
      label: 'General',
      fields: [
        { key: 'host', label: 'Host', required: true, type: 'string' },
        { key: 'port', label: 'Port', required: true, type: 'number' },
        { key: 'username', label: 'Username', required: true, type: 'string' },
      ],
    },
  ],
}

const mockDescriptor: WorkspaceDescriptor = {
  id: 'workspace.ssh',
  protocolId: 'ssh',
  displayName: 'SSH Workspace',
  icon: () => null,
  defaultRoute: (sessionId: string) => `/active-sessions/${sessionId}`,
  mount: () => null,
  features: {
    supportsSftp: true,
    supportsRecording: true,
  },
}

describe('LaunchConnectionModal', () => {
  it('renders connection details and template fields', () => {
    render(
      <LaunchConnectionModal
        open
        connection={mockConnection}
        descriptor={mockDescriptor}
        template={mockTemplate}
        activeSessions={[]}
        onClose={vi.fn()}
        onLaunch={vi.fn()}
        onResumeSession={vi.fn()}
      />
    )

    expect(screen.getByText('Launch Staging SSH')).toBeInTheDocument()
    expect(screen.getAllByText('Host').length).toBeGreaterThan(0)
    expect(screen.getAllByText('Port').length).toBeGreaterThan(0)
    expect(screen.getAllByText('Username').length).toBeGreaterThan(0)
    expect(screen.getAllByText('staging.example.com').length).toBeGreaterThan(0)
    expect(screen.getAllByText('22').length).toBeGreaterThan(0)
    expect(screen.getAllByText('deploy').length).toBeGreaterThan(0)
  })

  it('calls onResumeSession when resume button clicked', () => {
    const onResume = vi.fn()
    render(
      <LaunchConnectionModal
        open
        connection={mockConnection}
        descriptor={mockDescriptor}
        template={mockTemplate}
        activeSessions={[
          {
            id: 'sess-1',
            connection_id: 'conn-123',
            connection_name: 'Staging SSH',
            user_id: 'user-123',
            user_name: 'Alice',
            team_id: null,
            protocol_id: 'ssh',
            started_at: '2024-01-01T00:00:00Z',
            last_seen_at: new Date().toISOString(),
            metadata: {},
            participants: {},
          },
        ]}
        onClose={vi.fn()}
        onLaunch={vi.fn()}
        onResumeSession={onResume}
      />
    )

    const resumeButton = screen.getByRole('button', { name: /Resume/i })
    fireEvent.click(resumeButton)
    expect(onResume).toHaveBeenCalledTimes(1)
  })

  it('calls onLaunch when launch button clicked', () => {
    const onLaunch = vi.fn().mockResolvedValue(undefined)
    render(
      <LaunchConnectionModal
        open
        connection={mockConnection}
        descriptor={mockDescriptor}
        template={mockTemplate}
        activeSessions={[]}
        onClose={vi.fn()}
        onLaunch={onLaunch}
        onResumeSession={vi.fn()}
      />
    )

    const launchButton = screen.getByRole('button', { name: /Launch new session/i })
    fireEvent.click(launchButton)
    expect(onLaunch).toHaveBeenCalledTimes(1)
  })
})
