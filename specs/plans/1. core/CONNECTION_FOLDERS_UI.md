# Connection Folders UI Specification

**Version:** 1.3
**Date:** 2025-10-10
**Status:** Draft
**Default Mode:** Flat Structure (Single Level)
**UI Terminology:** "Resources" (user-facing) / "Protocols" (technical)
**Form Strategy:** Basic fields first, protocol-specific fields later

---

## 1. Overview

This specification defines the UI/UX implementation for Connection Folder management in ShellCN. Folders provide organization of connections with **configurable nesting depth** (default: single level, flat structure). The folder system is multi-tenant aware, supporting both personal and team-scoped folders.

### 1.0 Folder Depth Configuration

**Default Behavior: Single Level (Flat)**
- By default, folders are **one level deep** (no subfolders)
- Keeps the UI simple and prevents over-complication
- Most users only need basic categorization

**Configuration Option:**
```typescript
// web/src/config/folders.ts
export const FOLDER_CONFIG = {
  // Maximum folder nesting depth
  // 1 = flat (no subfolders, default)
  // 2 = folders can have subfolders
  // n = unlimited depth
  maxDepth: 1,

  // Whether to show "Create Subfolder" option
  allowSubfolders: false,

  // Whether to show parent folder selector in create/edit
  allowParentSelection: false,
}
```

**Backend Support:**
- Backend **already supports unlimited nesting** (via `parent_id` field)
- No backend changes needed to enable/disable nesting
- Simply control UI behavior via frontend configuration

**Migration Path:**
- Start with `maxDepth: 1` (flat structure) for simplicity
- If users request hierarchical folders, change to `maxDepth: 2` or higher
- Existing folders automatically support deeper nesting when enabled

### 1.1 Current State

**Backend Implementation (Complete):**
- ✅ API endpoints implemented (`/api/connection-folders/*`)
- ✅ Service layer with CRUD operations (`ConnectionFolderService`)
- ✅ Handler layer with permission checks (`ConnectionFolderHandler`)
- ✅ Permission system: `connection.folder.view` and `connection.folder.manage`
- ✅ Hierarchical tree structure with parent-child relationships
- ✅ Connection count aggregation per folder
- ✅ Team/personal scope filtering
- ✅ Soft delete with child reassignment

**Frontend Implementation (Partial):**
- ✅ Folder tree display component (`FolderTree.tsx`)
- ✅ Folder sidebar with collapse/expand (`FolderSidebar.tsx`)
- ✅ API client functions (`connection-folders.ts`)
- ✅ Folder filtering on Connections page
- ❌ **Missing: Folder creation UI**
- ❌ **Missing: Folder management UI (edit, delete, move)**
- ❌ **Missing: Empty state with create folder CTA**
- ❌ **Missing: Drag-and-drop folder organization**

### 1.2 Problem Statement

Users cannot create or manage folders from the UI. The `FolderSidebar` component returns `null` when `folders.length === 0`, making it impossible for users to create their first folder. There is no accessible UI for:
- Creating new folders
- Editing folder metadata (name, description, icon, color)
- Moving folders within the hierarchy
- Deleting folders
- Moving connections between folders

---

## 2. API Reference

### 2.1 Available Endpoints

Based on `/home/ubuntu/projects/charlesng/shellcn/internal/api/routes_connection_folders.go`:

```go
GET    /api/connection-folders/tree      // Permission: connection.folder.view
POST   /api/connection-folders            // Permission: connection.folder.manage
PATCH  /api/connection-folders/:id        // Permission: connection.folder.manage
DELETE /api/connection-folders/:id        // Permission: connection.folder.manage
```

### 2.2 Data Structures

**ConnectionFolderNode (Tree Response):**
```typescript
interface ConnectionFolderNode {
  folder: ConnectionFolderDTO
  connection_count: number
  children?: ConnectionFolderNode[]
}

interface ConnectionFolderDTO {
  id: string
  name: string
  slug: string
  description: string
  icon: string
  color: string
  parent_id: string | null
  team_id: string | null
  metadata: Record<string, unknown>
}
```

**ConnectionFolderInput (Create/Update Payload):**
```typescript
interface ConnectionFolderInput {
  name: string              // Required for create
  description?: string
  icon?: string
  color?: string
  parent_id?: string | null // For hierarchical placement
  team_id?: string | null   // For team ownership
  metadata?: Record<string, unknown>
  ordering?: number         // For custom sort order
}
```

### 2.3 Backend Behaviors

From `/home/ubuntu/projects/charlesng/shellcn/internal/services/connection_folder_service.go`:

1. **Folder Creation (lines 164-204):**
   - Auto-generates slug from name (lowercase, spaces to dashes)
   - Sets `owner_user_id` to current user
   - Validates `connection.folder.manage` permission
   - Supports parent folder assignment
   - Supports team assignment

2. **Folder Update (lines 206-261):**
   - Allows changing name, description, icon, color
   - Allows reparenting (moving in hierarchy)
   - Auto-updates slug if name changes
   - Requires `connection.folder.manage` permission

3. **Folder Deletion (lines 263-299):**
   - **Transaction-safe** deletion with child reassignment
   - Child folders reassigned to parent (or root if no parent)
   - Connections reassigned to parent folder (or unassigned)
   - Prevents orphaned data

4. **Tree Building (lines 62-162):**
   - Aggregates connection counts recursively
   - Filters by user permissions (personal + team memberships)
   - Supports team filtering (`personal`, team ID, or all)
   - Always includes "Unassigned" pseudo-folder when connections exist without folder

---

## 3. UI Components Architecture

### 3.1 Component Hierarchy

```
Connections Page (pages/connections/Connections.tsx)
├── FolderSidebar (components/connections/FolderSidebar.tsx) [✅ EXISTS]
│   ├── FolderTree (components/connections/FolderTree.tsx) [✅ EXISTS]
│   └── CreateFolderButton [❌ NEW]
│       └── FolderFormModal [❌ NEW]
├── FolderManagementMenu [❌ NEW]
│   ├── Edit Option → FolderFormModal
│   ├── Move Option → MoveFolderModal
│   └── Delete Option → DeleteConfirmModal
└── ConnectionCard [✅ EXISTS]
    └── Move to Folder Option [❌ NEW]
```

### 3.2 New Components Required

#### 3.2.1 FolderFormModal
**Purpose:** Create or edit folder
**Location:** `/web/src/components/connections/FolderFormModal.tsx`

**Features:**
- Modal form with name, description, icon, color fields
- **Parent folder selection** (conditionally shown based on `FOLDER_CONFIG.allowParentSelection`)
- Team assignment (if user has team permissions)
- Icon picker (predefined set)
- Color picker (predefined palette)
- Validation: name required, max lengths
- Mode: create vs edit

**Props:**
```typescript
interface FolderFormModalProps {
  open: boolean
  onClose: () => void
  mode: 'create' | 'edit'
  folder?: ConnectionFolderDTO  // For edit mode
  parentFolder?: ConnectionFolderDTO  // For create with parent (if hierarchical enabled)
  teamId?: string | null  // Pre-selected team
  availableFolders?: ConnectionFolderNode[]  // For parent selection (only if hierarchical)
  onSuccess: (folder: ConnectionFolderDTO) => void
}
```

**Flat Mode Behavior (maxDepth: 1):**
- Parent folder selector is **hidden**
- Always creates folders at root level (`parent_id: null`)
- Simplified UI with only name, description, icon, color, team

#### 3.2.2 DeleteFolderConfirmModal
**Purpose:** Confirm folder deletion with impact preview
**Location:** `/web/src/components/connections/DeleteFolderConfirmModal.tsx`

**Features:**
- Shows folder name and description
- Displays impact preview:
  - **Number of child folders** (only shown if hierarchical mode enabled)
  - Number of connections (will be unassigned)
- Warning message about action
- Confirmation input (type folder name to confirm)
- Permission check before showing

**Props:**
```typescript
interface DeleteFolderConfirmModalProps {
  open: boolean
  onClose: () => void
  folder: ConnectionFolderNode  // Includes count for preview
  onConfirm: () => Promise<void>
}
```

**Flat Mode Behavior (maxDepth: 1):**
- Impact preview simplified: only shows connection count
- Message: "X connections will be unassigned" (no subfolder mention)

#### 3.2.3 MoveFolderModal
**Purpose:** Change folder parent (reparent)
**Location:** `/web/src/components/connections/MoveFolderModal.tsx`

**⚠️ ONLY NEEDED IN HIERARCHICAL MODE**

**Features:**
- Current location display
- Hierarchical folder picker (exclude self and descendants)
- "Move to Root" option
- Visual preview of new location
- Validation: prevent circular references

**Props:**
```typescript
interface MoveFolderModalProps {
  open: boolean
  onClose: () => void
  folder: ConnectionFolderDTO
  availableFolders: ConnectionFolderNode[]
  onSuccess: () => void
}
```

**Flat Mode:** This component is **NOT implemented** when `maxDepth: 1`

#### 3.2.4 FolderContextMenu
**Purpose:** Right-click or actions menu for folders
**Location:** `/web/src/components/connections/FolderContextMenu.tsx`

**Features:**
- **Create subfolder** (conditionally shown based on `FOLDER_CONFIG.allowSubfolders`)
- Edit folder
- **Move folder** (conditionally shown based on `FOLDER_CONFIG.allowParentSelection`)
- Delete folder
- Permission-based visibility (`connection.folder.manage`)

**Props:**
```typescript
interface FolderContextMenuProps {
  folder: ConnectionFolderDTO
  onCreateSubfolder?: (parentId: string) => void  // Optional in flat mode
  onEdit: (folder: ConnectionFolderDTO) => void
  onMove?: (folder: ConnectionFolderDTO) => void  // Optional in flat mode
  onDelete: (folder: ConnectionFolderDTO) => void
}
```

**Flat Mode Behavior (maxDepth: 1):**
- Menu shows only: **Edit** and **Delete**
- "Create Subfolder" is **hidden**
- "Move Folder" is **hidden**

#### 3.2.5 ResourceSelectionModal
**Purpose:** First step in connection creation - select protocol/resource type
**Location:** `/web/src/components/connections/ResourceSelectionModal.tsx`

**Features:**
- Two-step connection creation wizard (step 1)
- Shows available protocols from `useAvailableProtocols()` hook
- Groups protocols by category (terminal, container, database, cloud, etc.)
- Filters by user permissions (shows only protocols user can use)
- Search/filter functionality
- Visual protocol cards with icons and descriptions

**Props:**
```typescript
interface ResourceSelectionModalProps {
  open: boolean
  onClose: () => void
  onSelectProtocol: (protocolId: string) => void
}
```

**Integration with Existing API:**
- Uses existing `GET /api/protocols/available` endpoint
- Filters protocols where `available: true` and user has `{protocol}.connect` permission
- Categories from protocol metadata: `category` field (terminal, container, database, etc.)

**UI Labels:**
- Modal title: "Select Resource Type"
- Description: "Choose the type of resource to connect to"
- User-facing term: "Resource" (instead of "Protocol" or "Driver")

#### 3.2.6 ConnectionFormModal
**Purpose:** Second step in connection creation - configure basic connection details
**Location:** `/web/src/components/connections/ConnectionFormModal.tsx`

**Features (Initial Implementation):**
- Basic connection entity fields only
- Name and description inputs
- Folder assignment dropdown
- Team assignment (if user has teams)
- Protocol ID from previous step (hidden)
- Form validation (name required, max lengths)

**Props:**
```typescript
interface ConnectionFormModalProps {
  open: boolean
  onClose: () => void
  protocolId: string  // Selected from ResourceSelectionModal
  onSuccess: (connection: ConnectionRecord) => void
}
```

**Future Enhancement:**
- Dynamic protocol-specific fields based on `protocolId`
- Field schema from protocol driver metadata
- Advanced settings (targets, identities, metadata)
- Connection testing/preview

**Form Fields (MVP):**
```typescript
{
  name: string              // Required, max 255 chars
  description?: string      // Optional, max 1000 chars
  protocol_id: string       // From ResourceSelectionModal (hidden)
  folder_id?: string        // Selected folder (optional)
  team_id?: string          // Current team context (optional)
}
```

---

## 4. User Flows

### 4.1 Create First Folder (Empty State)

**Current Behavior:**
- `FolderSidebar` returns `null` when `folders.length === 0`
- No UI visible to create folders

**Proposed Flow:**

1. User lands on Connections page with no folders
2. Instead of hiding sidebar, show empty state card:
   ```
   ┌─────────────────────────────┐
   │  📁 Folders                 │
   │                             │
   │  No folders yet             │
   │                             │
   │  Organize your connections  │
   │  into folders for easier    │
   │  navigation.                │
   │                             │
   │  [+ Create Folder]          │
   └─────────────────────────────┘
   ```
3. Click "Create Folder" → Opens `FolderFormModal` in create mode
4. User fills form (name required, others optional)
5. On submit:
   - POST `/api/connection-folders`
   - Refresh folder tree
   - Select newly created folder
   - Show success toast

**Code Changes Required:**

`FolderSidebar.tsx` (lines 22-24):
```typescript
// Current (REMOVE):
if (folders.length === 0) {
  return null
}

// New (REPLACE WITH):
const isEmpty = folders.length === 0

return (
  <div className={cn('shrink-0 transition-all', collapsed ? 'w-16' : 'w-72')}>
    {isEmpty ? (
      <EmptyFolderState onCreateFolder={handleCreateFolder} />
    ) : (
      // ... existing folder tree rendering
    )}
  </div>
)
```

### 4.2 Create Subfolder

**⚠️ ONLY APPLICABLE IN HIERARCHICAL MODE (`allowSubfolders: true`)**

**Flat Mode (maxDepth: 1):** This flow is **disabled**. All folders are created at root level.

**Hierarchical Mode Flow:**

1. User right-clicks folder OR clicks folder actions menu (⋮)
2. Menu shows "Create Subfolder" option
3. Click → Opens `FolderFormModal` with `parentFolder` pre-filled
4. Form shows:
   - Parent: "Production > Databases" (read-only, shown as breadcrumb)
   - Name: [input]
   - Description, icon, color: [optional]
5. On submit:
   - POST `/api/connection-folders` with `parent_id`
   - Refresh tree
   - Auto-expand parent to show new subfolder

### 4.3 Edit Folder

**Flow:**

1. User clicks folder actions menu → "Edit"
2. Opens `FolderFormModal` in edit mode with existing data
3. User modifies name, description, icon, or color
4. On submit:
   - PATCH `/api/connection-folders/:id`
   - Refresh tree
   - Maintain current selection

**Note:** Moving to different parent uses separate "Move" action (4.4)

### 4.4 Move Folder

**⚠️ ONLY APPLICABLE IN HIERARCHICAL MODE (`allowParentSelection: true`)**

**Flat Mode (maxDepth: 1):** This flow is **disabled**. Folders cannot be moved since all exist at root level.

**Hierarchical Mode Flow:**

1. User clicks folder actions menu → "Move"
2. Opens `MoveFolderModal`
3. Shows current location: "Personal > Projects > Backend"
4. Shows hierarchical picker with:
   - "Move to Root" option
   - All folders except:
     - Current folder (can't move to self)
     - Descendants (prevent circular reference)
5. User selects new parent
6. Shows preview: "Will move to: Production > Services"
7. On confirm:
   - PATCH `/api/connection-folders/:id` with new `parent_id`
   - Refresh tree
   - Auto-expand new parent

### 4.5 Delete Folder

**Flow:**

1. User clicks folder actions menu → "Delete"
2. Opens `DeleteFolderConfirmModal`
3. Shows impact based on mode:

**Flat Mode (maxDepth: 1):**
   ```
   Delete folder "Production Servers"?

   Impact:
   • 12 connections will be unassigned

   Type "Production Servers" to confirm
   ```

**Hierarchical Mode:**
   ```
   Delete folder "Backend Projects"?

   Impact:
   • 3 subfolders will be moved to parent
   • 12 connections will be unassigned

   Type "Backend Projects" to confirm
   ```

4. User types folder name to confirm
5. On confirm:
   - DELETE `/api/connection-folders/:id`
   - Refresh tree
   - Clear folder filter (show all)
   - Show success toast

### 4.6 Create New Connection

**Entry Point:** "New Connection" button on Connections page

**Permission Required:** `connection.manage`

**Flow:**

1. User clicks "New Connection" button (shown only with `connection.manage` permission)
2. Opens **Resource Selection Modal** (step 1 of 2)
3. Shows available protocols/drivers grouped by category:
   ```
   ┌──── Select Resource Type ─────────────┐
   │                                       │
   │  Choose the type of resource to      │
   │  connect to:                         │
   │                                       │
   │  🖥️  Terminal & Remote Access         │
   │    • SSH - Secure Shell              │
   │    • RDP - Remote Desktop            │
   │    • VNC - Virtual Network Computing │
   │                                       │
   │  🐳  Containers & Orchestration       │
   │    • Docker - Container Platform     │
   │    • Kubernetes - Container Orchestr.│
   │                                       │
   │  💾  Databases                        │
   │    • PostgreSQL                      │
   │    • MySQL                           │
   │    • MongoDB                         │
   │                                       │
   │  ☁️  Cloud Platforms                  │
   │    • AWS - Amazon Web Services       │
   │    • Azure - Microsoft Cloud         │
   │                                       │
   │             [Cancel]  [Next →]       │
   └───────────────────────────────────────┘
   ```
4. User selects a protocol (e.g., "SSH")
5. Click "Next" → Opens **Connection Form** (step 2 of 2)
6. **Initial Implementation - Basic Fields Only:**
   - Name (required)
   - Description (optional)
   - **Folder assignment** (dropdown with available folders)
   - Team assignment (if applicable)
   - Protocol ID (hidden, from step 1 selection)
7. **Future Enhancement - Protocol-Specific Fields:**
   - Dynamic form fields based on selected protocol
   - SSH: host, port, username, key selection
   - Docker: socket path, TLS options
   - Kubernetes: context, namespace, kubeconfig
   - Database: connection string, database name, etc.
   - _Note: Protocol-specific fields will be implemented in a later phase_
8. On submit:
   - POST `/api/connections` with:
     ```json
     {
       "name": "Production Server",
       "description": "Main production SSH server",
       "protocol_id": "ssh",
       "folder_id": "folder_abc123",
       "team_id": "team_xyz789"
       // Protocol-specific settings to be added later
     }
     ```
   - Redirect to connection detail or connections list

**Permission Check:**
```typescript
import { PERMISSIONS } from '@/constants/permissions'

<PermissionGuard permission={PERMISSIONS.CONNECTION.MANAGE}>
  <Button asChild>
    <Link to="/connections?create=true">
      <Plus className="mr-1 h-4 w-4" />
      New Connection
    </Link>
  </Button>
</PermissionGuard>
```

**UI Label Convention:**
- In UI, refer to protocols as **"Resources"** for user-friendly terminology
- Technical documentation uses "protocols" or "drivers"
- Example: "Select Resource Type" instead of "Select Protocol"

### 4.7 Move Connection to Folder

**Proposed Enhancement (Optional):**

Add to `ConnectionCard` actions menu:
```typescript
<DropdownMenuItem onClick={() => setShowMoveFolderPicker(true)}>
  <FolderInput className="h-4 w-4" />
  Move to Folder
</DropdownMenuItem>
```

Opens folder picker to reassign connection's `folder_id`.

---

## 5. Permission Integration

### 5.1 Permission Checks

From `web/src/constants/permissions.ts`:
```typescript
export const PERMISSIONS = {
  CONNECTION: {
    VIEW: 'connection.view',
    MANAGE: 'connection.manage',
    FOLDER: {
      VIEW: 'connection.folder.view',
      MANAGE: 'connection.folder.manage'
    }
  }
}
```

### 5.2 UI Permission Gates

**View Folders:**
- Always visible if user has `connection.folder.view`
- Empty state with create button requires `connection.folder.manage`

**Create/Edit/Delete Folders:**
```typescript
import { usePermissions } from '@/hooks/usePermissions'
import { PERMISSIONS } from '@/constants/permissions'

const { hasPermission } = usePermissions()
const canManageFolders = hasPermission(PERMISSIONS.CONNECTION.FOLDER.MANAGE)

// Show create button only if has permission
{canManageFolders && (
  <Button onClick={handleCreateFolder}>
    <Plus className="h-4 w-4 mr-2" />
    Create Folder
  </Button>
)}
```

**Folder Actions Menu:**
```typescript
<PermissionGuard permission={PERMISSIONS.CONNECTION.FOLDER.MANAGE}>
  <FolderContextMenu
    folder={folder}
    onEdit={handleEdit}
    onMove={handleMove}
    onDelete={handleDelete}
  />
</PermissionGuard>
```

---

## 6. Icon and Color System

### 6.1 Folder Icons

Predefined icon set using Lucide React icons:

```typescript
import {
  Folder,
  FolderOpen,
  FolderClosed,
  FolderTree,
  FolderKanban,
  FolderGit,
  Database,
  Server,
  Cloud,
  Package,
  Layers,
  Box,
} from 'lucide-react'

const FOLDER_ICONS = [
  { id: 'folder', icon: Folder, label: 'Folder' },
  { id: 'folder-open', icon: FolderOpen, label: 'Open Folder' },
  { id: 'database', icon: Database, label: 'Database' },
  { id: 'server', icon: Server, label: 'Server' },
  { id: 'cloud', icon: Cloud, label: 'Cloud' },
  { id: 'package', icon: Package, label: 'Package' },
  { id: 'layers', icon: Layers, label: 'Layers' },
  { id: 'box', icon: Box, label: 'Box' },
]
```

### 6.2 Folder Colors

Predefined color palette (Tailwind compatible):

```typescript
const FOLDER_COLORS = [
  { id: 'blue', value: '#3b82f6', label: 'Blue' },
  { id: 'green', value: '#10b981', label: 'Green' },
  { id: 'red', value: '#ef4444', label: 'Red' },
  { id: 'yellow', value: '#f59e0b', label: 'Yellow' },
  { id: 'purple', value: '#8b5cf6', label: 'Purple' },
  { id: 'pink', value: '#ec4899', label: 'Pink' },
  { id: 'indigo', value: '#6366f1', label: 'Indigo' },
  { id: 'gray', value: '#6b7280', label: 'Gray' },
]
```

**Usage in FolderTree:**
```typescript
<div
  className="flex items-center gap-2"
  style={{ color: folder.color || 'inherit' }}
>
  <FolderIcon className="h-4 w-4" style={{ color: folder.color }} />
  <span>{folder.name}</span>
</div>
```

---

## 7. Team Integration

### 7.1 Team Scope Awareness

Folders are created with team context from active team filter:

```typescript
// In Connections.tsx
const teamParam = searchParams.get('team') ?? 'all'
const teamFilterValue =
  teamParam === 'all' ? undefined :
  teamParam === 'personal' ? 'personal' :
  teamParam

// Pass to FolderFormModal
<FolderFormModal
  teamId={teamFilterValue}
  // ... other props
/>
```

**Backend handles team assignment:**
- If `team_id` is null → Personal folder
- If `team_id` is set → Team folder
- Visibility filtered by user's team memberships

### 7.2 Team Switching Behavior

When user switches team filter:
1. Folder tree refreshes with new team scope
2. Connection counts recalculated for new scope
3. Active folder selection cleared (reset to "All Folders")

---

## 8. API Client Implementation

### 8.1 Existing API Functions

From `web/src/lib/api/connection-folders.ts`:

```typescript
export async function fetchConnectionFolderTree(teamId?: string) {
  const params = teamId ? `?team_id=${teamId}` : ''
  const response = await fetch(`/api/connection-folders/tree${params}`)
  return response.json()
}

export async function createConnectionFolder(
  input: ConnectionFolderInput
): Promise<ApiResponse<ConnectionFolderDTO>> {
  const response = await fetch('/api/connection-folders', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(input),
  })
  return response.json()
}

export async function updateConnectionFolder(
  id: string,
  input: ConnectionFolderInput
): Promise<ApiResponse<ConnectionFolderDTO>> {
  const response = await fetch(`/api/connection-folders/${id}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(input),
  })
  return response.json()
}

export async function deleteConnectionFolder(
  id: string
): Promise<ApiResponse<{ deleted: boolean }>> {
  const response = await fetch(`/api/connection-folders/${id}`, {
    method: 'DELETE',
  })
  return response.json()
}
```

### 8.2 React Query Hooks

Create custom hooks for mutations:

```typescript
// web/src/hooks/useConnectionFolderMutations.ts

import { useMutation, useQueryClient } from '@tanstack/react-query'
import {
  createConnectionFolder,
  updateConnectionFolder,
  deleteConnectionFolder
} from '@/lib/api/connection-folders'

export function useCreateFolder() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: createConnectionFolder,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['connection-folders'] })
    },
  })
}

export function useUpdateFolder() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({ id, input }: { id: string; input: ConnectionFolderInput }) =>
      updateConnectionFolder(id, input),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['connection-folders'] })
    },
  })
}

export function useDeleteFolder() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: deleteConnectionFolder,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['connection-folders'] })
    },
  })
}
```

---

## 9. Implementation Plan

### Phase 1: Core Folder Management - Flat Mode (MVP)

**Priority: High**
**Default Configuration: `maxDepth: 1` (flat structure)**

0. **Create Folder Configuration** (30 min)
   - [ ] Create `web/src/config/folders.ts` with `FOLDER_CONFIG`
   - [ ] Set default `maxDepth: 1`, `allowSubfolders: false`, `allowParentSelection: false`

1. **Fix Empty State** (2 hours)
   - [ ] Update `FolderSidebar.tsx` to show empty state instead of `null`
   - [ ] Create `EmptyFolderState` component with CTA
   - [ ] Add permission check for create button

2. **Create FolderFormModal - Flat Mode** (3 hours)
   - [ ] Build modal form component
   - [ ] Implement create mode with name, description, icon, color
   - [ ] **Skip parent folder selection** (always `parent_id: null`)
   - [ ] Add team assignment (if applicable)
   - [ ] Form validation and error handling
   - [ ] Integrate with `useCreateFolder` mutation

3. **Edit Folder** (2 hours)
   - [ ] Add edit mode to `FolderFormModal`
   - [ ] Add actions menu to folder items (⋮ button)
   - [ ] **Menu shows only Edit + Delete** (no subfolder/move options)
   - [ ] Integrate with `useUpdateFolder` mutation

4. **Delete Folder - Flat Mode** (2 hours)
   - [ ] Create `DeleteFolderConfirmModal`
   - [ ] Show impact preview: **connections only** (no children mention)
   - [ ] Confirmation input validation
   - [ ] Integrate with `useDeleteFolder` mutation

5. **Icon and Color Pickers** (3 hours)
   - [ ] Icon picker component with predefined set
   - [ ] Color picker with palette
   - [ ] Preview in folder tree

6. **Connection Creation - Resource Selection** (3 hours)
   - [ ] Create `ResourceSelectionModal` component
   - [ ] Integrate with `useAvailableProtocols()` hook
   - [ ] Group protocols by category
   - [ ] Filter by user permissions
   - [ ] Search/filter functionality

7. **Connection Creation - Basic Form** (3 hours)
   - [ ] Create `ConnectionFormModal` component
   - [ ] Basic fields: name, description, folder, team
   - [ ] Folder dropdown with available folders
   - [ ] Team assignment from context
   - [ ] Form validation and error handling
   - [ ] Integrate with `useCreateConnection` mutation
   - [ ] Add info message about protocol-specific fields (future)

**Total Phase 1: ~18.5 hours**

### Phase 2: Hierarchical Mode (Optional Future Enhancement)

**Priority: Medium (only if users request nested folders)**
**Configuration Change: Set `maxDepth: 2+`, `allowSubfolders: true`, `allowParentSelection: true`**

6. **Enable Parent Selection** (2 hours)
   - [ ] Add parent folder dropdown to `FolderFormModal`
   - [ ] Show only when `FOLDER_CONFIG.allowParentSelection === true`
   - [ ] Hierarchical folder picker component

7. **Move Folder Modal** (3 hours)
   - [ ] Create `MoveFolderModal`
   - [ ] Hierarchical folder picker with exclusions
   - [ ] Visual preview of new location
   - [ ] Circular reference prevention
   - [ ] Add to folder context menu (conditional)

8. **Subfolder Support** (2 hours)
   - [ ] Add "Create Subfolder" to context menu
   - [ ] Show only when `FOLDER_CONFIG.allowSubfolders === true`
   - [ ] Update delete modal to show subfolder impact

9. **Enhanced Tree UI** (2 hours)
   - [ ] Visual indicators for nested folders
   - [ ] Breadcrumb navigation
   - [ ] Expand/collapse all option

**Total Phase 2: ~9 hours**

### Phase 3: Protocol-Specific Connection Fields

**Priority: High (after Phase 1 complete)**

10. **Dynamic Form System** (6 hours)
    - [ ] Protocol field schema definition
    - [ ] Dynamic form field rendering based on protocol
    - [ ] Field type support: text, number, select, file upload, etc.
    - [ ] Validation rules from protocol metadata
    - [ ] Conditional field visibility

11. **Protocol-Specific Implementations** (8-12 hours, varies by protocol)
    - [ ] SSH: host, port, username, key selection, authentication method
    - [ ] RDP: host, port, username, domain, screen resolution
    - [ ] Docker: socket path, TLS certificate, API version
    - [ ] Kubernetes: context, namespace, kubeconfig upload
    - [ ] Database protocols: connection string, database name, SSL options
    - [ ] Each protocol requires custom field mapping

12. **Connection Testing** (4 hours)
    - [ ] "Test Connection" button
    - [ ] Preview/dry-run endpoint integration
    - [ ] Connection validation feedback
    - [ ] Error handling and troubleshooting hints

**Total Phase 3: ~18-22 hours**

### Phase 4: Advanced Features (Future)

**Priority: Low**

8. **Drag and Drop** (8 hours)
   - [ ] Drag folders to reorder
   - [ ] Drag folders to reparent
   - [ ] Drag connections to folders
   - [ ] Visual drop indicators
   - [ ] Optimistic updates

9. **Bulk Operations** (4 hours)
   - [ ] Multi-select folders
   - [ ] Bulk move to parent
   - [ ] Bulk delete with confirmation

10. **Folder Templates** (4 hours)
    - [ ] Predefined folder structures
    - [ ] One-click folder hierarchy creation
    - [ ] Templates per protocol type

---

## 10. Testing Checklist

### Unit Tests

- [ ] `FolderFormModal` validation logic
- [ ] `DeleteFolderConfirmModal` impact calculation
- [ ] `MoveFolderModal` circular reference detection
- [ ] Permission checks in all components
- [ ] API client functions with mocked responses

### Integration Tests

- [ ] Create folder with parent
- [ ] Edit folder metadata
- [ ] Delete folder with children (verify reassignment)
- [ ] Move folder to new parent
- [ ] Team scope filtering

### E2E Tests

- [ ] Complete folder lifecycle: create → edit → move → delete
- [ ] Empty state → create first folder → create subfolder
- [ ] Team switching with folder persistence
- [ ] Permission-based UI visibility

### Edge Cases

- [ ] Create folder with duplicate name (backend slug conflict)
- [ ] Delete folder with deeply nested children
- [ ] Move folder to descendant (circular reference prevention)
- [ ] Concurrent edits (optimistic locking)
- [ ] Network errors and retry logic

---

## 11. UI/UX Mockups

### Empty State
```
┌─────────────────────────────────────────┐
│  📁 Folders                             │
├─────────────────────────────────────────┤
│                                         │
│          📂                             │
│                                         │
│     No folders yet                      │
│                                         │
│  Organize your connections into folders │
│  for easier navigation and management.  │
│                                         │
│     [+ Create Folder]                   │
│                                         │
└─────────────────────────────────────────┘
```

### Create Folder Modal - Flat Mode (Default)
```
┌─────────── Create Folder ────────────┐
│                                       │
│  Name *                               │
│  [Production Servers_____________]    │
│                                       │
│  Description                          │
│  [All production infrastructure__]    │
│  [________________________________]    │
│                                       │
│  Icon           Color                 │
│  [📁 ▼]         [🔵 Blue ▼]           │
│                                       │
│  Team                                 │
│  [Personal                    ▼]      │
│                                       │
│           [Cancel]  [Create Folder]   │
└───────────────────────────────────────┘
```

**Note:** Parent Folder selector is **hidden** in flat mode.

### Create Folder Modal - Hierarchical Mode (Optional)
```
┌─────────── Create Folder ────────────┐
│                                       │
│  Name *                               │
│  [Web Services___________________]    │
│                                       │
│  Description                          │
│  [Web tier services______________]    │
│  [________________________________]    │
│                                       │
│  Parent Folder                        │
│  [Production > Backend        ▼]      │
│                                       │
│  Icon           Color                 │
│  [🌐 ▼]         [🟢 Green ▼]          │
│                                       │
│  Team                                 │
│  [Engineering Team            ▼]      │
│                                       │
│           [Cancel]  [Create Folder]   │
└───────────────────────────────────────┘
```

**Note:** Parent Folder selector is **shown** when `allowParentSelection: true`.

### Delete Confirmation Modal - Flat Mode (Default)
```
┌──────── Delete Folder ────────────┐
│                                   │
│  Delete "Production Servers"?     │
│                                   │
│  ⚠️  This action cannot be undone │
│                                   │
│  Impact:                          │
│  • 12 connections → unassigned    │
│                                   │
│  Type folder name to confirm:     │
│  [________________________]       │
│                                   │
│      [Cancel]  [Delete Folder]    │
└───────────────────────────────────┘
```

### Delete Confirmation Modal - Hierarchical Mode (Optional)
```
┌──────── Delete Folder ────────────┐
│                                   │
│  Delete "Backend Projects"?       │
│                                   │
│  ⚠️  This action cannot be undone │
│                                   │
│  Impact:                          │
│  • 3 subfolders → moved to parent │
│  • 12 connections → unassigned    │
│                                   │
│  Type folder name to confirm:     │
│  [________________________]       │
│                                   │
│      [Cancel]  [Delete Folder]    │
└───────────────────────────────────┘
```

### Folder Actions Menu - Flat Mode (Default)
```
┌─────────────────────────┐
│  ✏️  Edit Folder         │
│  🗑️  Delete Folder       │
└─────────────────────────┘
```

### Folder Actions Menu - Hierarchical Mode (Optional)
```
┌─────────────────────────┐
│  ✏️  Edit Folder         │
│  📁  Create Subfolder    │
│  ↗️  Move Folder         │
│  🗑️  Delete Folder       │
└─────────────────────────┘
```

### Connection Form - Basic Fields (MVP)
```
┌──── New Connection: SSH ──────────┐
│                                    │
│  Step 2 of 2                       │
│                                    │
│  Name *                            │
│  [Production SSH Server_______]   │
│                                    │
│  Description                       │
│  [Main production server______]   │
│  [______________________________]  │
│                                    │
│  Folder                            │
│  [Production              ▼]       │
│                                    │
│  Team                              │
│  [Engineering Team        ▼]       │
│                                    │
│  ℹ️  Protocol-specific settings    │
│     will be added in a later       │
│     update                         │
│                                    │
│        [← Back]  [Create]          │
└────────────────────────────────────┘
```

### Connection Form - With Protocol Fields (Future)
```
┌──── New Connection: SSH ──────────┐
│                                    │
│  Step 2 of 2                       │
│                                    │
│  Name *                            │
│  [Production SSH Server_______]   │
│                                    │
│  Description                       │
│  [Main production server______]   │
│                                    │
│  Host *                            │
│  [ssh.example.com_____________]   │
│                                    │
│  Port                              │
│  [22]                              │
│                                    │
│  Username                          │
│  [admin____________________]      │
│                                    │
│  SSH Key                           │
│  [My Production Key       ▼]       │
│                                    │
│  Folder                            │
│  [Production              ▼]       │
│                                    │
│  Team                              │
│  [Engineering Team        ▼]       │
│                                    │
│        [← Back]  [Create]          │
└────────────────────────────────────┘
```

---

## 12. Accessibility Considerations

### Keyboard Navigation

- [ ] Modal dialogs trap focus
- [ ] Folder tree navigable with arrow keys
- [ ] Actions menu accessible via keyboard
- [ ] Form inputs have proper tab order
- [ ] Escape key closes modals

### Screen Readers

- [ ] All interactive elements have aria-labels
- [ ] Form validation errors announced
- [ ] Success/error toasts have aria-live regions
- [ ] Folder tree has proper ARIA tree structure
- [ ] Modal dialogs have descriptive aria-labelledby

### Visual Accessibility

- [ ] Color contrast meets WCAG AA standards
- [ ] Focus indicators visible
- [ ] Icon + text labels (not icon-only)
- [ ] Error states use icons + color
- [ ] Form validation visible without color alone

---

## 13. Performance Considerations

### Optimizations

1. **Tree Rendering:**
   - Virtualize folders if tree exceeds 100 items
   - Lazy load children on expand
   - Memoize tree calculations

2. **API Calls:**
   - Debounce folder search/filter
   - Cache folder tree (5 min stale time)
   - Optimistic updates for mutations

3. **State Management:**
   - Use React Query for server state
   - Local state for UI-only (modal open/close)
   - Avoid prop drilling with context if needed

### Metrics to Monitor

- [ ] Tree render time with 500+ folders
- [ ] Modal open/close animation smoothness
- [ ] API response time for tree endpoint
- [ ] Bundle size impact of new components

---

## 14. Security Considerations

### Permission Validation

- [ ] All mutations check `connection.folder.manage` permission
- [ ] Tree endpoint respects user team memberships
- [ ] Team-scoped folders only editable by team members
- [ ] Audit trail for folder operations (backend)

### Input Sanitization

- [ ] Folder names sanitized (no XSS)
- [ ] Description field sanitized
- [ ] Metadata JSON validated
- [ ] Parent ID validated (prevent arbitrary reassignment)

---

## 15. Success Metrics

### User Adoption

- **Target:** 60% of users create at least one folder within 30 days
- **Target:** Average 5 folders per active user
- **Target:** 80% of connections assigned to folders (vs unassigned)

### Performance

- **Target:** Folder tree loads in < 500ms (p95)
- **Target:** Folder CRUD operations < 200ms (p95)
- **Target:** Zero permission bypass incidents

### User Satisfaction

- **Target:** < 5% support tickets related to folder confusion
- **Target:** Positive feedback on folder UX in user surveys
- **Target:** Feature usage increases connection organization by 3x

---

## 16. References

### Backend Implementation Files
- `/home/ubuntu/projects/charlesng/shellcn/internal/api/routes_connection_folders.go`
- `/home/ubuntu/projects/charlesng/shellcn/internal/handlers/connection_folders.go`
- `/home/ubuntu/projects/charlesng/shellcn/internal/services/connection_folder_service.go`
- `/home/ubuntu/projects/charlesng/shellcn/internal/models/connection_folder.go`

### Frontend Files
- `/home/ubuntu/projects/charlesng/shellcn/web/src/components/connections/FolderTree.tsx`
- `/home/ubuntu/projects/charlesng/shellcn/web/src/components/connections/FolderSidebar.tsx`
- `/home/ubuntu/projects/charlesng/shellcn/web/src/lib/api/connection-folders.ts`
- `/home/ubuntu/projects/charlesng/shellcn/web/src/hooks/useConnectionFolders.ts`

### Design System
- `/home/ubuntu/projects/charlesng/shellcn/web/src/components/ui/Modal.tsx`
- `/home/ubuntu/projects/charlesng/shellcn/specs/project/FRONTEND_GUIDELINES.md`

### API Documentation
- `/home/ubuntu/projects/charlesng/shellcn/specs/plans/1. core/CORE_MODULE_API.md` (lines 942-945)

---

## 17. Change Log

- **2025-10-10 (v1.3)** - Clarified **phased approach** for connection form fields
  - Initial implementation: Basic connection entity fields only (name, description, folder, team)
  - Protocol-specific fields deferred to Phase 3 (future enhancement)
  - Added `ConnectionFormModal` component specification
  - Added mockups for basic form and future protocol-specific form
  - Updated implementation plan: Phase 1 (MVP) ~18.5 hours, Phase 3 (protocol fields) ~18-22 hours
  - Clear separation of concerns: entity management first, protocol details later

- **2025-10-10 (v1.2)** - Added **Connection Creation Flow** specification
  - Added section 4.6: Create New Connection with two-step wizard
  - First step: Resource Selection Modal (protocol picker)
  - Permission requirement: `connection.manage`
  - UI terminology: Use "Resources" instead of "Protocols" for user-friendliness
  - Integration with existing `GET /api/protocols/available` endpoint
  - Folder assignment during connection creation

- **2025-10-10 (v1.1)** - Updated specification to default to **flat folder structure** (`maxDepth: 1`) with configurable hierarchical mode for future expansion. This simplifies the initial implementation and prevents UI over-complication.
  - Added `FOLDER_CONFIG` configuration system
  - Set default to single-level folders (no subfolders)
  - Made hierarchical features (subfolder creation, moving folders) optional
  - Updated all components, flows, and mockups to reflect flat vs hierarchical modes
  - Reduced Phase 1 MVP scope to ~12.5 hours (from ~16 hours)
  - Moved hierarchical features to Phase 2 (optional, ~9 hours)

- **2025-10-10 (v1.0)** - Initial specification created based on backend analysis and existing UI components
