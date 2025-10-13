import type { AuthTokens } from '@/types/auth'

type TokenListener = (tokens: AuthTokens | null) => void

const ACCESS_TOKEN_KEY = 'shellcn.access_token'
const REFRESH_TOKEN_KEY = 'shellcn.refresh_token'
const EXPIRES_AT_KEY = 'shellcn.access_expires_at'

const listeners = new Set<TokenListener>()

interface StorageLike {
  getItem(key: string): string | null
  setItem(key: string, value: string): void
  removeItem(key: string): void
}

const memoryStorage: StorageLike = (() => {
  const store = new Map<string, string>()
  return {
    getItem(key: string) {
      return store.get(key) ?? null
    },
    setItem(key: string, value: string) {
      store.set(key, value)
    },
    removeItem(key: string) {
      store.delete(key)
    },
  }
})()

function getStorage(): StorageLike {
  if (typeof window !== 'undefined' && window.localStorage) {
    return window.localStorage
  }

  return memoryStorage
}

export function getTokens(): AuthTokens | null {
  const storage = getStorage()
  const accessToken = storage.getItem(ACCESS_TOKEN_KEY)
  const refreshToken = storage.getItem(REFRESH_TOKEN_KEY)
  const expiresAt = storage.getItem(EXPIRES_AT_KEY)

  if (!accessToken || !refreshToken || !expiresAt) {
    return null
  }

  const expiresAtNumber = Number.parseInt(expiresAt, 10)

  if (Number.isNaN(expiresAtNumber)) {
    return null
  }

  const expiresIn = Math.max(0, Math.round((expiresAtNumber - Date.now()) / 1000))

  return {
    accessToken,
    refreshToken,
    expiresAt: expiresAtNumber,
    expiresIn,
  }
}

export function setTokens(tokens: AuthTokens | null) {
  const storage = getStorage()

  if (!tokens) {
    storage.removeItem(ACCESS_TOKEN_KEY)
    storage.removeItem(REFRESH_TOKEN_KEY)
    storage.removeItem(EXPIRES_AT_KEY)
    notifyListeners(null)
    return
  }

  storage.setItem(ACCESS_TOKEN_KEY, tokens.accessToken)
  storage.setItem(REFRESH_TOKEN_KEY, tokens.refreshToken)
  storage.setItem(EXPIRES_AT_KEY, tokens.expiresAt.toString())

  notifyListeners(tokens)
}

function notifyListeners(tokens: AuthTokens | null) {
  listeners.forEach((listener) => listener(tokens))
}

export function subscribeTokens(listener: TokenListener) {
  listeners.add(listener)
  return () => listeners.delete(listener)
}

export function clearTokens() {
  setTokens(null)
}
