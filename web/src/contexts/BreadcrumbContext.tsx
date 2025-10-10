import { createContext, useContext, useState, type ReactNode } from 'react'

interface BreadcrumbOverride {
  [path: string]: string
}

interface BreadcrumbContextValue {
  overrides: BreadcrumbOverride
  setOverride: (path: string, label: string) => void
  clearOverride: (path: string) => void
}

const BreadcrumbContext = createContext<BreadcrumbContextValue | undefined>(undefined)

export function BreadcrumbProvider({ children }: { children: ReactNode }) {
  const [overrides, setOverrides] = useState<BreadcrumbOverride>({})

  const setOverride = (path: string, label: string) => {
    setOverrides((prev) => ({ ...prev, [path]: label }))
  }

  const clearOverride = (path: string) => {
    setOverrides((prev) => {
      const next = { ...prev }
      delete next[path]
      return next
    })
  }

  return (
    <BreadcrumbContext.Provider value={{ overrides, setOverride, clearOverride }}>
      {children}
    </BreadcrumbContext.Provider>
  )
}

// eslint-disable-next-line react-refresh/only-export-components
export function useBreadcrumb() {
  const context = useContext(BreadcrumbContext)
  if (!context) {
    throw new Error('useBreadcrumb must be used within BreadcrumbProvider')
  }
  return context
}
