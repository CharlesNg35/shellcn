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

vi.mock('@/hooks/useMonitoring', () => ({
  useMonitoringSummary: vi.fn(),
  useHealthReadiness: vi.fn(),
  useHealthLiveness: vi.fn(),
}))

import { Security } from '@/pages/settings/Security'
import { PERMISSIONS } from '@/constants/permissions'
import { useSecurityAudit } from '@/hooks/useSecurityAudit'
import { useMonitoringSummary, useHealthReadiness, useHealthLiveness } from '@/hooks/useMonitoring'

describe('Security settings page', () => {
  const mockedUseSecurityAudit = vi.mocked(useSecurityAudit)
  const mockedUseMonitoringSummary = vi.mocked(useMonitoringSummary)
  const mockedUseHealthReadiness = vi.mocked(useHealthReadiness)
  const mockedUseHealthLiveness = vi.mocked(useHealthLiveness)
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

    mockedUseMonitoringSummary.mockReturnValue({
      data: {
        summary: {
          generated_at: new Date('2025-01-01T12:00:00Z').toISOString(),
          auth: { success: 5, failure: 1, error: 0 },
          permissions: { allowed: 10, denied: 2, error: 0 },
          sessions: {
            active: 3,
            completed: 12,
            average_duration_seconds: 42,
            last_duration: 5_000_000_000,
            last_ended_at: new Date('2025-01-01T12:05:00Z').toISOString(),
          },
          realtime: {
            active_connections: 4,
            broadcasts: 20,
            failures: 0,
            last_failure: null,
          },
          maintenance: { jobs: [] },
          protocols: [],
        },
        prometheus: {
          enabled: true,
          endpoint: '/metrics',
        },
      },
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useMonitoringSummary>)

    const readinessReport = {
      success: true,
      status: 'up' as const,
      checks: [
        { component: 'database', status: 'up' as const, details: 'OK', duration: 1_000_000 },
      ],
      checked_at: new Date('2025-01-01T12:00:00Z').toISOString(),
    }

    mockedUseHealthReadiness.mockReturnValue({
      data: readinessReport,
      isLoading: false,
      isError: false,
      error: null,
    } as unknown as ReturnType<typeof useHealthReadiness>)

    mockedUseHealthLiveness.mockReturnValue({
      data: readinessReport,
      isLoading: false,
      isError: false,
      error: null,
    } as unknown as ReturnType<typeof useHealthLiveness>)
  })

  it('renders audit results when permitted', () => {
    render(<Security />)

    expect(screen.getByText('Security & Monitoring')).toBeInTheDocument()
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

  it('renders monitoring data when permitted', async () => {
    render(<Security />)

    const monitoringTab = screen.getByRole('tab', { name: /Monitoring/i })
    fireEvent.mouseDown(monitoringTab)
    fireEvent.click(monitoringTab)

    expect(await screen.findByText('Readiness')).toBeInTheDocument()
    expect(screen.getByText('Prometheus')).toBeInTheDocument()
    expect(screen.getByText('Authentication')).toBeInTheDocument()
  })

  it('shows monitoring fallback when permission missing', async () => {
    mockHasPermission.mockImplementation((permission) => permission !== PERMISSIONS.MONITORING.VIEW)
    render(<Security />)

    const monitoringTab = screen.getByRole('tab', { name: /Monitoring/i })
    fireEvent.mouseDown(monitoringTab)
    fireEvent.click(monitoringTab)

    expect(await screen.findByText('Monitoring unavailable')).toBeInTheDocument()
  })
})
