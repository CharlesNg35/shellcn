import { useEffect, useRef } from 'react'
import 'asciinema-player/dist/bundle/asciinema-player.css'

interface RecordingPlayerProps {
  cast: string
  autoPlay?: boolean
}

export function RecordingPlayer({ cast, autoPlay = false }: RecordingPlayerProps) {
  const containerRef = useRef<HTMLDivElement | null>(null)

  useEffect(() => {
    const element = containerRef.current
    if (!element) {
      return
    }

    let disposed = false
    let player: { dispose?: () => void } | null = null

    element.innerHTML = ''

    import('asciinema-player').then(({ create }) => {
      if (disposed || !element.isConnected) {
        return
      }
      player = create(cast, element, {
        autoplay: autoPlay,
        preload: true,
        fit: 'width',
        theme: 'asciinema',
      })
    })

    return () => {
      disposed = true
      if (player && typeof player.dispose === 'function') {
        player.dispose()
      }
      element.innerHTML = ''
    }
  }, [autoPlay, cast])

  return (
    <div
      ref={containerRef}
      className="w-full overflow-hidden rounded-lg border border-border bg-black"
    />
  )
}
