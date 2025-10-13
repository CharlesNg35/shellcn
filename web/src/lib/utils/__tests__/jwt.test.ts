import { decodeJwt, getSessionIdFromToken } from '@/lib/utils/jwt'

function base64UrlEncode(value: object): string {
  const json = JSON.stringify(value)
  return Buffer.from(json).toString('base64url')
}

describe('jwt utils', () => {
  it('decodes token payload', () => {
    const payload = { sid: 'session-abc', uid: 'user-1' }
    const token = ['header', base64UrlEncode(payload), 'signature'].join('.')

    expect(decodeJwt(token)).toMatchObject(payload)
  })

  it('extracts session id from token', () => {
    const payload = { sid: 'session-xyz', jti: 'fallback' }
    const token = ['hdr', base64UrlEncode(payload), 'sig'].join('.')

    expect(getSessionIdFromToken(token)).toBe('session-xyz')
  })

  it('falls back to jti when sid missing', () => {
    const payload = { jti: 'fallback-id' }
    const token = ['hdr', base64UrlEncode(payload), 'sig'].join('.')

    expect(getSessionIdFromToken(token)).toBe('fallback-id')
  })
})
