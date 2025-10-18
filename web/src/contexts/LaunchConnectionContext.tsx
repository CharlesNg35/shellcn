import { createContext, useContext, type ReactNode } from 'react'

import type { ConnectionRecord } from '@/types/connections'
import { useLaunchConnection } from '@/hooks/useLaunchConnection'
import LaunchConnectionModal from '@/components/connections/LaunchConnectionModal'

interface LaunchConnectionContextValue {
  open: (connection: ConnectionRecord) => void
  openById: (connectionId: string) => Promise<void>
  close: () => void
  isOpen: boolean
}

const LaunchConnectionContext = createContext<LaunchConnectionContextValue | undefined>(undefined)

interface LaunchConnectionProviderProps {
  children: ReactNode
}

export function LaunchConnectionProvider({ children }: LaunchConnectionProviderProps) {
  const launch = useLaunchConnection()

  return (
    <LaunchConnectionContext.Provider
      value={{
        open: launch.open,
        openById: launch.openById,
        close: launch.close,
        isOpen: launch.state.isOpen,
      }}
    >
      {children}
      <LaunchConnectionModal
        open={launch.state.isOpen}
        connection={launch.state.connection}
        descriptor={launch.descriptor}
        template={launch.template}
        activeSessions={launch.activeSessions}
        isFetchingSessions={launch.isFetchingSessions}
        isLaunching={launch.isLaunching}
        errorMessage={launch.errorMessage}
        onClose={launch.close}
        onLaunch={launch.launch}
        onResumeSession={launch.resumeSession}
      />
    </LaunchConnectionContext.Provider>
  )
}

// eslint-disable-next-line react-refresh/only-export-components
export function useLaunchConnectionContext(): LaunchConnectionContextValue {
  const context = useContext(LaunchConnectionContext)
  if (!context) {
    throw new Error('useLaunchConnectionContext must be used within a LaunchConnectionProvider')
  }
  return context
}
