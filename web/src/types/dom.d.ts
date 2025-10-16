export {}

declare global {
  interface IdleDeadline {
    readonly didTimeout: boolean
    timeRemaining(): number
  }

  type IdleCallbackHandle = number
  type IdleCallback = (deadline: IdleDeadline) => void

  interface Window {
    requestIdleCallback?: (
      callback: IdleCallback,
      options?: { timeout?: number }
    ) => IdleCallbackHandle
    cancelIdleCallback?: (handle: IdleCallbackHandle) => void
  }

  type WindowWithIdleCallback = Window
}
