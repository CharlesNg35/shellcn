import { fireEvent, render, screen } from '@testing-library/react'
import type { ReactNode } from 'react'
import { describe, expect, it, vi } from 'vitest'
import SshWorkspaceTabsBar from '@/components/workspace/ssh/SshWorkspaceTabsBar'
import type { WorkspaceTab } from '@/store/ssh-session-tabs-store'

vi.mock('framer-motion', () => ({
  Reorder: {
    Group: ({ children }: { children: ReactNode }) => <div>{children}</div>,
    Item: ({
      children,
      onClick,
      'data-testid': dataTestId,
    }: {
      children: ReactNode
      onClick?: () => void
      'data-testid'?: string
    }) => (
      <div data-testid={dataTestId} onClick={onClick} role="presentation">
        {children}
      </div>
    ),
  },
  AnimatePresence: ({ children }: { children: ReactNode }) => <>{children}</>,
  motion: { button: 'button' },
}))

function createTabs(): WorkspaceTab[] {
  return [
    {
      id: 'sess-1:terminal',
      sessionId: 'sess-1',
      type: 'terminal',
      title: 'Terminal',
      closable: false,
    },
    {
      id: 'sess-1:sftp',
      sessionId: 'sess-1',
      type: 'sftp',
      title: 'Files',
      closable: true,
    },
  ]
}

describe('SshWorkspaceTabsBar', () => {
  it('invokes callbacks when selecting and closing tabs', () => {
    const tabs = createTabs()
    const onSelect = vi.fn()
    const onClose = vi.fn()

    render(
      <SshWorkspaceTabsBar
        tabs={tabs}
        activeTabId={tabs[0]!.id}
        onTabSelect={onSelect}
        onTabClose={onClose}
      />
    )

    fireEvent.click(screen.getByText('Files'))
    expect(onSelect).toHaveBeenCalledWith('sess-1:sftp')

    fireEvent.click(screen.getByLabelText('Close Files'))
    expect(onClose).toHaveBeenCalledWith('sess-1:sftp')
  })

  it('syncs rendered order when tabs prop changes', () => {
    const tabs = createTabs()
    const { rerender, container } = render(
      <SshWorkspaceTabsBar
        tabs={tabs}
        activeTabId={tabs[0]!.id}
        onTabSelect={vi.fn()}
        onTabClose={vi.fn()}
      />
    )

    const getOrder = () =>
      Array.from(container.querySelectorAll<HTMLElement>('[data-testid^="workspace-tab-"]')).map(
        (element) => element.dataset.testid
      )

    expect(getOrder()).toEqual(['workspace-tab-sess-1:terminal', 'workspace-tab-sess-1:sftp'])

    const swapped = [...tabs].reverse()
    rerender(
      <SshWorkspaceTabsBar
        tabs={swapped}
        activeTabId={swapped[0]!.id}
        onTabSelect={vi.fn()}
        onTabClose={vi.fn()}
      />
    )

    expect(getOrder()).toEqual(['workspace-tab-sess-1:sftp', 'workspace-tab-sess-1:terminal'])
  })

  it('appends new tabs to ordering while preserving existing sequence', () => {
    const [terminal, sftp] = createTabs()
    const { rerender, container } = render(
      <SshWorkspaceTabsBar
        tabs={[terminal]}
        activeTabId={terminal!.id}
        onTabSelect={vi.fn()}
        onTabClose={vi.fn()}
      />
    )

    const getOrder = () =>
      Array.from(container.querySelectorAll<HTMLElement>('[data-testid^="workspace-tab-"]')).map(
        (element) => element.dataset.testid
      )

    expect(getOrder()).toEqual(['workspace-tab-sess-1:terminal'])

    rerender(
      <SshWorkspaceTabsBar
        tabs={[terminal, sftp]}
        activeTabId={terminal!.id}
        onTabSelect={vi.fn()}
        onTabClose={vi.fn()}
      />
    )

    expect(getOrder()).toEqual(['workspace-tab-sess-1:terminal', 'workspace-tab-sess-1:sftp'])
  })
})
