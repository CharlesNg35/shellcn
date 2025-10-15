import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { useSshWorkspaceStore, resetSshWorkspaceStore } from '@/store/ssh-workspace-store'
import type { SftpListResult } from '@/types/sftp'

const SESSION_ID = 'session-1'

function createTransfer() {
  return {
    id: 'transfer-1',
    name: 'example.txt',
    path: 'example.txt',
    direction: 'upload',
    size: 128,
    uploaded: 0,
    status: 'pending' as const,
    startedAt: new Date('2024-01-01T00:00:00Z'),
  }
}

describe('ssh workspace store', () => {
  beforeEach(() => {
    resetSshWorkspaceStore()
  })

  afterEach(() => {
    resetSshWorkspaceStore()
  })

  it('ensures session with default browser tab', () => {
    const first = useSshWorkspaceStore.getState().ensureSession(SESSION_ID)
    const second = useSshWorkspaceStore.getState().ensureSession(SESSION_ID)

    expect(first).toBe(second)
    expect(first.tabs).toHaveLength(1)
    expect(first.tabs[0]).toMatchObject({
      type: 'browser',
      title: 'Files',
    })
    expect(useSshWorkspaceStore.getState().sessions[SESSION_ID]?.activeTabId).toBe(first.tabs[0].id)
  })

  it('normalizes browser path updates', () => {
    const store = useSshWorkspaceStore.getState()
    store.ensureSession(SESSION_ID)
    store.setBrowserPath(SESSION_ID, ' /var//log/ ')

    const session = useSshWorkspaceStore.getState().sessions[SESSION_ID]
    expect(session?.browserPath).toBe('var/log')
  })

  it('opens editors and avoids duplicates', () => {
    const store = useSshWorkspaceStore.getState()
    store.ensureSession(SESSION_ID)

    store.openEditor(SESSION_ID, '/etc/hosts', 'hosts')
    let session = useSshWorkspaceStore.getState().sessions[SESSION_ID]
    expect(session?.tabs).toHaveLength(2)
    const editorTab = session?.tabs.find((tab) => tab.type === 'editor')
    expect(editorTab?.path).toBe('etc/hosts')
    expect(session?.activeTabId).toBe(editorTab?.id)

    // Opening the same file focuses existing tab without duplication
    if (editorTab) {
      store.openEditor(SESSION_ID, '/etc/hosts', 'hosts')
    }
    session = useSshWorkspaceStore.getState().sessions[SESSION_ID]
    expect(session?.tabs).toHaveLength(2)
    expect(session?.activeTabId).toBe(editorTab?.id)
  })

  it('closes editor tabs while preserving browser tab', () => {
    const store = useSshWorkspaceStore.getState()
    store.ensureSession(SESSION_ID)
    store.openEditor(SESSION_ID, '/etc/hosts')
    let session = useSshWorkspaceStore.getState().sessions[SESSION_ID]
    const browserTabId = session?.tabs.find((tab) => tab.type === 'browser')?.id
    const editorTabId = session?.tabs.find((tab) => tab.type === 'editor')?.id

    if (editorTabId) {
      store.closeTab(SESSION_ID, editorTabId)
    }

    session = useSshWorkspaceStore.getState().sessions[SESSION_ID]
    expect(session?.tabs).toHaveLength(1)
    expect(session?.activeTabId).toBe(browserTabId)

    // Attempting to close the last tab should be a no-op
    if (browserTabId) {
      store.closeTab(SESSION_ID, browserTabId)
    }
    session = useSshWorkspaceStore.getState().sessions[SESSION_ID]
    expect(session?.tabs).toHaveLength(1)
  })

  it('tracks tab dirty state', () => {
    const store = useSshWorkspaceStore.getState()
    store.ensureSession(SESSION_ID)
    store.openEditor(SESSION_ID, '/etc/hosts')
    const editorTab = useSshWorkspaceStore
      .getState()
      .sessions[SESSION_ID]?.tabs.find((tab) => tab.type === 'editor')
    expect(editorTab?.dirty).toBeFalsy()

    if (editorTab) {
      store.setTabDirty(SESSION_ID, editorTab.id, true)
    }
    const updatedTab = useSshWorkspaceStore
      .getState()
      .sessions[SESSION_ID]?.tabs.find((tab) => tab.id === editorTab?.id)
    expect(updatedTab?.dirty).toBe(true)
  })

  it('manages transfer records and updates', () => {
    const store = useSshWorkspaceStore.getState()
    store.ensureSession(SESSION_ID)
    const transfer = createTransfer()
    store.upsertTransfer(SESSION_ID, transfer)

    let session = useSshWorkspaceStore.getState().sessions[SESSION_ID]
    expect(session?.transferOrder).toContain(transfer.id)
    expect(session?.transfers[transfer.id]).toMatchObject(transfer)

    store.updateTransfer(SESSION_ID, transfer.id, (existing) => ({
      ...existing,
      uploaded: 128,
      status: 'completed',
    }))

    session = useSshWorkspaceStore.getState().sessions[SESSION_ID]
    expect(session?.transfers[transfer.id]?.uploaded).toBe(128)
    expect(session?.transfers[transfer.id]?.status).toBe('completed')
  })

  it('clears completed or failed transfers while preserving in-progress ones', () => {
    const store = useSshWorkspaceStore.getState()
    store.ensureSession(SESSION_ID)
    store.upsertTransfer(SESSION_ID, { ...createTransfer(), id: 't1', status: 'completed' })
    store.upsertTransfer(SESSION_ID, {
      ...createTransfer(),
      id: 't2',
      status: 'failed',
      name: 'failed.txt',
    })
    store.upsertTransfer(SESSION_ID, {
      ...createTransfer(),
      id: 't3',
      status: 'uploading',
      name: 'active.txt',
    })

    store.clearCompletedTransfers(SESSION_ID)
    const session = useSshWorkspaceStore.getState().sessions[SESSION_ID]
    expect(session?.transferOrder).toEqual(['t3'])
    expect(Object.keys(session?.transfers ?? {})).toEqual(['t3'])
  })

  it('caches directory listings and clears them selectively', () => {
    const store = useSshWorkspaceStore.getState()
    store.ensureSession(SESSION_ID)
    const listing: SftpListResult = {
      path: '.',
      entries: [],
    }
    store.cacheDirectory(SESSION_ID, '.', listing)

    const cached = store.getCachedDirectory(SESSION_ID, '.')
    expect(cached).toBe(listing)

    store.clearDirectoryCache(SESSION_ID, '.')
    expect(store.getCachedDirectory(SESSION_ID, '.')).toBeUndefined()

    store.cacheDirectory(SESSION_ID, '.', listing)
    store.cacheDirectory(SESSION_ID, 'logs', { path: 'logs', entries: [] })
    store.clearDirectoryCache(SESSION_ID)
    expect(store.getCachedDirectory(SESSION_ID, '.')).toBeUndefined()
    expect(store.getCachedDirectory(SESSION_ID, 'logs')).toBeUndefined()
  })

  it('resets store state', () => {
    useSshWorkspaceStore.getState().ensureSession(SESSION_ID)
    expect(Object.keys(useSshWorkspaceStore.getState().sessions)).toHaveLength(1)

    resetSshWorkspaceStore()
    expect(Object.keys(useSshWorkspaceStore.getState().sessions)).toHaveLength(0)
  })
})
