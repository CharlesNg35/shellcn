import { describe, expect, it, vi } from 'vitest'
import { promises as fs } from 'node:fs'
import { tmpdir } from 'node:os'
import path from 'node:path'
import type { OutputBundle, OutputChunk } from 'vite'
import { performanceGuardrailsPlugin } from '../performanceGuardrailsPlugin'

function createBundle(codeSize: number, facadeModuleId: string): OutputBundle {
  const chunk: OutputChunk = {
    type: 'chunk',
    fileName: 'assets/ssh.js',
    code: 'x'.repeat(codeSize),
    imports: [],
    dynamicImports: [],
    isEntry: false,
    isDynamicEntry: true,
    facadeModuleId,
    modules: {},
  }
  return {
    [chunk.fileName]: chunk,
  }
}

describe('performanceGuardrailsPlugin', () => {
  it('emits a warning when SSH workspace chunk exceeds the guardrail', () => {
    const plugin = performanceGuardrailsPlugin({ maxSshChunkBytes: 100 })
    const warnings: string[] = []
    const context = {
      warn: (message: string) => {
        warnings.push(message)
      },
    }

    plugin.generateBundle!.call(context as never, {}, createBundle(150, '/src/pages/sessions/SshWorkspace.tsx'))

    expect(warnings).toHaveLength(1)
    expect(warnings[0]).toContain('SSH workspace chunk')
  })

  it('writes bundle analysis report on writeBundle', async () => {
    const plugin = performanceGuardrailsPlugin()
    const context = {
      warn: () => {},
    }

    const bundle = createBundle(50, '/src/pages/sessions/SshWorkspace.tsx')
    plugin.generateBundle!.call(context as never, {}, bundle)
    const outDir = await fs.mkdtemp(path.join(tmpdir(), 'bundle-test-'))
    try {
      await plugin.writeBundle!.call(context as never, { dir: outDir }, bundle)
      const reportPath = path.join(outDir, 'bundle-report.json')
      const stat = await fs.stat(reportPath)
      expect(stat.isFile()).toBe(true)
      const contents = await fs.readFile(reportPath, 'utf8')
      expect(contents).toContain('chunks')
    } finally {
      await fs.rm(outDir, { recursive: true, force: true })
    }
  })
})
