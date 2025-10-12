import { render, screen } from '@testing-library/react'
import { vi } from 'vitest'

const mockHasPermission = vi.fn<(permission: string) => boolean>()

vi.mock('@/hooks/usePermissions', () => ({
  usePermissions: () => ({
    permissions: [],
    hasPermission: (permission: string) => mockHasPermission(permission),
    hasAnyPermission: (permissions: ReadonlyArray<string>) =>
      permissions.some((permission) => mockHasPermission(permission)),
    hasAllPermissions: (permissions: ReadonlyArray<string>) =>
      permissions.every((permission) => mockHasPermission(permission)),
    isLoading: false,
    refetch: vi.fn(),
  }),
}))

vi.mock('@/hooks/useAuditLogs', () => ({
  useAuditLogs: vi.fn(),
}))

import { AuditLogs } from '@/pages/settings/AuditLogs'
import { useAuditLogs } from '@/hooks/useAuditLogs'

describe('AuditLogs settings page', () => {
  const mockedUseAuditLogs = vi.mocked(useAuditLogs)

  beforeEach(() => {
    vi.clearAllMocks()
    mockHasPermission.mockReturnValue(true)

    mockedUseAuditLogs.mockReturnValue({
      data: {
        data: [
          {
            id: 'audit_01',
            username: 'alice',
            action: 'user.create',
            resource: 'user:usr_123',
            result: 'success',
            ip_address: '127.0.0.1',
            user_agent: 'Mozilla/5.0',
            created_at: '2025-01-01T12:00:00Z',
          },
        ],
        meta: {
          page: 1,
          per_page: 50,
          total: 1,
          total_pages: 1,
        },
      },
      isLoading: false,
      isFetching: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useAuditLogs>)
  })

  it('renders audit logs when permissions allow access', () => {
    render(<AuditLogs />)

    expect(screen.getByText('Audit Logs')).toBeInTheDocument()
    expect(screen.getByText('alice')).toBeInTheDocument()
    expect(screen.getByText('user.create')).toBeInTheDocument()
  })

  it('shows fallback when audit view permission is denied', () => {
    mockHasPermission.mockReturnValue(false)
    render(<AuditLogs />)

    expect(screen.getByText('Audit logs unavailable')).toBeInTheDocument()
  })
})
