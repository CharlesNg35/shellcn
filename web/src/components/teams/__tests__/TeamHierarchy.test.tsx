import { fireEvent, render, screen } from '@testing-library/react'
import { vi } from 'vitest'
import { TeamHierarchy } from '@/components/teams/TeamHierarchy'
import type { TeamRecord } from '@/types/teams'

describe('TeamHierarchy', () => {
  const teams: TeamRecord[] = [
    { id: 'team-1', name: 'Security', description: 'Parent team' },
    { id: 'team-2', name: 'Security/Incident Response' },
    { id: 'team-3', name: 'Security/Threat Hunting' },
    { id: 'team-4', name: 'Engineering' },
  ]

  it('renders hierarchical structure based on team names', () => {
    const handleSelect = vi.fn()

    render(<TeamHierarchy teams={teams} selectedTeamId="team-2" onSelectTeam={handleSelect} />)

    expect(screen.getByText('Security/Incident Response')).toBeInTheDocument()
    expect(screen.getByText('Security/Threat Hunting')).toBeInTheDocument()
    expect(screen.getByText('Engineering')).toBeInTheDocument()

    fireEvent.click(screen.getByText('Engineering'))
    expect(handleSelect).toHaveBeenCalledWith('team-4')
  })
})
