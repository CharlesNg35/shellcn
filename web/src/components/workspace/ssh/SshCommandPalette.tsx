import { Button } from '@/components/ui/Button'
import { Modal } from '@/components/ui/Modal'

interface CommandPaletteTabItem {
  id: string
  label: string
  isActive: boolean
  onSelect: () => void
}

interface CommandPaletteSessionItem {
  id: string
  label: string
  onNavigate: () => void
}

interface SshCommandPaletteProps {
  open: boolean
  onClose: () => void
  tabs: CommandPaletteTabItem[]
  sessions: CommandPaletteSessionItem[]
}

export function SshCommandPalette({ open, onClose, tabs, sessions }: SshCommandPaletteProps) {
  return (
    <Modal
      open={open}
      onClose={onClose}
      title="Command palette"
      description="Switch tabs or jump between active sessions."
      size="lg"
    >
      <div className="space-y-4">
        <div>
          <h3 className="text-sm font-semibold text-foreground">Current workspace</h3>
          <div className="mt-2 grid gap-2">
            {tabs.map((tab) => (
              <Button
                key={tab.id}
                variant={tab.isActive ? 'secondary' : 'outline'}
                className="justify-start"
                onClick={() => {
                  tab.onSelect()
                  onClose()
                }}
              >
                <span className="truncate">{tab.label}</span>
                {tab.isActive && (
                  <span className="ml-auto text-xs text-muted-foreground">Active</span>
                )}
              </Button>
            ))}
          </div>
        </div>

        {sessions.length > 0 && (
          <div>
            <h3 className="text-sm font-semibold text-foreground">Other sessions</h3>
            <div className="mt-2 grid gap-2">
              {sessions.map((session) => (
                <Button
                  key={session.id}
                  variant="outline"
                  className="justify-start"
                  onClick={() => {
                    session.onNavigate()
                    onClose()
                  }}
                >
                  <span className="truncate">{session.label}</span>
                </Button>
              ))}
            </div>
          </div>
        )}
      </div>
    </Modal>
  )
}

export default SshCommandPalette
