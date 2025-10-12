import { fireEvent, render, screen } from '@testing-library/react'
import { vi } from 'vitest'
import { AuditLogTable } from '@/components/audit/AuditLogTable'
import type { AuditLogEntry } from '@/types/audit'

const SAMPLE_LOG: AuditLogEntry = {
  id: 'audit_001',
  username: 'alice',
  action: 'user.create',
  resource: 'user:usr_123',
  result: 'success',
  ip_address: '127.0.0.1',
  user_agent: 'Mozilla/5.0',
  created_at: '2025-01-01T12:00:00Z',
}

describe('AuditLogTable', () => {
  it('renders audit logs and triggers detail selection', () => {
    const handleSelect = vi.fn()

    render(
      <AuditLogTable
        logs={[SAMPLE_LOG]}
        meta={{ page: 1, per_page: 50, total: 1, total_pages: 1 }}
        page={1}
        perPage={50}
        onPageChange={() => {}}
        onSelectLog={handleSelect}
      />
    )

    expect(screen.getByText('alice')).toBeInTheDocument()
    expect(screen.getByText('user.create')).toBeInTheDocument()

    const detailButton = screen.getByRole('button', { name: /details/i })
    fireEvent.click(detailButton)

    expect(handleSelect).toHaveBeenCalledWith(SAMPLE_LOG)
  })
})
