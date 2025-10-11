import { act, fireEvent, render, screen } from '@testing-library/react'
import { vi } from 'vitest'
import { UserFilters, type UserFilterState } from '@/components/users/UserFilters'

describe('UserFilters', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.runOnlyPendingTimers()
    vi.useRealTimers()
  })

  it('updates search filter with debounce', () => {
    const onChange = vi.fn()
    const filters: UserFilterState = { status: 'all', search: '' }
    render(<UserFilters filters={filters} onChange={onChange} />)

    const input = screen.getByLabelText(/search/i)
    fireEvent.change(input, { target: { value: 'alice' } })

    act(() => {
      vi.advanceTimersByTime(300)
    })

    expect(onChange).toHaveBeenCalledWith({ ...filters, search: 'alice' })
  })

  it('changes status filter immediately when clicking buttons', () => {
    const onChange = vi.fn()
    render(<UserFilters filters={{ status: 'all' }} onChange={onChange} />)

    fireEvent.click(screen.getByRole('button', { name: /^Active$/i }))

    expect(onChange).toHaveBeenCalledWith({ status: 'active' })
  })
})
