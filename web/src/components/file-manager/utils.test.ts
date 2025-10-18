import { describe, expect, it } from 'vitest'
import {
  displayPath,
  normalizePath,
  parentPath,
  resolveChildPath,
} from '@/components/file-manager/utils'

describe('file manager utils', () => {
  describe('normalizePath', () => {
    it('returns empty string for empty values', () => {
      expect(normalizePath('')).toBe('')
      expect(normalizePath('   ')).toBe('')
    })

    it('handles root path', () => {
      expect(normalizePath('/')).toBe('/')
    })

    it('returns empty string for dot notation', () => {
      expect(normalizePath('.')).toBe('')
    })

    it('preserves absolute paths and normalizes multiple slashes', () => {
      expect(normalizePath('/home/user')).toBe('/home/user')
      expect(normalizePath('  //var///log/ ')).toBe('//var///log')
      expect(normalizePath('/etc/nginx/')).toBe('/etc/nginx')
    })
  })

  describe('displayPath', () => {
    it('formats empty or dot paths as root', () => {
      expect(displayPath('.')).toBe('/.')
      expect(displayPath('')).toBe('/')
    })

    it('preserves absolute paths', () => {
      expect(displayPath('/var/log')).toBe('/var/log')
      expect(displayPath('/already')).toBe('/already')
    })

    it('adds leading slash to relative paths', () => {
      expect(displayPath('var/log')).toBe('/var/log')
    })
  })

  describe('resolveChildPath', () => {
    it('appends child names to absolute parent paths', () => {
      expect(resolveChildPath('/home/user', 'config.yaml')).toBe('/home/user/config.yaml')
      expect(resolveChildPath('/var/log', 'app.log')).toBe('/var/log/app.log')
    })

    it('handles root and empty parent paths', () => {
      expect(resolveChildPath('/', 'file.txt')).toBe('/file.txt')
      expect(resolveChildPath('', 'file.txt')).toBe('/file.txt')
      expect(resolveChildPath('.', 'config.yaml')).toBe('/config.yaml')
    })

    it('drops leading slashes from child segments', () => {
      expect(resolveChildPath('/var', '/log.txt')).toBe('/var/log.txt')
    })
  })

  describe('parentPath', () => {
    it('returns root when no parent exists', () => {
      expect(parentPath('/')).toBe('/')
      expect(parentPath('')).toBe('/')
      expect(parentPath('.')).toBe('/')
    })

    it('returns parent directory for absolute paths', () => {
      expect(parentPath('/var/log/app')).toBe('/var/log')
      expect(parentPath('/etc/nginx/nginx.conf')).toBe('/etc/nginx')
      expect(parentPath('/home')).toBe('/')
    })
  })
})
