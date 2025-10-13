import { Children, useLayoutEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import { cn } from '@/lib/utils/cn'

const DEFAULT_DURATION = 220

interface CollapsibleProps {
  isOpen: boolean
  children: ReactNode
  className?: string
  duration?: number
}

export function Collapsible({
  isOpen,
  children,
  className,
  duration = DEFAULT_DURATION,
}: CollapsibleProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const [height, setHeight] = useState<string>(isOpen ? 'auto' : '0px')
  const childCount = useMemo(() => Children.count(children), [children])

  useLayoutEffect(() => {
    const el = containerRef.current
    if (!el) {
      return
    }

    const contentHeight = el.scrollHeight

    if (isOpen) {
      setHeight(`${contentHeight}px`)
      const timer = window.setTimeout(() => setHeight('auto'), duration)
      return () => window.clearTimeout(timer)
    }

    setHeight(`${contentHeight}px`)
    const raf = window.requestAnimationFrame(() => setHeight('0px'))
    return () => window.cancelAnimationFrame(raf)
  }, [isOpen, childCount, duration])

  return (
    <div
      ref={containerRef}
      className={cn('overflow-hidden', className)}
      style={{
        height: height === 'auto' ? 'auto' : height,
        transition:
          height === 'auto' ? 'none' : `height ${duration}ms cubic-bezier(0.33, 1, 0.68, 1)`,
      }}
      aria-hidden={!isOpen && height === '0px'}
    >
      <div className={cn('transition-opacity duration-200', isOpen ? 'opacity-100' : 'opacity-0')}>
        {children}
      </div>
    </div>
  )
}
