'use client'

import {
  CircleCheckIcon,
  InfoIcon,
  Loader2Icon,
  OctagonXIcon,
  TriangleAlertIcon,
} from 'lucide-react'
import { Toaster as Sonner } from 'sonner'
import { useTheme } from '../theme/useTheme'

const Toaster = ({ ...props } = {}) => {
  const { theme } = useTheme()

  return (
    <Sonner
      theme={theme}
      className="toaster group"
      position="top-right"
      closeButton
      icons={{
        success: <CircleCheckIcon className="size-4 text-emerald-500 dark:text-emerald-400" />,
        info: <InfoIcon className="size-4 text-sky-500 dark:text-sky-400" />,
        warning: <TriangleAlertIcon className="size-4 text-amber-500 dark:text-amber-400" />,
        error: <OctagonXIcon className="size-4 text-red-500 dark:text-red-400" />,
        loading: (
          <Loader2Icon className="size-4 animate-spin text-indigo-500 dark:text-indigo-400" />
        ),
      }}
      style={
        {
          '--normal-bg': 'var(--popover)',
          '--normal-text': 'var(--popover-foreground)',
          '--normal-border': 'var(--border)',
          '--border-radius': 'var(--radius)',
        } as React.CSSProperties
      }
      {...props}
    />
  )
}

export { Toaster }
