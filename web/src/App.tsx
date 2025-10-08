import { useState, useEffect } from 'react'
import logo from './assets/logo.svg'
import './App.css'

function App() {
  const [isDark, setIsDark] = useState(false)

  useEffect(() => {
    // Check system preference
    const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches
    setIsDark(prefersDark)
    if (prefersDark) {
      document.documentElement.classList.add('dark')
    }
  }, [])

  const toggleTheme = () => {
    setIsDark(!isDark)
    document.documentElement.classList.toggle('dark')
  }

  return (
    <div className="flex min-h-screen flex-col items-center justify-center bg-background">
      <div className="text-center">
        {/* Theme Toggle */}
        <button
          onClick={toggleTheme}
          className="absolute right-8 top-8 rounded-lg bg-card p-3 shadow-md transition-colors hover:bg-muted"
          aria-label="Toggle theme"
        >
          {isDark ? (
            <svg
              className="h-5 w-5 text-foreground"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z"
              />
            </svg>
          ) : (
            <svg
              className="h-5 w-5 text-foreground"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z"
              />
            </svg>
          )}
        </button>

        <img src={logo} className="mx-auto mb-8 h-48 w-48" alt="ShellCN Logo" />
        <h1 className="mb-4 font-serif text-5xl font-bold text-foreground">ShellCN</h1>
        <p className="mb-8 text-xl text-muted-foreground">Enterprise Remote Access Platform</p>
        <div className="space-y-4">
          <p className="text-muted-foreground">
            Secure, scalable remote access to your infrastructure
          </p>
          <div className="flex justify-center gap-3">
            <span className="rounded-full bg-primary px-4 py-2 text-sm font-medium text-primary-foreground shadow-sm">
              SSH
            </span>
            <span className="rounded-full bg-secondary px-4 py-2 text-sm font-medium text-secondary-foreground shadow-sm">
              RDP
            </span>
            <span className="rounded-full bg-accent px-4 py-2 text-sm font-medium text-accent-foreground shadow-sm">
              VNC
            </span>
            <span className="rounded-full bg-chart-4 px-4 py-2 text-sm font-medium text-white shadow-sm">
              Kubernetes
            </span>
          </div>
        </div>
      </div>
    </div>
  )
}

export default App
