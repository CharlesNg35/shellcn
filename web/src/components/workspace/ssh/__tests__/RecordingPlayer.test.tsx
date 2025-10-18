import { render, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { RecordingPlayer } from '../RecordingPlayer'

const createMock = vi.fn()
const disposeMock = vi.fn()

vi.mock('asciinema-player', () => ({
  create: (...args: unknown[]) => {
    createMock(...args)
    return { dispose: disposeMock }
  },
}))

describe('RecordingPlayer', () => {
  beforeEach(() => {
    createMock.mockClear()
    disposeMock.mockClear()
  })

  it('initialises the player with provided options and disposes on unmount', async () => {
    const { unmount } = render(<RecordingPlayer cast="test.cast" autoPlay />)

    await waitFor(() => expect(createMock).toHaveBeenCalled())

    const [source, element, options] = createMock.mock.calls[0]
    expect(source).toBe('test.cast')
    expect(element).toBeInstanceOf(HTMLElement)
    expect(options).toMatchObject({
      autoplay: true,
      preload: true,
      fit: 'width',
      theme: 'asciinema',
    })

    unmount()
    expect(disposeMock).toHaveBeenCalledTimes(1)
  })

  it('recreates the player when the cast changes', async () => {
    const { rerender } = render(<RecordingPlayer cast="first.cast" />)
    await waitFor(() => expect(createMock).toHaveBeenCalledTimes(1))

    rerender(<RecordingPlayer cast="second.cast" />)
    await waitFor(() => expect(createMock).toHaveBeenCalledTimes(2))
    expect(disposeMock).toHaveBeenCalledTimes(1)

    const lastCall = createMock.mock.calls.at(-1)
    expect(lastCall?.[0]).toBe('second.cast')
  })
})
