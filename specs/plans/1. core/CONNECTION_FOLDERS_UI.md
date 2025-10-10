# Connection Folders UI Specification

**Version:** 1.0
**Date:** 2025-10-10
**Status:** Draft

---

## 1. Overview

This specification defines the UI/UX implementation for Connection Folder management in ShellCN. Folders provide hierarchical organization of connections, supporting unlimited nesting depth with parent-child relationships. The folder system is multi-tenant aware, supporting both personal and team-scoped folders.

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
- Parent folder selection (hierarchical dropdown)
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
  parentFolder?: ConnectionFolderDTO  // For create with parent
  teamId?: string | null  // Pre-selected team
  availableFolders: ConnectionFolderNode[]  // For parent selection
  onSuccess: (folder: ConnectionFolderDTO) => void
}
```

#### 3.2.2 DeleteFolderConfirmModal
**Purpose:** Confirm folder deletion with impact preview
**Location:** `/web/src/components/connections/DeleteFolderConfirmModal.tsx`

**Features:**
- Shows folder name and description
- Displays impact preview:
  - Number of child folders (will be moved to parent)
  - Number of connections (will be moved to parent or unassigned)
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

#### 3.2.3 MoveFolderModal
**Purpose:** Change folder parent (reparent)
**Location:** `/web/src/components/connections/MoveFolderModal.tsx`

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

#### 3.2.4 FolderContextMenu
**Purpose:** Right-click or actions menu for folders
**Location:** `/web/src/components/connections/FolderContextMenu.tsx`

**Features:**
- Create subfolder
- Edit folder
- Move folder
- Delete folder
- Permission-based visibility (`connection.folder.manage`)

**Props:**
```typescript
interface FolderContextMenuProps {
  folder: ConnectionFolderDTO
  onCreateSubfolder: (parentId: string) => void
  onEdit: (folder: ConnectionFolderDTO) => void
  onMove: (folder: ConnectionFolderDTO) => void
  onDelete: (folder: ConnectionFolderDTO) => void
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

**Flow:**

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

**Flow:**

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
3. Shows impact:
   ```
   Delete folder "Backend Projects"?

   Impact:
   • 3 subfolders will be moved to "Projects"
   • 12 connections will be moved to "Projects"

   Type "Backend Projects" to confirm
   ```
4. User types folder name to confirm
5. On confirm:
   - DELETE `/api/connection-folders/:id`
   - Refresh tree
   - Clear folder filter (show all)
   - Show success toast with undo option (if feasible)

### 4.6 Move Connection to Folder

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

### Phase 1: Core Folder Management (MVP)

**Priority: High**

1. **Fix Empty State** (2 hours)
   - [ ] Update `FolderSidebar.tsx` to show empty state instead of `null`
   - [ ] Create `EmptyFolderState` component with CTA
   - [ ] Add permission check for create button

2. **Create FolderFormModal** (4 hours)
   - [ ] Build modal form component
   - [ ] Implement create mode with name, description, icon, color
   - [ ] Add parent folder selection (hierarchical dropdown)
   - [ ] Add team assignment (if applicable)
   - [ ] Form validation and error handling
   - [ ] Integrate with `useCreateFolder` mutation

3. **Edit Folder** (2 hours)
   - [ ] Add edit mode to `FolderFormModal`
   - [ ] Add actions menu to folder items (⋮ button)
   - [ ] Integrate with `useUpdateFolder` mutation

4. **Delete Folder** (3 hours)
   - [ ] Create `DeleteFolderConfirmModal`
   - [ ] Show impact preview (children, connections count)
   - [ ] Confirmation input validation
   - [ ] Integrate with `useDeleteFolder` mutation

### Phase 2: Enhanced UX (Nice to Have)

**Priority: Medium**

5. **Move Folder** (3 hours)
   - [ ] Create `MoveFolderModal`
   - [ ] Hierarchical folder picker with exclusions
   - [ ] Visual preview of new location
   - [ ] Circular reference prevention

6. **Folder Context Menu** (2 hours)
   - [ ] Right-click context menu on folders
   - [ ] Create subfolder shortcut
   - [ ] Quick actions (edit, move, delete)

7. **Icon and Color Pickers** (3 hours)
   - [ ] Icon picker component with predefined set
   - [ ] Color picker with palette
   - [ ] Preview in folder tree

### Phase 3: Advanced Features (Future)

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

### Create Folder Modal
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
│  Parent Folder                        │
│  [🏠 Root (No Parent)         ▼]      │
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

### Delete Confirmation Modal
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

### Folder Actions Menu
```
┌─────────────────────────┐
│  ✏️  Edit Folder         │
│  📁  Create Subfolder    │
│  ↗️  Move Folder         │
│  🗑️  Delete Folder       │
└─────────────────────────┘
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

- **2025-10-10** - Initial specification created based on backend analysis and existing UI components
