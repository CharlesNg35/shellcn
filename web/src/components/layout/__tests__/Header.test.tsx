import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import * as useAuthModule from '@/hooks/useAuth'
import { Header } from '../Header'

vi.mock('@/hooks/useAuth')

describe('Header', () => {
  it('renders user menu and allows logout', async () => {
    const logout = vi.fn(async () => {})
    vi.spyOn(useAuthModule, 'useAuth').mockReturnValue({
      user: {
        id: 'u1',
        username: 'alice',
        email: 'alice@example.com',
        first_name: 'Alice',
        last_name: 'Smith',
        is_root: false,
        is_active: true,
        permissions: [],
        roles: [],
      },
      logout,
    } as unknown as ReturnType<typeof useAuthModule.useAuth>)

    render(
      <MemoryRouter>
        <Header />
      </MemoryRouter>
    )

    // Open user menu by clicking on the user display name inside the menu button
    const nameEl = await screen.findByText('Alice Smith')
    fireEvent.click(nameEl)

    // Opened menu should show Profile and Settings links and Sign out button
    expect(await screen.findByText(/Profile/i)).toBeInTheDocument()
    expect(screen.getByText(/Settings/i)).toBeInTheDocument()

    const signOutButton = screen.getByRole('button', { name: /Sign out/i })
    fireEvent.click(signOutButton)

    expect(logout).toHaveBeenCalled()
  })
})
