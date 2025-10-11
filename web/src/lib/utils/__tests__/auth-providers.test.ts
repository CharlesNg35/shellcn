import { parseAttributeMapping, serializeAttributeMapping } from '@/lib/utils/auth-providers'

describe('auth provider utils', () => {
  it('serializes attribute mapping', () => {
    const text = serializeAttributeMapping({ email: 'mail', username: 'uid' })
    expect(text).toContain('email=mail')
    expect(text).toContain('username=uid')
  })

  it('parses attribute mapping lines', () => {
    const mapping = parseAttributeMapping('email=mail\nusername=uid')
    expect(mapping).toEqual({ email: 'mail', username: 'uid' })
  })

  it('throws on invalid attribute mapping', () => {
    expect(() => parseAttributeMapping('invalid-line')).toThrow()
  })
})
