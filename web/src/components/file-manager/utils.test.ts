import { describe, expect, it } from 'vitest'
import {
  displayPath,
  normalizePath,
  parentPath,
  resolveChildPath,
} from '@/components/file-manager/utils'

describe('file manager utils', () => {
  describe('normalizePath', () => {
    it('returns "." for empty or root-like values', () => {
      expect(normalizePath('')).toBe('.')
      expect(normalizePath('   ')).toBe('.')
      expect(normalizePath('/')).toBe('.')
      expect(normalizePath('.')).toBe('.')
    })

    it('trims whitespace and leading/trailing slashes', () => {
      expect(normalizePath('  //var///log/ ')).toBe('var///log')
      expect(normalizePath('/home/user')).toBe('home/user')
    })
  })

  describe('displayPath', () => {
    it('formats normalized paths with a leading slash', () => {
      expect(displayPath('.')).toBe('/')
      expect(displayPath('')).toBe('/')
      expect(displayPath('var/log')).toBe('/var/log')
      expect(displayPath('/already')).toBe('/already')
    })
  })

  describe('resolveChildPath', () => {
    it('appends child names to parent paths safely', () => {
      expect(resolveChildPath('.', 'config.yaml')).toBe('config.yaml')
      expect(resolveChildPath('var/log', 'app.log')).toBe('var/log/app.log')
    })

    it('drops leading slashes from child segments', () => {
      expect(resolveChildPath('var', '/log.txt')).toBe('var/log.txt')
    })
  })

  describe('parentPath', () => {
    it('returns dot when no parent exists', () => {
      expect(parentPath('.')).toBe('.')
      expect(parentPath('file.txt')).toBe('.')
    })

    it('returns parent directory for nested paths', () => {
      expect(parentPath('var/log/app')).toBe('var/log')
      expect(parentPath('/etc/nginx/nginx.conf')).toBe('/etc/nginx')
    })
  })
})
