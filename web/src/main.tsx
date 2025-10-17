import { StrictMode, useEffect, useState, type ComponentType } from 'react'
import { createRoot } from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import App from './App.tsx'
import './index.css'
import '@xterm/xterm/css/xterm.css'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 60_000,
      retry: 1,
      refetchOnWindowFocus: false,
    },
  },
})

interface ReactQueryDevtoolsProps {
  initialIsOpen?: boolean
  buttonPosition?: 'top-left' | 'top-right' | 'bottom-left' | 'bottom-right'
  position?: 'top' | 'bottom' | 'left' | 'right'
  client?: unknown
  errorTypes?: unknown
  styleNonce?: string
  shadowDOMTarget?: ShadowRoot
  hideDisabledQueries?: boolean
  [key: string]: unknown
}

export function Devtools() {
  const [DevtoolsComponent, setDevtoolsComponent] =
    useState<ComponentType<ReactQueryDevtoolsProps> | null>(null)

  useEffect(() => {
    if (!import.meta.env.DEV) {
      return
    }

    let mounted = true

    void import('@tanstack/react-query-devtools')
      .then((module) => {
        if (mounted) {
          setDevtoolsComponent(
            () => module.ReactQueryDevtools as ComponentType<ReactQueryDevtoolsProps>
          )
        }
      })
      .catch(() => {
        // Devtools are optional; ignore loading failures in development.
      })

    return () => {
      mounted = false
    }
  }, [])

  if (!import.meta.env.DEV || !DevtoolsComponent) {
    return null
  }

  return <DevtoolsComponent initialIsOpen={false} buttonPosition="bottom-right" />
}

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <App />
      <Devtools />
    </QueryClientProvider>
  </StrictMode>
)

if (typeof window !== 'undefined' && import.meta.env.MODE !== 'test') {
  void import('@/lib/monitoring/registerWebVitals')
    .then(({ registerWebVitals }) => registerWebVitals())
    .catch((error) => {
      if (import.meta.env.DEV) {
        console.warn('Failed to initialise web vitals monitoring', error)
      }
    })
}
