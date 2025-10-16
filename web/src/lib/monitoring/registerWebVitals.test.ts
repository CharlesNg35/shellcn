import { beforeEach, describe, expect, it, vi } from 'vitest'

type FakeMetric = {
  name: string
  value: number
  rating?: string
  navigationType?: string
  delta?: number
}

type MetricHandler = (metric: FakeMetric) => void

const metricHandlers: Partial<Record<string, MetricHandler>> = {}

vi.mock('web-vitals', () => ({
  onCLS: (cb: MetricHandler) => {
    metricHandlers.CLS = cb
  },
  onFID: (cb: MetricHandler) => {
    metricHandlers.FID = cb
  },
  onINP: (cb: MetricHandler) => {
    metricHandlers.INP = cb
  },
  onLCP: (cb: MetricHandler) => {
    metricHandlers.LCP = cb
  },
  onTTFB: (cb: MetricHandler) => {
    metricHandlers.TTFB = cb
  },
}))

vi.mock('@/lib/api/monitoring', () => ({
  monitoringApi: {
    submitWebVitals: vi.fn(() => Promise.resolve()),
  },
}))

describe('registerWebVitals', () => {
  beforeEach(() => {
    Object.keys(metricHandlers).forEach((key) => {
      delete metricHandlers[key]
    })
  })

  it('buffers metrics and flushes them through the monitoring API', async () => {
    vi.stubEnv('MODE', 'development')

    vi.useFakeTimers()
    const originalIdle = window.requestIdleCallback
    window.requestIdleCallback = (callback) =>
      window.setTimeout(
        () =>
          callback({
            didTimeout: false,
            timeRemaining: () => 50,
          }),
        0
      )

    vi.resetModules()
    const { registerWebVitals, flushWebVitals } = await import('./registerWebVitals')
    const { monitoringApi } = await import('@/lib/api/monitoring')
    const submitWebVitalsMock = monitoringApi.submitWebVitals as unknown as vi.Mock

    await registerWebVitals()
    metricHandlers.LCP?.({
      name: 'LCP',
      value: 1250,
      rating: 'good',
      navigationType: 'navigate',
      delta: 100,
    })

    await flushWebVitals()

    expect(submitWebVitalsMock).toHaveBeenCalledTimes(1)
    expect(submitWebVitalsMock).toHaveBeenCalledWith([
      expect.objectContaining({ metric: 'LCP', value: 1250, rating: 'good' }),
    ])

    window.requestIdleCallback = originalIdle
    vi.useRealTimers()
    vi.unstubAllEnvs()
  })
})
