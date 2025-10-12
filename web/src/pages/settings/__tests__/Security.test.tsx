import { fireEvent, render, screen } from '@testing-library/react'
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

vi.mock('@/hooks/useSecurityAudit', () => ({
  useSecurityAudit: vi.fn(),
}))

import { Security } from '@/pages/settings/Security'
import { useSecurityAudit } from '@/hooks/useSecurityAudit'

describe('Security settings page', () => {
  const mockedUseSecurityAudit = vi.mocked(useSecurityAudit)
  const mockRefetch = vi.fn()

  beforeEach(() => {
    vi.clearAllMocks()
    mockHasPermission.mockReturnValue(true)

    mockedUseSecurityAudit.mockReturnValue({
      data: {
        checked_at: new Date('2025-01-01T12:00:00Z').toISOString(),
        summary: {
          pass: 2,
          warn: 1,
          fail: 0,
        },
        checks: [
          {
            id: 'root_user_present',
            status: 'pass',
            message: 'Root user present.',
          },
          {
            id: 'jwt_secret_strength',
            status: 'warn',
            message: 'JWT signing secret is 40 bytes.',
            remediation: 'Consider increasing to at least 48 bytes.',
          },
        ],
      },
      isLoading: false,
      isFetching: false,
      isError: false,
      error: null,
      refetch: mockRefetch,
    } as unknown as ReturnType<typeof useSecurityAudit>)
  })

  it('renders audit results when permitted', () => {
    render(<Security />)

    expect(screen.getByText('Security Posture')).toBeInTheDocument()
    expect(screen.getByText('Passing checks')).toBeInTheDocument()
    expect(screen.getByText('2')).toBeInTheDocument()
    expect(screen.getByText('Warnings')).toBeInTheDocument()
    expect(screen.getByText('Root user present.')).toBeInTheDocument()
    expect(screen.getByText('JWT signing secret is 40 bytes.')).toBeInTheDocument()
  })

  it('runs audit again when clicking Run audit', () => {
    render(<Security />)

    const runAuditButton = screen.getByRole('button', { name: /Run audit/i })
    fireEvent.click(runAuditButton)

    expect(mockRefetch).toHaveBeenCalled()
  })

  it('shows fallback when user lacks permission', () => {
    mockHasPermission.mockReturnValue(false)
    render(<Security />)

    expect(screen.getByText('Security audit unavailable')).toBeInTheDocument()
    expect(screen.queryByText('Run audit')).not.toBeInTheDocument()
  })
})
