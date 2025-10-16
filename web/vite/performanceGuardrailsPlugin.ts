import { promises as fs } from 'node:fs'
import path from 'node:path'
import type { Plugin } from 'vite'

interface ChunkAnalysis {
  file: string
  size: number
  isEntry: boolean
  isDynamicEntry: boolean
  facadeModuleId?: string
  imports: string[]
  dynamicImports: string[]
}

type BundleLike = Record<string, unknown>

interface BundleChunk {
  type: 'chunk'
  fileName: string
  code?: string
  isEntry?: boolean
  isDynamicEntry?: boolean
  facadeModuleId?: string | null
  imports?: string[]
  dynamicImports?: string[]
}

interface PerformanceGuardrailsOptions {
  maxSshChunkBytes?: number
  reportFilename?: string
}

const DEFAULT_MAX_SSH_CHUNK_BYTES = 300 * 1024
const DEFAULT_REPORT_FILENAME = 'bundle-report.json'

function formatBytes(bytes: number): string {
  if (bytes < 1024) {
    return `${bytes} B`
  }
  const units = ['KB', 'MB', 'GB']
  let size = bytes / 1024
  let unitIndex = 0
  while (size >= 1024 && unitIndex < units.length - 1) {
    size /= 1024
    unitIndex += 1
  }
  return `${size.toFixed(2)} ${units[unitIndex]}`
}

function analyseBundle(bundle: BundleLike): ChunkAnalysis[] {
  return Object.values(bundle)
    .filter((item): item is BundleChunk =>
      Boolean(item) && typeof item === 'object' && (item as BundleChunk).type === 'chunk'
    )
    .map((chunk) => ({
      file: chunk.fileName,
      size: Buffer.byteLength(chunk.code ?? ''),
      isEntry: chunk.isEntry ?? false,
      isDynamicEntry: chunk.isDynamicEntry ?? false,
      facadeModuleId: chunk.facadeModuleId ?? undefined,
      imports: chunk.imports ?? [],
      dynamicImports: chunk.dynamicImports ?? [],
    }))
}

export function performanceGuardrailsPlugin(
  options: PerformanceGuardrailsOptions = {}
): Plugin {
  const maxSshChunkBytes = options.maxSshChunkBytes ?? DEFAULT_MAX_SSH_CHUNK_BYTES
  const reportFilename = options.reportFilename ?? DEFAULT_REPORT_FILENAME
  let analysis: ChunkAnalysis[] = []

  return {
    name: 'performance-guardrails',
    apply: 'build',
    generateBundle(_, bundle) {
      analysis = analyseBundle(bundle)
      const sshChunk = analysis.find((chunk) =>
        chunk.facadeModuleId?.includes('src/pages/sessions/SshWorkspace')
      )

      if (!sshChunk) {
        this.warn('performance-guardrails: SSH workspace chunk not found during analysis')
        return
      }

      if (sshChunk.size > maxSshChunkBytes) {
        this.warn(
          `performance-guardrails: SSH workspace chunk ${sshChunk.file} is ${formatBytes(sshChunk.size)}, exceeding the ${formatBytes(maxSshChunkBytes)} guardrail`
        )
      }
    },
    async writeBundle(options) {
      if (!analysis.length) {
        return
      }
      const outDir = options.dir ?? (options.file ? path.dirname(options.file) : null)
      if (!outDir) {
        return
      }
      const reportPath = path.join(outDir, reportFilename)
      await fs.mkdir(path.dirname(reportPath), { recursive: true })
      await fs.writeFile(
        reportPath,
        JSON.stringify(
          {
            generatedAt: new Date().toISOString(),
            chunks: analysis,
          },
          null,
          2
        ),
        'utf8'
      )
    },
  }
}
