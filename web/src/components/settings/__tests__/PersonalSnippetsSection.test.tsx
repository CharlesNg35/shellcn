import type { ReactNode } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { PersonalSnippetsSection } from '../PersonalSnippetsSection'
import type { SnippetRecord } from '@/lib/api/snippets'

vi.mock('sonner', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}))

const useSnippetsMock = vi.fn()

vi.mock('@/hooks/useSnippets', () => ({
  useSnippets: () => useSnippetsMock(),
  SNIPPETS_QUERY_KEY: ['snippets'] as const,
}))

const createSnippetMock = vi.fn()
const updateSnippetMock = vi.fn()
const deleteSnippetMock = vi.fn()

vi.mock('@/lib/api/snippets', () => ({
  createSnippet: (...args: unknown[]) => createSnippetMock(...args),
  updateSnippet: (...args: unknown[]) => updateSnippetMock(...args),
  deleteSnippet: (...args: unknown[]) => deleteSnippetMock(...args),
}))

describe('PersonalSnippetsSection', () => {
  beforeEach(() => {
    useSnippetsMock.mockReset()
    createSnippetMock.mockReset()
    updateSnippetMock.mockReset()
    deleteSnippetMock.mockReset()
    createSnippetMock.mockResolvedValue({} as SnippetRecord)
    updateSnippetMock.mockResolvedValue({} as SnippetRecord)
    deleteSnippetMock.mockResolvedValue(undefined)
  })

  const createWrapper = (ui: ReactNode) => {
    const queryClient = new QueryClient()
    return render(<QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>)
  }

  it('renders empty state when no snippets exist', () => {
    useSnippetsMock.mockReturnValue({ data: [], isLoading: false })
    createWrapper(<PersonalSnippetsSection />)
    expect(screen.getByText(/You haven’t created any personal snippets yet/i)).toBeInTheDocument()
  })

  it('allows creating a new snippet', async () => {
    const user = userEvent.setup()
    useSnippetsMock.mockReturnValue({ data: [], isLoading: false })
    createSnippetMock.mockResolvedValue({
      id: 'snp-1',
      name: 'Tail logs',
      command: 'tail -f /var/log/app.log',
      scope: 'user',
    })

    createWrapper(<PersonalSnippetsSection />)

    await user.click(screen.getByRole('button', { name: /add snippet/i }))

    await user.type(screen.getByLabelText('Name'), 'Tail logs')
    await user.type(screen.getByLabelText('Command'), 'tail -f /var/log/app.log')

    await user.click(screen.getByRole('button', { name: /create snippet/i }))

    await waitFor(() =>
      expect(createSnippetMock).toHaveBeenCalledWith({
        name: 'Tail logs',
        description: undefined,
        command: 'tail -f /var/log/app.log',
        scope: 'user',
      })
    )
  })

  it('shows existing snippets with actions', () => {
    const snippets: SnippetRecord[] = [
      {
        id: 'snp-1',
        name: 'List processes',
        description: 'Show running processes',
        command: 'ps aux',
        scope: 'user',
      },
    ]
    useSnippetsMock.mockReturnValue({ data: snippets, isLoading: false })

    createWrapper(<PersonalSnippetsSection />)

    expect(screen.getByText('List processes')).toBeInTheDocument()
    expect(screen.getByText('ps aux')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /edit/i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /delete/i })).toBeInTheDocument()
  })
})
