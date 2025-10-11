import { toast as sonnerToast } from 'sonner'

/**
 * Toast notification utilities with consistent styling and behavior.
 * Built on top of Sonner for a unified notification experience.
 *
 * Usage:
 * ```tsx
 * import { toast } from '@/lib/utils/toast'
 *
 * toast.success('Connection established successfully')
 * toast.error('Failed to authenticate', { description: 'Invalid credentials' })
 * toast.loading('Connecting to server...', { id: 'connect-123' })
 * ```
 */

interface ToastOptions {
  description?: string
  action?: {
    label: string
    onClick: () => void
  }
  duration?: number
  id?: string | number
}

export const toast = {
  /**
   * Show success toast notification
   */
  success: (message: string, options?: ToastOptions) => {
    return sonnerToast.success(message, {
      description: options?.description,
      action: options?.action,
      duration: options?.duration ?? 4000,
      id: options?.id,
    })
  },

  /**
   * Show error toast notification
   */
  error: (message: string, options?: ToastOptions) => {
    return sonnerToast.error(message, {
      description: options?.description,
      action: options?.action,
      duration: options?.duration ?? 6000,
      id: options?.id,
    })
  },

  /**
   * Show info toast notification
   */
  info: (message: string, options?: ToastOptions) => {
    return sonnerToast.info(message, {
      description: options?.description,
      action: options?.action,
      duration: options?.duration ?? 4000,
      id: options?.id,
    })
  },

  /**
   * Show warning toast notification
   */
  warning: (message: string, options?: ToastOptions) => {
    return sonnerToast.warning(message, {
      description: options?.description,
      action: options?.action,
      duration: options?.duration ?? 5000,
      id: options?.id,
    })
  },

  /**
   * Show loading toast notification
   * Returns the toast ID for dismissal
   */
  loading: (message: string, options?: Omit<ToastOptions, 'duration'>) => {
    return sonnerToast.loading(message, {
      description: options?.description,
      id: options?.id,
    })
  },

  /**
   * Show promise-based toast notification
   * Automatically handles loading, success, and error states
   */
  promise: <T>(
    promise: Promise<T>,
    messages: {
      loading: string
      success: string | ((data: T) => string)
      error: string | ((error: Error) => string)
    }
  ) => {
    return sonnerToast.promise(promise, messages)
  },

  /**
   * Dismiss a toast by ID
   */
  dismiss: (toastId?: string | number) => {
    return sonnerToast.dismiss(toastId)
  },

  /**
   * Custom toast with full control
   */
  custom: (message: string, options?: ToastOptions) => {
    return sonnerToast(message, {
      description: options?.description,
      action: options?.action,
      duration: options?.duration ?? 4000,
      id: options?.id,
    })
  },
}
