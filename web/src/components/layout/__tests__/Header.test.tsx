import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent, act } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import * as useAuthModule from '@/hooks/useAuth'
import * as useNotificationsModule from '@/hooks/useNotifications'
import { Header } from '../Header'
import { BreadcrumbProvider } from '@/contexts/BreadcrumbContext'

vi.mock('@/hooks/useAuth', () => ({
  useAuth: vi.fn(),
}))

vi.mock('@/hooks/useNotifications', () => ({
  useNotifications: vi.fn(),
}))

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

    vi.spyOn(useNotificationsModule, 'useNotifications').mockReturnValue({
      notifications: [],
      unreadCount: 0,
      isConnected: false,
      isLoading: false,
      refetch: vi.fn(),
      markAsRead: vi.fn(),
      markAsUnread: vi.fn(),
      removeNotification: vi.fn(),
      markAllAsRead: vi.fn(),
    } as unknown as ReturnType<typeof useNotificationsModule.useNotifications>)

    await act(async () => {
      render(
        <MemoryRouter>
          <BreadcrumbProvider>
            <Header />
          </BreadcrumbProvider>
        </MemoryRouter>
      )
    })

    // Open user menu by clicking on the user display name inside the menu button
    const nameEl = await screen.findByText('Alice Smith')
    await act(async () => {
      fireEvent.click(nameEl)
    })

    // Wait for menu to open and verify items are visible
    const profileLink = await screen.findByText(/Profile/i)
    expect(profileLink).toBeInTheDocument()
    expect(screen.getByText(/Settings/i)).toBeInTheDocument()

    const signOutButton = screen.getByRole('button', { name: /Sign out/i })
    await act(async () => {
      fireEvent.click(signOutButton)
    })

    expect(logout).toHaveBeenCalled()
  })
})
