import { afterEach, describe, expect, it } from 'vitest'
import {
  resetSshWorkspaceTabsStore,
  useSshWorkspaceTabsStore,
  type WorkspaceTabMeta,
} from '@/store/ssh-session-tabs-store'

const SESSION_ID = 'sess-123'
const CONNECTION_ID = 'conn-456'

describe('ssh workspace tabs store', () => {
  afterEach(() => {
    resetSshWorkspaceTabsStore()
  })

  it('opens session with default terminal tab', () => {
    const store = useSshWorkspaceTabsStore.getState()
    const session = store.openSession({
      sessionId: SESSION_ID,
      connectionId: CONNECTION_ID,
      connectionName: 'Primary host',
    })

    expect(session.sessionId).toBe(SESSION_ID)
    expect(session.tabs).toHaveLength(1)
    expect(session.tabs[0]).toMatchObject({
      type: 'terminal',
      title: 'Terminal',
      closable: false,
    })
    expect(useSshWorkspaceTabsStore.getState().activeSessionId).toBe(SESSION_ID)
  })

  it('ensures secondary tabs without duplication', () => {
    const store = useSshWorkspaceTabsStore.getState()
    store.openSession({
      sessionId: SESSION_ID,
      connectionId: CONNECTION_ID,
    })

    const first = store.ensureTab(SESSION_ID, 'sftp', { title: 'Files' })
    const second = store.ensureTab(SESSION_ID, 'sftp', { title: 'Ignored' })

    const tabs = useSshWorkspaceTabsStore.getState().sessions[SESSION_ID]?.tabs ?? []
    expect(tabs).toHaveLength(2)
    expect(first.id).toBe(second.id)
    expect(tabs[1]?.title).toBe('Files')
  })

  it('updates active tab and maintains ordering', () => {
    const store = useSshWorkspaceTabsStore.getState()
    store.openSession({
      sessionId: SESSION_ID,
      connectionId: CONNECTION_ID,
    })
    const sftpTab = store.ensureTab(SESSION_ID, 'sftp')

    store.setActiveTab(SESSION_ID, sftpTab.id)
    const session = useSshWorkspaceTabsStore.getState().sessions[SESSION_ID]

    expect(session?.activeTabId).toBe(sftpTab.id)
    expect(session?.lastFocusedAt).toBeGreaterThan(0)
  })

  it('updates tab metadata incrementally', () => {
    const store = useSshWorkspaceTabsStore.getState()
    store.openSession({
      sessionId: SESSION_ID,
      connectionId: CONNECTION_ID,
    })
    const sftpTab = store.ensureTab(SESSION_ID, 'sftp')
    const metaPatch: WorkspaceTabMeta = {
      badge: 'Read',
      ownerName: 'Alice',
    }

    store.updateTabMeta(SESSION_ID, sftpTab.id, metaPatch)
    const updated = useSshWorkspaceTabsStore
      .getState()
      .sessions[SESSION_ID]?.tabs.find((tab) => tab.id === sftpTab.id)

    expect(updated?.meta).toMatchObject(metaPatch)
    store.updateTabMeta(SESSION_ID, sftpTab.id, { badge: 'Write' })
    const patched = useSshWorkspaceTabsStore
      .getState()
      .sessions[SESSION_ID]?.tabs.find((tab) => tab.id === sftpTab.id)

    expect(patched?.meta).toMatchObject({ badge: 'Write', ownerName: 'Alice' })
  })

  it('removes closable tabs and keeps terminal active', () => {
    const store = useSshWorkspaceTabsStore.getState()
    store.openSession({
      sessionId: SESSION_ID,
      connectionId: CONNECTION_ID,
    })
    const sftpTab = store.ensureTab(SESSION_ID, 'sftp')
    expect(useSshWorkspaceTabsStore.getState().sessions[SESSION_ID]?.tabs).toHaveLength(2)

    store.closeTab(SESSION_ID, sftpTab.id)

    const session = useSshWorkspaceTabsStore.getState().sessions[SESSION_ID]
    expect(session?.tabs).toHaveLength(1)
    expect(session?.tabs[0]?.type).toBe('terminal')
    expect(session?.activeTabId).toBe(session?.tabs[0]?.id)
  })

  it('normalises layout columns and toggles fullscreen', () => {
    const store = useSshWorkspaceTabsStore.getState()
    store.openSession({
      sessionId: SESSION_ID,
      connectionId: CONNECTION_ID,
    })

    store.setLayoutColumns(SESSION_ID, 3)
    store.setLayoutColumns(SESSION_ID, 9)
    store.setFullscreen(SESSION_ID, true)

    const session = useSshWorkspaceTabsStore.getState().sessions[SESSION_ID]
    expect(session?.layoutColumns).toBe(5)
    expect(session?.isFullscreen).toBe(true)
    store.setFullscreen(SESSION_ID)
    expect(useSshWorkspaceTabsStore.getState().sessions[SESSION_ID]?.isFullscreen).toBe(false)
  })

  it('closes session and updates active selection', () => {
    const store = useSshWorkspaceTabsStore.getState()
    store.openSession({
      sessionId: SESSION_ID,
      connectionId: CONNECTION_ID,
    })
    store.openSession({
      sessionId: 'sess-999',
      connectionId: 'conn-999',
    })

    store.closeSession(SESSION_ID)
    const state = useSshWorkspaceTabsStore.getState()
    expect(state.sessions[SESSION_ID]).toBeUndefined()
    expect(state.orderedSessionIds).not.toContain(SESSION_ID)
    expect(state.activeSessionId).toBe('sess-999')
  })

  it('reorders tabs and persists order to localStorage', () => {
    const store = useSshWorkspaceTabsStore.getState()
    const session = store.openSession({
      sessionId: SESSION_ID,
      connectionId: CONNECTION_ID,
    })
    const terminalId = session.tabs[0]?.id ?? ''
    const sftpTab = store.ensureTab(SESSION_ID, 'sftp', { title: 'Files', closable: true })

    store.reorderTabs(SESSION_ID, [sftpTab.id, terminalId])

    const reordered = useSshWorkspaceTabsStore.getState().sessions[SESSION_ID]?.tabs ?? []
    expect(reordered.map((tab) => tab.id)).toEqual([sftpTab.id, terminalId])
    const storedOrder = window.localStorage.getItem('sshWorkspace.tabOrder.sess-123')
    expect(storedOrder).toBeTruthy()

    resetSshWorkspaceTabsStore()
    const reopened = useSshWorkspaceTabsStore.getState().openSession({
      sessionId: SESSION_ID,
      connectionId: CONNECTION_ID,
    })
    expect(reopened.tabs[0]?.id).toBe(terminalId)
    const withSftp = useSshWorkspaceTabsStore.getState().ensureTab(SESSION_ID, 'sftp', {
      title: 'Files',
      closable: true,
    })
    const reorderedAfterEnsure =
      useSshWorkspaceTabsStore.getState().sessions[SESSION_ID]?.tabs ?? []
    expect(reorderedAfterEnsure.map((tab) => tab.id)).toEqual([withSftp.id, terminalId])
  })

  it('clears stored tab order on session close', () => {
    const store = useSshWorkspaceTabsStore.getState()
    store.openSession({ sessionId: SESSION_ID, connectionId: CONNECTION_ID })
    const sftpTab = store.ensureTab(SESSION_ID, 'sftp', { title: 'Files', closable: true })
    const terminalId = `${SESSION_ID}:terminal`
    store.reorderTabs(SESSION_ID, [sftpTab.id, terminalId])

    expect(window.localStorage.getItem('sshWorkspace.tabOrder.sess-123')).toBeTruthy()

    store.closeSession(SESSION_ID)
    expect(window.localStorage.getItem('sshWorkspace.tabOrder.sess-123')).toBeNull()
  })
})
