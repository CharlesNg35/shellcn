import { fireEvent, render, screen } from '@testing-library/react'
import { vi } from 'vitest'
import { AuditFilters } from '@/components/audit/AuditFilters'

describe('AuditFilters', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('debounces search input changes before calling onChange', () => {
    const handleChange = vi.fn()

    render(<AuditFilters filters={{}} onChange={handleChange} />)

    const searchInput = screen.getByPlaceholderText(/search by actor/i)
    fireEvent.change(searchInput, { target: { value: 'alice' } })

    expect(handleChange).not.toHaveBeenCalled()

    vi.advanceTimersByTime(300)

    expect(handleChange).toHaveBeenCalledWith(expect.objectContaining({ search: 'alice' }))
  })

  it('resets filters when clicking the reset button', () => {
    const handleChange = vi.fn()

    render(
      <AuditFilters
        filters={{ action: 'user.create', result: 'success', actor: 'alice' }}
        onChange={handleChange}
      />
    )

    const resetButton = screen.getByRole('button', { name: /reset/i })
    fireEvent.click(resetButton)

    expect(handleChange).toHaveBeenCalledWith({ result: 'all' })
  })
})
