import { monitoringApi, type WebVitalMetricPayload } from '@/lib/api/monitoring'
import type { CLSMetric, FIDMetric, INPMetric, LCPMetric, TTFBMetric } from 'web-vitals'

type WebVitalMetric = CLSMetric | FIDMetric | INPMetric | LCPMetric | TTFBMetric

const FLUSH_DELAY_MS = 3_000
const MAX_BUFFER_SIZE = 20

let buffer: WebVitalMetricPayload[] = []
let flushTimer: number | null = null
let isRegistered = false

function scheduleFlush() {
  if (flushTimer != null) {
    return
  }
  flushTimer = window.setTimeout(() => {
    flushTimer = null
    void flushBuffer()
  }, FLUSH_DELAY_MS)
}

async function flushBuffer() {
  if (!buffer.length) {
    return
  }
  const payload = buffer
  buffer = []
  try {
    await monitoringApi.submitWebVitals(payload)
  } catch (error) {
    if (import.meta.env.DEV) {
      console.warn('Failed to submit web vitals', error)
    }
  }
}

function enqueueMetric(metric: WebVitalMetric) {
  if (!metric || typeof metric.value !== 'number') {
    return
  }

  buffer.push({
    metric: metric.name ?? 'unknown',
    value: metric.value,
    rating: metric.rating,
    navigation_type: metric.navigationType,
    delta: metric.delta,
  })

  if (buffer.length >= MAX_BUFFER_SIZE) {
    void flushBuffer()
    if (flushTimer !== null) {
      window.clearTimeout(flushTimer)
      flushTimer = null
    }
    return
  }

  scheduleFlush()
}

export async function registerWebVitals() {
  if (typeof window === 'undefined') {
    return
  }
  if (isRegistered) {
    return
  }
  if (import.meta.env.MODE === 'test') {
    return
  }
  isRegistered = true

  const [{ onCLS, onFID, onINP, onLCP, onTTFB }] = await Promise.all([import('web-vitals')])

  onCLS((metric) => enqueueMetric(metric))
  onFID((metric) => enqueueMetric(metric))
  onINP((metric) => enqueueMetric(metric))
  onLCP((metric) => enqueueMetric(metric))
  onTTFB((metric) => enqueueMetric(metric))
}

export function flushWebVitals(): Promise<void> {
  if (flushTimer !== null) {
    window.clearTimeout(flushTimer)
    flushTimer = null
  }
  return flushBuffer()
}
