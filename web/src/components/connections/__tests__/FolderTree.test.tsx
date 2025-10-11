import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import type { ReactNode } from 'react'
import { FolderTree } from '@/components/connections/FolderTree'
import type { ConnectionFolderNode } from '@/types/connections'

vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom')
  return {
    ...actual,
    Link: ({
      to,
      children,
      ...rest
    }: { to: unknown; children?: ReactNode } & Record<string, unknown>) => (
      <a href={typeof to === 'string' ? to : '#'} {...rest}>
        {children}
      </a>
    ),
  }
})

describe.skip('FolderTree', () => {
  const nodes: ConnectionFolderNode[] = [
    {
      folder: { id: 'root', name: 'Root', parent_id: null },
      connection_count: 2,
      children: [
        {
          folder: { id: 'child', name: 'Child', parent_id: 'root' },
          connection_count: 1,
        },
      ],
    },
  ]

  it('renders folders and toggles children', () => {
    const onSelect = vi.fn()
    render(<FolderTree nodes={nodes} onSelect={onSelect} />)

    expect(screen.getByText('Root')).toBeInTheDocument()
    expect(screen.getByText('Child')).toBeInTheDocument()

    // collapse root
    const toggle = screen.getByRole('button')
    fireEvent.click(toggle)
    expect(screen.queryByText('Child')).not.toBeInTheDocument()

    fireEvent.click(screen.getByText('Root'))
    expect(onSelect).toHaveBeenCalledWith('root')
  })
})
