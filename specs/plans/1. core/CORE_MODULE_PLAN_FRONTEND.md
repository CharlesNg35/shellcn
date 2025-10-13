# Core Module - Frontend Implementation Plan

**Module:** Core (Auth, Users, Permissions)
**Status:** Required (Always Enabled)
**Dependencies:** None (Foundation Module)

---

## Table of Contents

1. [Overview](#overview)
2. [Technology Stack](#technology-stack)
3. [Project Structure](#project-structure)
4. [Application Architecture](#application-architecture)
5. [Authentication & Setup Flows](#authentication--setup-flows)
6. [User Management UI](#user-management-ui)
7. [Team Management](#team-management)
8. [Permission Management](#permission-management)
9. [Session Management](#session-management)
10. [Audit Log Viewer](#audit-log-viewer)
11. [Shared Components](#shared-components)
12. [Custom Hooks](#custom-hooks)
13. [State Management](#state-management)
14. [API Integration](#api-integration)
15. [Routing & Navigation](#routing--navigation)
16. [Security Implementation](#security-implementation)
17. [Testing Strategy](#testing-strategy)
18. [Implementation Checklist](#implementation-checklist)

---

## Overview

The Core Module frontend provides the user interface for authentication, user management, team management, permission management, and audit logging. This is the foundation that all other modules build upon.

### Key Features

- **Authentication UI:**
  - Login page with local auth
  - Optional SSO provider buttons (OIDC, SAML, LDAP)
  - MFA setup and verification
  - Password reset flow
  - First-time setup wizard

- **User Management:**
  - User list with filtering and pagination
  - Create/edit user forms
  - User activation/deactivation
  - Password management
  - Profile management

- **Team Management:**
  - Team CRUD
  - Member assignment
  - Team hierarchy views

- **Permission Management:**
  - Permission matrix view
  - Role management
  - Permission assignment
  - Dependency visualization

- **Session Management:**
  - Active session list
  - Session revocation
  - Multi-device view

- **Audit Logging:**
  - Audit log viewer
  - Filtering and search
  - Export functionality

---

## Technology Stack

### Core Technologies

- **React 19:** Latest React features (Compiler, Actions, use hook)
- **TypeScript 5.9+:** Type safety with latest features
- **Vite 7:** Next-generation build tool and dev server with improved performance
- **React Router 7:** SPA routing with loaders/actions (TanStack Router alternative)
- **Tailwind CSS v4:** Utility-first styling with new engine and improved performance

**IMPORTANT: Technology Versions**
- Always use the latest stable versions specified in `project_spec.md`
- Vite 7 brings significant performance improvements over Vite 5/6
- Tailwind CSS 4 has a new engine (Oxide) with better performance and new features
- **Tailwind CSS v4 uses CSS-based configuration via `@theme` directive in CSS files, NOT `tailwind.config.ts`**
- React 19 includes the new Compiler (no more manual memoization needed)

### State Management

- **TanStack Query (React Query):** Server state management
- **Zustand:** Client state management (auth, preferences)

### Form Handling

- **react-hook-form:** Form state management
- **Zod:** Schema validation

### UI Components

- **Radix UI:** Headless accessible components
- **class-variance-authority (CVA):** Component variants
- **Lucide React:** Icon library

### Data Display

- **TanStack Table:** Data grids
- **Recharts:** Charts and visualizations

### Testing

- **Vitest:** Unit testing
- **React Testing Library:** Component testing
- **Cypress:** E2E testing

---

## Project Structure

```
web/
├── src/
│   ├── pages/                          # Page components
│   │   ├── auth/
│   │   │   ├── Login.tsx
│   │   │   ├── Setup.tsx
│   │   │   ├── PasswordReset.tsx
│   │   │   └── MFASetup.tsx
│   │   │
│   │   ├── dashboard/
│   │   │   └── Dashboard.tsx
│   │   │
│   │   └── settings/
│   │       ├── Users.tsx
│   │       ├── Teams.tsx
│   │       ├── Permissions.tsx
│   │       ├── Security.tsx
│   │       ├── Sessions.tsx
│   │       ├── AuditLogs.tsx
│   │       └── AuthProviders.tsx
│   │
│   ├── components/                     # Reusable components
│   │   ├── ui/                         # Base UI components
│   │   │   ├── Button.tsx
│   │   │   ├── Input.tsx
│   │   │   ├── Select.tsx
│   │   │   ├── Modal.tsx
│   │   │   ├── Table.tsx
│   │   │   ├── Card.tsx
│   │   │   ├── Badge.tsx
│   │   │   ├── Tabs.tsx
│   │   │   └── ...
│   │   │
│   │   ├── auth/                       # Auth-specific components
│   │   │   ├── LoginForm.tsx
│   │   │   ├── MFAVerification.tsx
│   │   │   ├── PasswordStrengthMeter.tsx
│   │   │   └── SSOButtons.tsx
│   │   │
│   │   ├── users/                      # User management components
│   │   │   ├── UserTable.tsx
│   │   │   ├── UserForm.tsx
│   │   │   ├── UserDetailModal.tsx
│   │   │   └── UserFilters.tsx
│   │   │
│   │   ├── permissions/                # Permission components
│   │   │   ├── PermissionMatrix.tsx
│   │   │   ├── RoleManager.tsx
│   │   │   ├── PermissionGuard.tsx
│   │   │   └── PermissionBadge.tsx
│   │   │
│   │   ├── settings/                   # Account & preference components
│   │   │   ├── AccountSettingsPanel.tsx
│   │   │   ├── SecuritySettingsPanel.tsx
│   │   │   ├── AppearanceSettingsPanel.tsx
│   │   │   └── ProfileSessionsPanel.tsx
│   │   │
│   │   ├── audit/                      # Audit components
│   │   │   ├── AuditLogTable.tsx
│   │   │   ├── AuditFilters.tsx
│   │   │   └── AuditExport.tsx
│   │   │
│   │   ├── auth-providers/             # Auth provider components
│   │   │   ├── ProviderCard.tsx
│   │   │   ├── OIDCConfigForm.tsx
│   │   │   ├── SAMLConfigForm.tsx
│   │   │   ├── LDAPConfigForm.tsx
│   │   │   ├── LocalSettingsForm.tsx
│   │   │   └── InviteSettingsForm.tsx
│   │   │
│   │   └── layout/                     # Layout components
│   │       ├── AuthLayout.tsx
│   │       ├── DashboardLayout.tsx
│   │       ├── Sidebar.tsx
│   │       ├── Header.tsx
│   │       └── Footer.tsx
│   │
│   ├── hooks/                          # Custom hooks
│   │   ├── useAuth.ts
│   │   ├── useCurrentUser.ts
│   │   ├── usePermissions.ts
│   │   ├── useUsers.ts
│   │   ├── useTeams.ts
│   │   ├── useProfileSettings.ts      # Profile preferences, MFA, sessions
│   │   ├── useAuditLogs.ts
│   │   └── useAuthProviders.ts
│   │
│   ├── lib/                            # Utilities and libraries
│   │   ├── api/                        # API client
│   │   │   ├── client.ts               # Axios instance
│   │   │   ├── auth.ts
│   │   │   ├── users.ts
│   │   │   ├── teams.ts
│   │   │   ├── permissions.ts
│   │   │   ├── sessions.ts
│   │   │   ├── audit.ts
│   │   │   └── authProviders.ts
│   │   │
│   │   ├── utils/                      # Utility functions
│   │   │   ├── cn.ts                   # Class name merger
│   │   │   ├── format.ts               # Date/time formatting
│   │   │   └── validation.ts           # Validation helpers
│   │   │
│   │   └── constants.ts                # App constants
│   │
│   ├── store/                          # Zustand stores
│   │   ├── authStore.ts                # Auth state
│   │   ├── settingsStore.ts            # User preferences
│   │   └── featureFlagsStore.ts        # Feature flags
│   │
│   ├── types/                          # TypeScript types
│   │   ├── auth.ts
│   │   ├── user.ts
│   │   ├── team.ts
│   │   ├── permission.ts
│   │   ├── session.ts
│   │   ├── audit.ts
│   │   └── api.ts
│   │
│   ├── schemas/                        # Zod validation schemas
│   │   ├── auth.ts
│   │   ├── user.ts
│   │   └── team.ts
│   │
│   ├── App.tsx                         # Root component
│   ├── main.tsx                        # Entry point
│   └── router.tsx                      # Route configuration
│
├── public/
│   └── assets/
│
├── index.html
├── vite.config.ts
├── tsconfig.json
└── package.json
```

**Note:** Tailwind CSS v4 does not use `tailwind.config.ts`. Configuration is done via `@theme` directive in CSS files (see `src/index.css`).

---

## Application Architecture

### App Entry Point

**Location:** `src/main.tsx`

```tsx
import React from 'react'
import ReactDOM from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { ReactQueryDevtools } from '@tanstack/react-query-devtools'
import App from './App'
import './index.css'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 1000 * 60 * 5, // 5 minutes
      retry: 1,
    },
  },
})

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <QueryClientProvider client={queryClient}>
      <App />
      <ReactQueryDevtools initialIsOpen={false} />
    </QueryClientProvider>
  </React.StrictMode>,
)
```

### Root Component

**Location:** `src/App.tsx`

```tsx
import { RouterProvider } from 'react-router-dom'
import { Toaster } from '@/components/ui/toaster'
import { router } from './router'

function App() {
  return (
    <>
      <RouterProvider router={router} />
      <Toaster />
    </>
  )
}

export default App
```

### Layout Components

#### Auth Layout

**Location:** `src/components/layout/AuthLayout.tsx`

```tsx
import { Outlet } from 'react-router-dom'

export function AuthLayout() {
  return (
    <div className="min-h-screen flex items-center justify-center bg-gray-50">
      <div className="max-w-md w-full space-y-8 p-8 bg-white rounded-lg shadow-lg">
        <div className="text-center">
          <h1 className="text-3xl font-bold">ShellCN</h1>
          <p className="text-gray-600 mt-2">Enterprise Remote Access Platform</p>
        </div>
        <Outlet />
      </div>
    </div>
  )
}
```

#### Dashboard Layout

**Location:** `src/components/layout/DashboardLayout.tsx`

```tsx
import { Outlet } from 'react-router-dom'
import { Sidebar } from './Sidebar'
import { Header } from './Header'
import { useAuth } from '@/hooks/useAuth'
import { Navigate } from 'react-router-dom'

export function DashboardLayout() {
  const { isAuthenticated } = useAuth()

  if (!isAuthenticated) {
    return <Navigate to="/login" replace />
  }

  return (
    <div className="min-h-screen bg-gray-100">
      <Sidebar />
      <div className="ml-64">
        <Header />
        <main className="p-6">
          <Outlet />
        </main>
      </div>
    </div>
  )
}
```

#### Sidebar

**Location:** `src/components/layout/Sidebar.tsx`

```tsx
import { NavLink } from 'react-router-dom'
import {
  LayoutDashboard,
  Users,
  Building,
  Shield,
  Settings,
  FileText
} from 'lucide-react'
import { usePermissions } from '@/hooks/usePermissions'
import { cn } from '@/lib/utils/cn'

export function Sidebar() {
  const { hasPermission } = usePermissions()

  const navItems = [
    { to: '/dashboard', icon: LayoutDashboard, label: 'Dashboard' },
    {
      to: '/settings/users',
      icon: Users,
      label: 'Users',
      permission: 'user.view'
    },
    {
      to: '/settings/permissions',
      icon: Shield,
      label: 'Permissions',
      permission: 'permission.view'
    },
    {
      to: '/settings/auth-providers',
      icon: Shield,
      label: 'Auth Providers',
      permission: 'permission.manage'
    },
    {
      to: '/settings/audit',
      icon: FileText,
      label: 'Audit Logs',
      permission: 'audit.view'
    },
    { to: '/settings/security', icon: Settings, label: 'Security' },
  ]

  return (
    <aside className="fixed left-0 top-0 h-screen w-64 bg-gray-900 text-white">
      <div className="p-6">
        <h1 className="text-2xl font-bold">ShellCN</h1>
      </div>

      <nav className="mt-6">
        {navItems.map((item) => {
          // Hide items if user doesn't have permission
          if (item.permission && !hasPermission(item.permission)) {
            return null
          }

          return (
            <NavLink
              key={item.to}
              to={item.to}
              className={({ isActive }) =>
                cn(
                  'flex items-center px-6 py-3 text-gray-300 hover:bg-gray-800 hover:text-white transition-colors',
                  isActive && 'bg-gray-800 text-white border-l-4 border-blue-500'
                )
              }
            >
              <item.icon className="w-5 h-5 mr-3" />
              {item.label}
            </NavLink>
          )
        })}
      </nav>
    </aside>
  )
}
```

---

## Authentication & Setup Flows

### Login Page

**Location:** `src/pages/auth/Login.tsx`

```tsx
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { loginSchema } from '@/schemas/auth'
import { useAuth } from '@/hooks/useAuth'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import { Link } from 'react-router-dom'
import type { z } from 'zod'

type LoginFormData = z.infer<typeof loginSchema>

export function Login() {
  const { login, isLoading } = useAuth()

  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<LoginFormData>({
    resolver: zodResolver(loginSchema),
  })

  const onSubmit = async (data: LoginFormData) => {
    await login(data.username, data.password)
  }

  return (
    <div className="space-y-6">
      <h2 className="text-2xl font-bold text-center">Sign In</h2>

      <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
        <div>
          <Input
            {...register('username')}
            label="Username or Email"
            placeholder="Enter your username"
            error={errors.username?.message}
          />
        </div>

        <div>
          <Input
            {...register('password')}
            type="password"
            label="Password"
            placeholder="Enter your password"
            error={errors.password?.message}
          />
        </div>

        <Button
          type="submit"
          className="w-full"
          loading={isLoading}
        >
          Sign In
        </Button>
      </form>

      <div className="text-center text-sm">
        <Link to="/password-reset" className="text-blue-600 hover:underline">
          Forgot password?
        </Link>
      </div>
    </div>
  )
}
```

### Login Schema

**Location:** `src/schemas/auth.ts`

```tsx
import { z } from 'zod'

export const loginSchema = z.object({
  username: z.string().min(1, 'Username is required'),
  password: z.string().min(1, 'Password is required'),
})

export const setupSchema = z.object({
  username: z.string().min(3, 'Username must be at least 3 characters'),
  email: z.string().email('Invalid email address'),
  password: z.string()
    .min(8, 'Password must be at least 8 characters')
    .regex(/[A-Z]/, 'Password must contain at least one uppercase letter')
    .regex(/[a-z]/, 'Password must contain at least one lowercase letter')
    .regex(/[0-9]/, 'Password must contain at least one number'),
  confirmPassword: z.string(),
  firstName: z.string().optional(),
  lastName: z.string().optional(),
}).refine((data) => data.password === data.confirmPassword, {
  message: "Passwords don't match",
  path: ['confirmPassword'],
})

export const passwordResetRequestSchema = z.object({
  email: z.string().email('Invalid email address'),
})

export const passwordResetConfirmSchema = z.object({
  token: z.string(),
  password: z.string().min(8, 'Password must be at least 8 characters'),
  confirmPassword: z.string(),
}).refine((data) => data.password === data.confirmPassword, {
  message: "Passwords don't match",
  path: ['confirmPassword'],
})
```

### Setup Wizard

**Location:** `src/pages/auth/Setup.tsx`

```tsx
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { setupSchema } from '@/schemas/auth'
import { useNavigate } from 'react-router-dom'
import { useMutation } from '@tanstack/react-query'
import { authApi } from '@/lib/api/auth'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import { PasswordStrengthMeter } from '@/components/auth/PasswordStrengthMeter'
import type { z } from 'zod'

type SetupFormData = z.infer<typeof setupSchema>

export function Setup() {
  const navigate = useNavigate()

  const {
    register,
    handleSubmit,
    watch,
    formState: { errors },
  } = useForm<SetupFormData>({
    resolver: zodResolver(setupSchema),
  })

  const password = watch('password')

  const setupMutation = useMutation({
    mutationFn: authApi.completeSetup,
    onSuccess: () => {
      navigate('/login')
    },
  })

  const onSubmit = async (data: SetupFormData) => {
    await setupMutation.mutateAsync(data)
  }

  return (
    <div className="space-y-6">
      <div className="text-center">
        <h2 className="text-2xl font-bold">Welcome to ShellCN</h2>
        <p className="text-gray-600 mt-2">Create your administrator account</p>
      </div>

      <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
        <div className="grid grid-cols-2 gap-4">
          <Input
            {...register('firstName')}
            label="First Name"
            placeholder="John"
            error={errors.firstName?.message}
          />
          <Input
            {...register('lastName')}
            label="Last Name"
            placeholder="Doe"
            error={errors.lastName?.message}
          />
        </div>

        <Input
          {...register('username')}
          label="Username"
          placeholder="admin"
          error={errors.username?.message}
        />

        <Input
          {...register('email')}
          type="email"
          label="Email"
          placeholder="admin@example.com"
          error={errors.email?.message}
        />

        <div>
          <Input
            {...register('password')}
            type="password"
            label="Password"
            placeholder="Enter a strong password"
            error={errors.password?.message}
          />
          {password && <PasswordStrengthMeter password={password} />}
        </div>

        <Input
          {...register('confirmPassword')}
          type="password"
          label="Confirm Password"
          placeholder="Re-enter your password"
          error={errors.confirmPassword?.message}
        />

        <Button
          type="submit"
          className="w-full"
          loading={setupMutation.isPending}
        >
          Create Account
        </Button>
      </form>
    </div>
  )
}
```

---

## User Management UI

### User List Page

**Location:** `src/pages/settings/Users.tsx`

```tsx
import { useState } from 'react'
import { useUsers } from '@/hooks/useUsers'
import { UserTable } from '@/components/users/UserTable'
import { UserForm } from '@/components/users/UserForm'
import { UserFilters } from '@/components/users/UserFilters'
import { Button } from '@/components/ui/Button'
import { Modal } from '@/components/ui/Modal'
import { Plus } from 'lucide-react'
import { PermissionGuard } from '@/components/permissions/PermissionGuard'

export function Users() {
  const [page, setPage] = useState(1)
  const [filters, setFilters] = useState({})
  const [isCreateModalOpen, setIsCreateModalOpen] = useState(false)

  const { data, isLoading } = useUsers({ page, perPage: 20, filters })

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <h1 className="text-3xl font-bold">Users</h1>

        <PermissionGuard permission="user.create">
          <Button onClick={() => setIsCreateModalOpen(true)}>
            <Plus className="w-4 h-4 mr-2" />
            Create User
          </Button>
        </PermissionGuard>
      </div>

      <UserFilters filters={filters} onChange={setFilters} />

      <UserTable
        users={data?.users || []}
        total={data?.total || 0}
        page={page}
        perPage={20}
        onPageChange={setPage}
        isLoading={isLoading}
      />

      <Modal
        open={isCreateModalOpen}
        onClose={() => setIsCreateModalOpen(false)}
        title="Create User"
      >
        <UserForm
          onSuccess={() => setIsCreateModalOpen(false)}
        />
      </Modal>
    </div>
  )
}
```

### User Table Component

**Location:** `src/components/users/UserTable.tsx`

```tsx
import { useState } from 'react'
import {
  useReactTable,
  getCoreRowModel,
  flexRender,
  type ColumnDef,
} from '@tanstack/react-table'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { MoreVertical, Edit, Trash, Power } from 'lucide-react'
import { PermissionGuard } from '@/components/permissions/PermissionGuard'
import type { User } from '@/types/user'

interface UserTableProps {
  users: User[]
  total: number
  page: number
  perPage: number
  onPageChange: (page: number) => void
  isLoading: boolean
}

export function UserTable({ users, total, page, perPage, onPageChange, isLoading }: UserTableProps) {
  const columns: ColumnDef<User>[] = [
    {
      accessorKey: 'username',
      header: 'Username',
    },
    {
      accessorKey: 'email',
      header: 'Email',
    },
    {
      accessorKey: 'is_active',
      header: 'Status',
      cell: ({ row }) => (
        <Badge variant={row.original.is_active ? 'success' : 'secondary'}>
          {row.original.is_active ? 'Active' : 'Inactive'}
        </Badge>
      ),
    },
    {
      accessorKey: 'is_root',
      header: 'Role',
      cell: ({ row }) => (
        row.original.is_root ? (
          <Badge variant="destructive">Root</Badge>
        ) : (
          <Badge variant="default">User</Badge>
        )
      ),
    },
    {
      accessorKey: 'last_login_at',
      header: 'Last Login',
      cell: ({ row }) => (
        row.original.last_login_at
          ? new Date(row.original.last_login_at).toLocaleString()
          : 'Never'
      ),
    },
    {
      id: 'actions',
      cell: ({ row }) => (
        <div className="flex gap-2">
          <PermissionGuard permission="user.edit">
            <Button size="sm" variant="ghost">
              <Edit className="w-4 h-4" />
            </Button>
          </PermissionGuard>

          <PermissionGuard permission="user.delete">
            <Button
              size="sm"
              variant="ghost"
              disabled={row.original.is_root}
            >
              <Trash className="w-4 h-4" />
            </Button>
          </PermissionGuard>
        </div>
      ),
    },
  ]

  const table = useReactTable({
    data: users,
    columns,
    getCoreRowModel: getCoreRowModel(),
  })

  if (isLoading) {
    return <div>Loading...</div>
  }

  return (
    <div className="bg-white rounded-lg shadow">
      <table className="w-full">
        <thead className="bg-gray-50 border-b">
          {table.getHeaderGroups().map((headerGroup) => (
            <tr key={headerGroup.id}>
              {headerGroup.headers.map((header) => (
                <th
                  key={header.id}
                  className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider"
                >
                  {flexRender(
                    header.column.columnDef.header,
                    header.getContext()
                  )}
                </th>
              ))}
            </tr>
          ))}
        </thead>
        <tbody className="divide-y divide-gray-200">
          {table.getRowModel().rows.map((row) => (
            <tr key={row.id} className="hover:bg-gray-50">
              {row.getVisibleCells().map((cell) => (
                <td key={cell.id} className="px-6 py-4 whitespace-nowrap">
                  {flexRender(cell.column.columnDef.cell, cell.getContext())}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>

      {/* Pagination */}
      <div className="px-6 py-4 flex items-center justify-between border-t">
        <div className="text-sm text-gray-700">
          Showing {(page - 1) * perPage + 1} to {Math.min(page * perPage, total)} of {total} results
        </div>
        <div className="flex gap-2">
          <Button
            size="sm"
            variant="outline"
            onClick={() => onPageChange(page - 1)}
            disabled={page === 1}
          >
            Previous
          </Button>
          <Button
            size="sm"
            variant="outline"
            onClick={() => onPageChange(page + 1)}
            disabled={page * perPage >= total}
          >
            Next
          </Button>
        </div>
      </div>
    </div>
  )
}
```

### User Form Component

**Location:** `src/components/users/UserForm.tsx`

```tsx
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { userSchema } from '@/schemas/user'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { usersApi } from '@/lib/api/users'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import { Select } from '@/components/ui/Select'
import type { z } from 'zod'

type UserFormData = z.infer<typeof userSchema>

interface UserFormProps {
  user?: User
  onSuccess: () => void
}

export function UserForm({ user, onSuccess }: UserFormProps) {
  const queryClient = useQueryClient()

  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<UserFormData>({
    resolver: zodResolver(userSchema),
    defaultValues: user,
  })

  const mutation = useMutation({
    mutationFn: user
      ? (data: UserFormData) => usersApi.update(user.id, data)
      : usersApi.create,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['users'] })
      onSuccess()
    },
  })

  const onSubmit = async (data: UserFormData) => {
    await mutation.mutateAsync(data)
  }

  return (
    <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
      <Input
        {...register('username')}
        label="Username"
        error={errors.username?.message}
      />

      <Input
        {...register('email')}
        type="email"
        label="Email"
        error={errors.email?.message}
      />

      {!user && (
        <Input
          {...register('password')}
          type="password"
          label="Password"
          error={errors.password?.message}
        />
      )}

      <div className="grid grid-cols-2 gap-4">
        <Input
          {...register('first_name')}
          label="First Name"
          error={errors.first_name?.message}
        />
        <Input
          {...register('last_name')}
          label="Last Name"
          error={errors.last_name?.message}
        />
      </div>

      <div className="flex justify-end gap-2">
        <Button type="button" variant="outline" onClick={onSuccess}>
          Cancel
        </Button>
        <Button type="submit" loading={mutation.isPending}>
          {user ? 'Update' : 'Create'} User
        </Button>
      </div>
    </form>
  )
}
```

---

## Permission Management

### Permission Guard Component

**Location:** `src/components/permissions/PermissionGuard.tsx`

```tsx
import { usePermissions } from '@/hooks/usePermissions'
import type { ReactNode } from 'react'

interface PermissionGuardProps {
  permission: string
  children: ReactNode
  fallback?: ReactNode
}

export function PermissionGuard({ permission, children, fallback = null }: PermissionGuardProps) {
  const { hasPermission } = usePermissions()

  if (!hasPermission(permission)) {
    return <>{fallback}</>
  }

  return <>{children}</>
}
```

### Permission Matrix Component

**Location:** `src/components/permissions/PermissionMatrix.tsx`

```tsx
import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { permissionsApi } from '@/lib/api/permissions'
import { Checkbox } from '@/components/ui/Checkbox'
import { Badge } from '@/components/ui/Badge'
import { Info } from 'lucide-react'
import { Tooltip } from '@/components/ui/Tooltip'

interface PermissionMatrixProps {
  roleId: string
}

export function PermissionMatrix({ roleId }: PermissionMatrixProps) {
  const queryClient = useQueryClient()

  const { data: allPermissions } = useQuery({
    queryKey: ['permissions'],
    queryFn: permissionsApi.getAll,
  })

  const { data: rolePermissions } = useQuery({
    queryKey: ['permissions', 'role', roleId],
    queryFn: () => permissionsApi.getRolePermissions(roleId),
  })

  const mutation = useMutation({
    mutationFn: ({ permissionId, enabled }: { permissionId: string; enabled: boolean }) =>
      enabled
        ? permissionsApi.assignToRole(roleId, permissionId)
        : permissionsApi.removeFromRole(roleId, permissionId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['permissions', 'role', roleId] })
    },
  })

  const hasPermission = (permissionId: string) => {
    return rolePermissions?.some((p) => p.id === permissionId) || false
  }

  const handleToggle = (permissionId: string, enabled: boolean) => {
    mutation.mutate({ permissionId, enabled })
  }

  // Group permissions by module
  const groupedPermissions = allPermissions?.reduce((acc, perm) => {
    if (!acc[perm.module]) {
      acc[perm.module] = []
    }
    acc[perm.module].push(perm)
    return acc
  }, {} as Record<string, typeof allPermissions>)

  return (
    <div className="space-y-6">
      {Object.entries(groupedPermissions || {}).map(([module, permissions]) => (
        <div key={module} className="bg-white rounded-lg shadow p-6">
          <h3 className="text-lg font-semibold mb-4 capitalize">{module} Module</h3>

          <div className="space-y-3">
            {permissions.map((permission) => (
              <div key={permission.id} className="flex items-center justify-between">
                <div className="flex items-center gap-3">
                  <Checkbox
                    checked={hasPermission(permission.id)}
                    onCheckedChange={(checked) =>
                      handleToggle(permission.id, checked as boolean)
                    }
                  />
                  <div>
                    <div className="flex items-center gap-2">
                      <span className="font-medium">{permission.id}</span>
                      {permission.depends_on?.length > 0 && (
                        <Tooltip content={`Depends on: ${permission.depends_on.join(', ')}`}>
                          <Info className="w-4 h-4 text-gray-400" />
                        </Tooltip>
                      )}
                    </div>
                    <p className="text-sm text-gray-600">{permission.description}</p>
                  </div>
                </div>

                {permission.depends_on?.length > 0 && (
                  <div className="flex gap-1">
                    {permission.depends_on.map((dep) => (
                      <Badge key={dep} variant="outline" size="sm">
                        {dep}
                      </Badge>
                    ))}
                  </div>
                )}
              </div>
            ))}
          </div>
        </div>
      ))}
    </div>
  )
}
```

---

## Auth Provider Management UI

**IMPORTANT:** Authentication providers are configured via UI by admins, not config files.

### Auth Providers Page

**Location:** `src/pages/settings/AuthProviders.tsx`

```tsx
import { useState } from 'react'
import { useAuthProviders } from '@/hooks/useAuthProviders'
import { ProviderCard } from '@/components/auth-providers/ProviderCard'
import { OIDCConfigForm } from '@/components/auth-providers/OIDCConfigForm'
import { SAMLConfigForm } from '@/components/auth-providers/SAMLConfigForm'
import { LDAPConfigForm } from '@/components/auth-providers/LDAPConfigForm'
import { LocalSettingsForm } from '@/components/auth-providers/LocalSettingsForm'
import { InviteSettingsForm } from '@/components/auth-providers/InviteSettingsForm'
import { Modal } from '@/components/ui/Modal'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/Tabs'
import { PermissionGuard } from '@/components/permissions/PermissionGuard'

export function AuthProviders() {
  const { data: providers, isLoading } = useAuthProviders()
  const [configureModal, setConfigureModal] = useState<string | null>(null)

  const providerTypes = [
    { type: 'local', name: 'Local Authentication', icon: 'key', description: 'Username and password' },
    { type: 'invite', name: 'Email Invitation', icon: 'mail', description: 'Invite users via email' },
    { type: 'oidc', name: 'OpenID Connect', icon: 'shield-check', description: 'OIDC SSO' },
    { type: 'saml', name: 'SAML 2.0', icon: 'shield', description: 'SAML SSO' },
    { type: 'ldap', name: 'LDAP / AD', icon: 'building', description: 'LDAP or Active Directory' },
  ]

  return (
    <PermissionGuard permission="permission.manage">
      <div className="space-y-6">
        <div>
          <h1 className="text-3xl font-bold">Authentication Providers</h1>
          <p className="text-gray-600 mt-2">
            Configure authentication methods for your users
          </p>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {providerTypes.map((providerType) => {
            const provider = providers?.find((p) => p.type === providerType.type)

            return (
              <ProviderCard
                key={providerType.type}
                type={providerType.type}
                name={providerType.name}
                icon={providerType.icon}
                description={providerType.description}
                enabled={provider?.enabled || false}
                configured={!!provider}
                onConfigure={() => setConfigureModal(providerType.type)}
              />
            )
          })}
        </div>

        {/* Configuration Modals */}
        <Modal
          open={configureModal === 'local'}
          onClose={() => setConfigureModal(null)}
          title="Local Authentication Settings"
        >
          <LocalSettingsForm onSuccess={() => setConfigureModal(null)} />
        </Modal>

        <Modal
          open={configureModal === 'invite'}
          onClose={() => setConfigureModal(null)}
          title="Email Invitation Settings"
        >
          <InviteSettingsForm onSuccess={() => setConfigureModal(null)} />
        </Modal>

        <Modal
          open={configureModal === 'oidc'}
          onClose={() => setConfigureModal(null)}
          title="Configure OpenID Connect"
          size="large"
        >
          <OIDCConfigForm onSuccess={() => setConfigureModal(null)} />
        </Modal>

        <Modal
          open={configureModal === 'saml'}
          onClose={() => setConfigureModal(null)}
          title="Configure SAML 2.0"
          size="large"
        >
          <SAMLConfigForm onSuccess={() => setConfigureModal(null)} />
        </Modal>

        <Modal
          open={configureModal === 'ldap'}
          onClose={() => setConfigureModal(null)}
          title="Configure LDAP / Active Directory"
          size="large"
        >
          <LDAPConfigForm onSuccess={() => setConfigureModal(null)} />
        </Modal>
      </div>
    </PermissionGuard>
  )
}
```

### Provider Card Component

**Location:** `src/components/auth-providers/ProviderCard.tsx`

```tsx
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { authProvidersApi } from '@/lib/api/authProviders'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { Card } from '@/components/ui/Card'
import { Switch } from '@/components/ui/Switch'
import { Settings, CheckCircle, XCircle } from 'lucide-react'
import { toast } from '@/components/ui/use-toast'

interface ProviderCardProps {
  type: string
  name: string
  icon: string
  description: string
  enabled: boolean
  configured: boolean
  onConfigure: () => void
}

export function ProviderCard({
  type,
  name,
  icon,
  description,
  enabled,
  configured,
  onConfigure,
}: ProviderCardProps) {
  const queryClient = useQueryClient()

  const toggleMutation = useMutation({
    mutationFn: (enable: boolean) =>
      enable
        ? authProvidersApi.enable(type)
        : authProvidersApi.disable(type),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['auth-providers'] })
      toast({ title: `Provider ${enabled ? 'disabled' : 'enabled'}` })
    },
  })

  const handleToggle = (checked: boolean) => {
    // Local auth cannot be disabled
    if (type === 'local' && !checked) {
      toast({
        title: 'Cannot disable local authentication',
        variant: 'destructive',
      })
      return
    }

    // Must be configured before enabling
    if (checked && !configured && type !== 'local' && type !== 'invite') {
      toast({
        title: 'Please configure the provider first',
        variant: 'destructive',
      })
      return
    }

    toggleMutation.mutate(checked)
  }

  return (
    <Card className="p-6">
      <div className="flex items-start justify-between mb-4">
        <div className="flex items-center gap-3">
          <div className="w-12 h-12 bg-blue-100 rounded-lg flex items-center justify-center">
            {/* Icon */}
          </div>
          <div>
            <h3 className="font-semibold">{name}</h3>
            <p className="text-sm text-gray-600">{description}</p>
          </div>
        </div>

        <Switch
          checked={enabled}
          onCheckedChange={handleToggle}
          disabled={type === 'local' || toggleMutation.isPending}
        />
      </div>

      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          {configured ? (
            <>
              <CheckCircle className="w-4 h-4 text-green-500" />
              <span className="text-sm text-green-600">Configured</span>
            </>
          ) : (
            <>
              <XCircle className="w-4 h-4 text-gray-400" />
              <span className="text-sm text-gray-500">Not configured</span>
            </>
          )}
        </div>

        <Button
          size="sm"
          variant="outline"
          onClick={onConfigure}
        >
          <Settings className="w-4 h-4 mr-2" />
          Configure
        </Button>
      </div>

      {enabled && (
        <Badge variant="success" className="mt-4">
          Active
        </Badge>
      )}
    </Card>
  )
}
```

### OIDC Configuration Form

**Location:** `src/components/auth-providers/OIDCConfigForm.tsx`

```tsx
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { authProvidersApi } from '@/lib/api/authProviders'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import { toast } from '@/components/ui/use-toast'

const oidcSchema = z.object({
  issuer: z.string().url('Must be a valid URL'),
  client_id: z.string().min(1, 'Client ID is required'),
  client_secret: z.string().min(1, 'Client secret is required'),
  redirect_url: z.string().url('Must be a valid URL'),
  scopes: z.string().optional(),
  enabled: z.boolean().default(false),
})

type OIDCFormData = z.infer<typeof oidcSchema>

interface OIDCConfigFormProps {
  onSuccess: () => void
}

export function OIDCConfigForm({ onSuccess }: OIDCConfigFormProps) {
  const queryClient = useQueryClient()

  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<OIDCFormData>({
    resolver: zodResolver(oidcSchema),
    defaultValues: {
      redirect_url: `${window.location.origin}/api/auth/oidc/callback`,
      scopes: 'openid profile email',
    },
  })

  const mutation = useMutation({
    mutationFn: authProvidersApi.configureOIDC,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['auth-providers'] })
      toast({ title: 'OIDC provider configured successfully' })
      onSuccess()
    },
  })

  const onSubmit = async (data: OIDCFormData) => {
    await mutation.mutateAsync({
      issuer: data.issuer,
      client_id: data.client_id,
      client_secret: data.client_secret,
      redirect_url: data.redirect_url,
      scopes: data.scopes?.split(' ') || [],
      enabled: data.enabled,
    })
  }

  return (
    <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
      <Input
        {...register('issuer')}
        label="Issuer URL"
        placeholder="https://accounts.google.com"
        error={errors.issuer?.message}
        helpText="The OIDC provider's issuer URL"
      />

      <Input
        {...register('client_id')}
        label="Client ID"
        placeholder="your-client-id"
        error={errors.client_id?.message}
      />

      <Input
        {...register('client_secret')}
        type="password"
        label="Client Secret"
        placeholder="your-client-secret"
        error={errors.client_secret?.message}
      />

      <Input
        {...register('redirect_url')}
        label="Redirect URL"
        error={errors.redirect_url?.message}
        helpText="Copy this URL to your OIDC provider's configuration"
      />

      <Input
        {...register('scopes')}
        label="Scopes"
        placeholder="openid profile email"
        error={errors.scopes?.message}
        helpText="Space-separated list of scopes"
      />

      <div className="flex items-center gap-2">
        <input
          type="checkbox"
          {...register('enabled')}
          id="enabled"
          className="rounded"
        />
        <label htmlFor="enabled" className="text-sm">
          Enable this provider immediately
        </label>
      </div>

      <div className="flex justify-end gap-2">
        <Button type="button" variant="outline" onClick={onSuccess}>
          Cancel
        </Button>
        <Button type="submit" loading={mutation.isPending}>
          Save Configuration
        </Button>
      </div>
    </form>
  )
}
```

### Auth Providers API

**Location:** `src/lib/api/authProviders.ts`

```tsx
import { apiClient } from './client'

export const authProvidersApi = {
  // Get enabled providers (for login page)
  getEnabled: async () => {
    const response = await apiClient.get('/auth/providers')
    return response.data.data
  },

  // Get all providers (admin)
  getAll: async () => {
    const response = await apiClient.get('/auth/providers/all')
    return response.data.data
  },

  // Configure OIDC
  configureOIDC: async (config: {
    issuer: string
    client_id: string
    client_secret: string
    redirect_url: string
    scopes: string[]
    enabled: boolean
  }) => {
    const response = await apiClient.post('/auth/providers/oidc', config)
    return response.data
  },

  // Configure SAML
  configureSAML: async (config: {
    metadata_url: string
    entity_id: string
    sso_url: string
    certificate: string
    private_key: string
    attribute_mapping: Record<string, string>
    enabled: boolean
  }) => {
    const response = await apiClient.post('/auth/providers/saml', config)
    return response.data
  },

  // Configure LDAP
  configureLDAP: async (config: {
    host: string
    port: number
    base_dn: string
    bind_dn: string
    bind_password: string
    user_filter: string
    use_tls: boolean
    skip_verify: boolean
    attribute_mapping: Record<string, string>
    enabled: boolean
  }) => {
    const response = await apiClient.post('/auth/providers/ldap', config)
    return response.data
  },

  // Update local settings
  updateLocal: async (allow_registration: boolean) => {
    const response = await apiClient.put('/auth/providers/local', {
      allow_registration,
    })
    return response.data
  },

  // Update invite settings
  updateInvite: async (enabled: boolean, require_email_verification: boolean) => {
    const response = await apiClient.put('/auth/providers/invite', {
      enabled,
      require_email_verification,
    })
    return response.data
  },

  // Enable provider
  enable: async (type: string) => {
    const response = await apiClient.put(`/auth/providers/${type}/enable`)
    return response.data
  },

  // Disable provider
  disable: async (type: string) => {
    const response = await apiClient.put(`/auth/providers/${type}/disable`)
    return response.data
  },

  // Test connection
  testConnection: async (type: string) => {
    const response = await apiClient.post(`/auth/providers/${type}/test`)
    return response.data
  },

  // Delete provider
  delete: async (type: string) => {
    const response = await apiClient.delete(`/auth/providers/${type}`)
    return response.data
  },
}
```

### useAuthProviders Hook

**Location:** `src/hooks/useAuthProviders.ts`

```tsx
import { useQuery } from '@tanstack/react-query'
import { authProvidersApi } from '@/lib/api/authProviders'
import { usePermissions } from './usePermissions'

export function useAuthProviders() {
  const { hasPermission } = usePermissions()

  // Admins get all providers, regular users get only enabled ones
  const isAdmin = hasPermission('permission.manage')

  return useQuery({
    queryKey: ['auth-providers', isAdmin ? 'all' : 'enabled'],
    queryFn: isAdmin ? authProvidersApi.getAll : authProvidersApi.getEnabled,
  })
}
```

---

## Custom Hooks

### useAuth Hook

**Location:** `src/hooks/useAuth.ts`

```tsx
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { authApi } from '@/lib/api/auth'
import { useAuthStore } from '@/store/authStore'
import { toast } from '@/components/ui/use-toast'

export function useAuth() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const { accessToken, setTokens, clearTokens, isAuthenticated } = useAuthStore()

  const loginMutation = useMutation({
    mutationFn: ({ username, password }: { username: string; password: string }) =>
      authApi.login(username, password),
    onSuccess: (data) => {
      setTokens(data.access_token, data.refresh_token)
      navigate('/dashboard')
      toast({ title: 'Login successful' })
    },
    onError: (error: any) => {
      toast({
        title: 'Login failed',
        description: error.response?.data?.error?.message || 'Invalid credentials',
        variant: 'destructive',
      })
    },
  })

  const logoutMutation = useMutation({
    mutationFn: authApi.logout,
    onSuccess: () => {
      clearTokens()
      queryClient.clear()
      navigate('/login')
      toast({ title: 'Logged out successfully' })
    },
  })

  return {
    login: (username: string, password: string) =>
      loginMutation.mutateAsync({ username, password }),
    logout: () => logoutMutation.mutate(),
    isLoading: loginMutation.isPending,
    isAuthenticated,
    accessToken,
  }
}
```

### usePermissions Hook

**Location:** `src/hooks/usePermissions.ts`

```tsx
import { useQuery } from '@tanstack/react-query'
import { permissionsApi } from '@/lib/api/permissions'
import { useCurrentUser } from './useCurrentUser'

export function usePermissions() {
  const { data: user } = useCurrentUser()

  const { data: permissions = [] } = useQuery({
    queryKey: ['permissions', 'my'],
    queryFn: permissionsApi.getMyPermissions,
    enabled: !!user,
  })

  const hasPermission = (permissionId: string): boolean => {
    // Root user has all permissions
    if (user?.is_root) {
      return true
    }

    return permissions.includes(permissionId)
  }

  const hasAnyPermission = (permissionIds: string[]): boolean => {
    return permissionIds.some((id) => hasPermission(id))
  }

  const hasAllPermissions = (permissionIds: string[]): boolean => {
    return permissionIds.every((id) => hasPermission(id))
  }

  return {
    permissions,
    hasPermission,
    hasAnyPermission,
    hasAllPermissions,
  }
}
```

### useCurrentUser Hook

**Location:** `src/hooks/useCurrentUser.ts`

```tsx
import { useQuery } from '@tanstack/react-query'
import { authApi } from '@/lib/api/auth'
import { useAuthStore } from '@/store/authStore'

export function useCurrentUser() {
  const { isAuthenticated } = useAuthStore()

  return useQuery({
    queryKey: ['user', 'me'],
    queryFn: authApi.getCurrentUser,
    enabled: isAuthenticated,
    staleTime: 1000 * 60 * 5, // 5 minutes
  })
}
```

### useUsers Hook

**Location:** `src/hooks/useUsers.ts`

```tsx
import { useQuery } from '@tanstack/react-query'
import { usersApi } from '@/lib/api/users'

interface UseUsersOptions {
  page?: number
  perPage?: number
  filters?: Record<string, any>
}

export function useUsers({ page = 1, perPage = 20, filters = {} }: UseUsersOptions = {}) {
  return useQuery({
    queryKey: ['users', { page, perPage, filters }],
    queryFn: () => usersApi.list(page, perPage, filters),
  })
}

export function useUser(id: string) {
  return useQuery({
    queryKey: ['users', id],
    queryFn: () => usersApi.getById(id),
    enabled: !!id,
  })
}
```

---

## State Management

### Auth Store

**Location:** `src/store/authStore.ts`

```tsx
import { create } from 'zustand'
import { persist } from 'zustand/middleware'

interface AuthState {
  accessToken: string | null
  refreshToken: string | null
  isAuthenticated: boolean
  setTokens: (accessToken: string, refreshToken: string) => void
  clearTokens: () => void
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      accessToken: null,
      refreshToken: null,
      isAuthenticated: false,

      setTokens: (accessToken, refreshToken) =>
        set({
          accessToken,
          refreshToken,
          isAuthenticated: true,
        }),

      clearTokens: () =>
        set({
          accessToken: null,
          refreshToken: null,
          isAuthenticated: false,
        }),
    }),
    {
      name: 'auth-storage',
      partialize: (state) => ({
        // Only persist refresh token, not access token (security)
        refreshToken: state.refreshToken,
      }),
    }
  )
)
```

### Settings Store

**Location:** `src/store/settingsStore.ts`

**CRITICAL:** Never hardcode user preferences. All settings must be configurable.

```tsx
import { create } from 'zustand'
import { persist } from 'zustand/middleware'

interface SettingsState {
  // Theme
  theme: 'light' | 'dark' | 'system'
  setTheme: (theme: 'light' | 'dark' | 'system') => void

  // Terminal preferences (for future terminal module)
  terminalFontSize: number
  terminalFontFamily: string
  terminalTheme: string
  setTerminalPreferences: (prefs: Partial<TerminalPreferences>) => void

  // UI preferences
  sidebarCollapsed: boolean
  setSidebarCollapsed: (collapsed: boolean) => void

  // Pagination defaults
  defaultPageSize: number
  setDefaultPageSize: (size: number) => void
}

interface TerminalPreferences {
  fontSize: number
  fontFamily: string
  theme: string
}

export const useSettingsStore = create<SettingsState>()(
  persist(
    (set) => ({
      // Theme
      theme: 'system',
      setTheme: (theme) => set({ theme }),

      // Terminal
      terminalFontSize: 14,
      terminalFontFamily: 'monospace',
      terminalTheme: 'default',
      setTerminalPreferences: (prefs) =>
        set((state) => ({
          terminalFontSize: prefs.fontSize ?? state.terminalFontSize,
          terminalFontFamily: prefs.fontFamily ?? state.terminalFontFamily,
          terminalTheme: prefs.theme ?? state.terminalTheme,
        })),

      // UI
      sidebarCollapsed: false,
      setSidebarCollapsed: (collapsed) => set({ sidebarCollapsed: collapsed }),

      // Pagination
      defaultPageSize: 20,
      setDefaultPageSize: (size) => set({ defaultPageSize: size }),
    }),
    {
      name: 'settings-storage',
    }
  )
)
```

---

## API Integration

### API Client

**Location:** `src/lib/api/client.ts`

```tsx
import axios from 'axios'
import { useAuthStore } from '@/store/authStore'

const API_BASE_URL = import.meta.env.VITE_API_URL || 'http://localhost:8000/api'

export const apiClient = axios.create({
  baseURL: API_BASE_URL,
  headers: {
    'Content-Type': 'application/json',
  },
})

// Request interceptor - attach access token
apiClient.interceptors.request.use(
  (config) => {
    const { accessToken } = useAuthStore.getState()

    if (accessToken) {
      config.headers.Authorization = `Bearer ${accessToken}`
    }

    return config
  },
  (error) => Promise.reject(error)
)

// Response interceptor - handle token refresh
apiClient.interceptors.response.use(
  (response) => response,
  async (error) => {
    const originalRequest = error.config

    // If 401 and not already retried, try to refresh token
    if (error.response?.status === 401 && !originalRequest._retry) {
      originalRequest._retry = true

      try {
        const { refreshToken, setTokens, clearTokens } = useAuthStore.getState()

        if (!refreshToken) {
          clearTokens()
          window.location.href = '/login'
          return Promise.reject(error)
        }

        // Refresh token
        const response = await axios.post(`${API_BASE_URL}/auth/refresh`, {
          refresh_token: refreshToken,
        })

        const { access_token, refresh_token } = response.data.data
        setTokens(access_token, refresh_token)

        // Retry original request with new token
        originalRequest.headers.Authorization = `Bearer ${access_token}`
        return apiClient(originalRequest)
      } catch (refreshError) {
        // Refresh failed, logout
        useAuthStore.getState().clearTokens()
        window.location.href = '/login'
        return Promise.reject(refreshError)
      }
    }

    return Promise.reject(error)
  }
)
```

### Auth API

**Location:** `src/lib/api/auth.ts`

```tsx
import { apiClient } from './client'
import type { User } from '@/types/user'

export const authApi = {
  // Check if setup is needed
  checkSetup: async () => {
    const response = await apiClient.get('/setup/check')
    return response.data.data
  },

  // Complete first-time setup
  completeSetup: async (data: {
    username: string
    email: string
    password: string
    first_name?: string
    last_name?: string
  }) => {
    const response = await apiClient.post('/setup/complete', data)
    return response.data.data
  },

  // Login
  login: async (username: string, password: string) => {
    const response = await apiClient.post('/auth/login', { username, password })
    return response.data.data
  },

  // Logout
  logout: async () => {
    const response = await apiClient.post('/auth/logout')
    return response.data
  },

  // Get current user
  getCurrentUser: async (): Promise<User> => {
    const response = await apiClient.get('/auth/me')
    return response.data.data
  },

  // MFA setup
  setupMFA: async () => {
    const response = await apiClient.post('/auth/mfa/setup')
    return response.data.data
  },

  // Verify MFA
  verifyMFA: async (code: string) => {
    const response = await apiClient.post('/auth/mfa/verify', { code })
    return response.data.data
  },

  // Disable MFA
  disableMFA: async () => {
    const response = await apiClient.post('/auth/mfa/disable')
    return response.data
  },

  // Password reset request
  requestPasswordReset: async (email: string) => {
    const response = await apiClient.post('/auth/password-reset/request', { email })
    return response.data
  },

  // Password reset confirm
  confirmPasswordReset: async (token: string, password: string) => {
    const response = await apiClient.post('/auth/password-reset/confirm', {
      token,
      password,
    })
    return response.data
  },
}
```

### Users API

**Location:** `src/lib/api/users.ts`

```tsx
import { apiClient } from './client'
import type { User } from '@/types/user'

export const usersApi = {
  // List users
  list: async (page: number, perPage: number, filters: Record<string, any>) => {
    const response = await apiClient.get('/users', {
      params: { page, per_page: perPage, ...filters },
    })
    return response.data.data
  },

  // Get user by ID
  getById: async (id: string): Promise<User> => {
    const response = await apiClient.get(`/users/${id}`)
    return response.data.data
  },

  // Create user
  create: async (data: Partial<User>) => {
    const response = await apiClient.post('/users', data)
    return response.data.data
  },

  // Update user
  update: async (id: string, data: Partial<User>) => {
    const response = await apiClient.put(`/users/${id}`, data)
    return response.data.data
  },

  // Delete user
  delete: async (id: string) => {
    const response = await apiClient.delete(`/users/${id}`)
    return response.data
  },

  // Activate user
  activate: async (id: string) => {
    const response = await apiClient.post(`/users/${id}/activate`)
    return response.data
  },

  // Deactivate user
  deactivate: async (id: string) => {
    const response = await apiClient.post(`/users/${id}/deactivate`)
    return response.data
  },
}
```

### Permissions API

**Location:** `src/lib/api/permissions.ts`

```tsx
import { apiClient } from './client'
import type { Permission } from '@/types/permission'

export const permissionsApi = {
  // Get all permissions
  getAll: async (): Promise<Permission[]> => {
    const response = await apiClient.get('/permissions')
    return response.data.data
  },

  // Get my permissions
  getMyPermissions: async (): Promise<string[]> => {
    const response = await apiClient.get('/permissions/my')
    return response.data.data
  },

  // Get role permissions
  getRolePermissions: async (roleId: string): Promise<Permission[]> => {
    const response = await apiClient.get(`/permissions/roles/${roleId}`)
    return response.data.data
  },

  // Assign permission to role
  assignToRole: async (roleId: string, permissionId: string) => {
    const response = await apiClient.post(`/permissions/roles/${roleId}`, {
      permission_id: permissionId,
    })
    return response.data
  },

  // Remove permission from role
  removeFromRole: async (roleId: string, permissionId: string) => {
    const response = await apiClient.delete(`/permissions/roles/${roleId}/${permissionId}`)
    return response.data
  },
}
```

---

## Routing & Navigation

### Router Configuration

**Location:** `src/router.tsx`

```tsx
import { createBrowserRouter, redirect } from 'react-router-dom'
import { AuthLayout } from '@/components/layout/AuthLayout'
import { DashboardLayout } from '@/components/layout/DashboardLayout'
import { Login } from '@/pages/auth/Login'
import { Setup } from '@/pages/auth/Setup'
import { PasswordReset } from '@/pages/auth/PasswordReset'
import { Dashboard } from '@/pages/dashboard/Dashboard'
import { Users } from '@/pages/settings/Users'
import { Teams } from '@/pages/settings/Teams'
import { Permissions } from '@/pages/settings/Permissions'
import { Security } from '@/pages/settings/Security'
import { Sessions } from '@/pages/settings/Sessions'
import { AuditLogs } from '@/pages/settings/AuditLogs'
import { AuthProviders } from '@/pages/settings/AuthProviders'
import { authApi } from '@/lib/api/auth'
import { useAuthStore } from '@/store/authStore'

// Setup check loader
async function setupLoader() {
  const { setup_needed } = await authApi.checkSetup()

  if (setup_needed) {
    return null // Allow access to setup page
  }

  // Setup already completed, redirect to login
  return redirect('/login')
}

// Auth check loader
function authLoader() {
  const { isAuthenticated } = useAuthStore.getState()

  if (!isAuthenticated) {
    return redirect('/login')
  }

  return null
}

export const router = createBrowserRouter([
  {
    path: '/',
    element: <AuthLayout />,
    children: [
      {
        index: true,
        loader: () => redirect('/login'),
      },
      {
        path: 'login',
        element: <Login />,
      },
      {
        path: 'setup',
        element: <Setup />,
        loader: setupLoader,
      },
      {
        path: 'password-reset',
        element: <PasswordReset />,
      },
    ],
  },
  {
    path: '/',
    element: <DashboardLayout />,
    loader: authLoader,
    children: [
      {
        path: 'dashboard',
        element: <Dashboard />,
      },
      {
        path: 'settings',
        children: [
          {
            path: 'users',
            element: <Users />,
          },
          {
            path: 'teams',
            element: <Teams />,
          },
          {
            path: 'permissions',
            element: <Permissions />,
          },
          {
            path: 'security',
            element: <Security />,
          },
          {
            path: 'sessions',
            element: <Sessions />,
          },
          {
            path: 'audit',
            element: <AuditLogs />,
          },
          {
            path: 'auth-providers',
            element: <AuthProviders />,
          },
        ],
      },
    ],
  },
])
```

---

## Security Implementation

### Token Storage

**Best Practices:**

1. **Access Token:** Store in memory (Zustand state, not persisted)
2. **Refresh Token:** Store in localStorage via Zustand persist
3. **Never store tokens in cookies from frontend** (backend should set httpOnly cookies if using cookie-based auth)

### CSRF Protection

If backend implements CSRF tokens:

```tsx
// In API client
apiClient.interceptors.request.use((config) => {
  const csrfToken = document.cookie
    .split('; ')
    .find((row) => row.startsWith('csrf_token='))
    ?.split('=')[1]

  if (csrfToken) {
    config.headers['X-CSRF-Token'] = csrfToken
  }

  return config
})
```

### Input Sanitization

Always validate and sanitize user input:

```tsx
import DOMPurify from 'dompurify'

// Sanitize HTML content
const sanitizeHTML = (html: string) => {
  return DOMPurify.sanitize(html)
}

// Use in components
<div dangerouslySetInnerHTML={{ __html: sanitizeHTML(userContent) }} />
```

### XSS Prevention

- Use React's built-in XSS protection (JSX escaping)
- Never use `dangerouslySetInnerHTML` with unsanitized content
- Validate all user input with Zod schemas

### Permission-Based Rendering

Always check permissions before rendering sensitive UI:

```tsx
import { PermissionGuard } from '@/components/permissions/PermissionGuard'

// Hide entire component
<PermissionGuard permission="user.delete">
  <DeleteButton />
</PermissionGuard>

// Disable button
<Button
  disabled={!hasPermission('user.delete')}
  onClick={handleDelete}
>
  Delete
</Button>
```

---

## Testing Strategy

### Unit Tests (Vitest + React Testing Library)

**Example:** `src/components/auth/LoginForm.test.tsx`

```tsx
import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { Login } from '@/pages/auth/Login'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: { retry: false },
  },
})

const wrapper = ({ children }: { children: React.ReactNode }) => (
  <QueryClientProvider client={queryClient}>
    {children}
  </QueryClientProvider>
)

describe('Login', () => {
  it('renders login form', () => {
    render(<Login />, { wrapper })

    expect(screen.getByLabelText(/username/i)).toBeInTheDocument()
    expect(screen.getByLabelText(/password/i)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /sign in/i })).toBeInTheDocument()
  })

  it('shows validation errors for empty fields', async () => {
    render(<Login />, { wrapper })

    const submitButton = screen.getByRole('button', { name: /sign in/i })
    fireEvent.click(submitButton)

    await waitFor(() => {
      expect(screen.getByText(/username is required/i)).toBeInTheDocument()
      expect(screen.getByText(/password is required/i)).toBeInTheDocument()
    })
  })

  it('calls login API on form submit', async () => {
    const mockLogin = vi.fn()

    render(<Login />, { wrapper })

    fireEvent.change(screen.getByLabelText(/username/i), {
      target: { value: 'testuser' },
    })
    fireEvent.change(screen.getByLabelText(/password/i), {
      target: { value: 'password123' },
    })

    fireEvent.click(screen.getByRole('button', { name: /sign in/i }))

    await waitFor(() => {
      expect(mockLogin).toHaveBeenCalledWith('testuser', 'password123')
    })
  })
})
```

### Component Tests

**Example:** `src/components/permissions/PermissionGuard.test.tsx`

```tsx
import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { PermissionGuard } from '@/components/permissions/PermissionGuard'
import { usePermissions } from '@/hooks/usePermissions'

vi.mock('@/hooks/usePermissions')

describe('PermissionGuard', () => {
  it('renders children when user has permission', () => {
    vi.mocked(usePermissions).mockReturnValue({
      hasPermission: () => true,
      permissions: ['user.view'],
      hasAnyPermission: () => true,
      hasAllPermissions: () => true,
    })

    render(
      <PermissionGuard permission="user.view">
        <div>Protected Content</div>
      </PermissionGuard>
    )

    expect(screen.getByText('Protected Content')).toBeInTheDocument()
  })

  it('does not render children when user lacks permission', () => {
    vi.mocked(usePermissions).mockReturnValue({
      hasPermission: () => false,
      permissions: [],
      hasAnyPermission: () => false,
      hasAllPermissions: () => false,
    })

    render(
      <PermissionGuard permission="user.delete">
        <div>Protected Content</div>
      </PermissionGuard>
    )

    expect(screen.queryByText('Protected Content')).not.toBeInTheDocument()
  })

  it('renders fallback when provided', () => {
    vi.mocked(usePermissions).mockReturnValue({
      hasPermission: () => false,
      permissions: [],
      hasAnyPermission: () => false,
      hasAllPermissions: () => false,
    })

    render(
      <PermissionGuard
        permission="user.delete"
        fallback={<div>No Permission</div>}
      >
        <div>Protected Content</div>
      </PermissionGuard>
    )

    expect(screen.getByText('No Permission')).toBeInTheDocument()
  })
})
```

### E2E Tests (Cypress)

**Example:** `cypress/e2e/auth/login.cy.ts`

```typescript
describe('Login Flow', () => {
  beforeEach(() => {
    cy.visit('/login')
  })

  it('successfully logs in with valid credentials', () => {
    cy.get('input[name="username"]').type('admin')
    cy.get('input[name="password"]').type('password123')
    cy.get('button[type="submit"]').click()

    cy.url().should('include', '/dashboard')
    cy.contains('Welcome').should('be.visible')
  })

  it('shows error with invalid credentials', () => {
    cy.get('input[name="username"]').type('invalid')
    cy.get('input[name="password"]').type('wrong')
    cy.get('button[type="submit"]').click()

    cy.contains('Invalid credentials').should('be.visible')
  })

  it('redirects to setup when no users exist', () => {
    // Mock API response
    cy.intercept('GET', '/api/setup/check', {
      body: { data: { setup_needed: true } },
    })

    cy.visit('/login')
    cy.url().should('include', '/setup')
  })
})
```

### Storybook Stories

**Example:** `src/components/ui/Button.stories.tsx`

```tsx
import type { Meta, StoryObj } from '@storybook/react'
import { Button } from './Button'

const meta: Meta<typeof Button> = {
  title: 'UI/Button',
  component: Button,
  tags: ['autodocs'],
}

export default meta
type Story = StoryObj<typeof Button>

export const Primary: Story = {
  args: {
    children: 'Button',
    variant: 'primary',
  },
}

export const Secondary: Story = {
  args: {
    children: 'Button',
    variant: 'secondary',
  },
}

export const Loading: Story = {
  args: {
    children: 'Button',
    loading: true,
  },
}

export const Disabled: Story = {
  args: {
    children: 'Button',
    disabled: true,
  },
}
```

---

## Implementation Checklist

### Phase 1: Project Setup (Week 1)

- [ ] **Initialize Project**
  - [ ] Create Vite project with React + TypeScript
  - [ ] Configure Tailwind CSS v4
  - [ ] Setup ESLint and Prettier
  - [ ] Configure path aliases (@/)
  - [ ] Setup Git hooks with Husky

- [ ] **Install Dependencies**
  - [ ] React Router 7
  - [ ] TanStack Query
  - [ ] Zustand
  - [ ] react-hook-form + Zod
  - [ ] Axios
  - [ ] Radix UI components
  - [ ] Lucide React icons
  - [ ] class-variance-authority

- [ ] **Project Structure**
  - [ ] Create directory structure
  - [ ] Setup base configuration files
  - [ ] Create environment variables template

### Phase 2: Base UI Components (Week 2)

- [ ] **Design System**
  - [ ] Button component with variants
  - [ ] Input component
  - [ ] Select component
  - [ ] Modal/Dialog component
  - [ ] Card component
  - [ ] Badge component
  - [ ] Tabs component
  - [ ] Toast/notification system
  - [ ] Loading spinner
  - [ ] Skeleton loaders

- [ ] **Layout Components**
  - [ ] AuthLayout
  - [ ] DashboardLayout
  - [ ] Sidebar
  - [ ] Header
  - [ ] Footer

### Phase 3: Authentication & Setup (Week 3)

- [ ] **State Management**
  - [ ] Auth store (Zustand)
  - [ ] Settings store (Zustand)
  - [ ] Feature flags store

- [ ] **API Client**
  - [ ] Axios instance with interceptors
  - [ ] Token refresh logic
  - [ ] Error handling
  - [ ] Auth API module

- [ ] **Authentication Pages**
  - [ ] Login page
  - [ ] Setup wizard page
  - [ ] Password reset page
  - [ ] MFA setup page

- [ ] **Auth Components**
  - [ ] LoginForm
  - [ ] PasswordStrengthMeter
  - [ ] MFAVerification
  - [ ] SSOButtons (placeholder)

- [ ] **Hooks**
  - [ ] useAuth hook
  - [ ] useCurrentUser hook

- [ ] **Routing**
  - [ ] Router configuration
  - [ ] Route guards
  - [ ] Setup check loader

### Phase 4: User Management (Week 4)

- [ ] **API Modules**
  - [ ] Users API
  - [ ] Teams API

- [ ] **User Management Pages**
  - [ ] Users list page
  - [ ] User detail page
  - [ ] Teams page

- [ ] **User Components**
  - [ ] UserTable
  - [ ] UserForm
  - [ ] UserFilters
  - [ ] UserDetailModal

- [ ] **Hooks**
  - [ ] useUsers hook
  - [ ] useUser hook
  - [ ] useTeams hook

- [ ] **Schemas**
  - [ ] User validation schema
  - [ ] Team validation schema

### Phase 5: Permission System (Week 5)

- [ ] **API Modules**
  - [ ] Permissions API

- [ ] **Permission Pages**
  - [ ] Permissions management page
  - [ ] Role management page

- [ ] **Permission Components**
  - [ ] PermissionGuard
  - [ ] PermissionMatrix
  - [ ] RoleManager
  - [ ] PermissionBadge

- [ ] **Hooks**
  - [ ] usePermissions hook

- [ ] **Integration**
  - [ ] Add permission guards to all routes
  - [ ] Add permission checks to UI elements
  - [ ] Update sidebar with permission-based visibility

### Phase 6: Session & Audit (Week 6)

- [ ] **API Modules**
  - [ ] Sessions API
  - [ ] Audit API
  - [ ] Auth Providers API

- [ ] **Pages**
-  - [ ] Sessions settings tab
  - [ ] Audit logs page
  - [ ] Security settings page
  - [ ] Auth providers page

- [ ] **Components**
-  - [ ] ProfileSessionsPanel
  - [ ] AuditLogTable
  - [ ] AuditFilters
  - [ ] AuditExport
  - [ ] ProviderCard
  - [ ] OIDCConfigForm
  - [ ] SAMLConfigForm
  - [ ] LDAPConfigForm
  - [ ] LocalSettingsForm
  - [ ] InviteSettingsForm

- [ ] **Hooks**
  - [ ] useProfileSessions hook
  - [ ] useAuditLogs hook
  - [ ] useAuthProviders hook

### Phase 7: Dashboard & Analytics (Week 7)

- [ ] **Dashboard Page**
  - [ ] Dashboard layout
  - [ ] KPI cards
  - [ ] Charts (active users, login attempts)
  - [ ] Recent activity feed

- [ ] **Components**
  - [ ] StatCard
  - [ ] ActivityFeed
  - [ ] Charts (using Recharts)

### Phase 8: Testing & Polish (Week 8)

- [ ] **Unit Tests**
  - [ ] Component tests (≥80% coverage)
  - [ ] Hook tests
  - [ ] Utility function tests

- [ ] **Integration Tests**
  - [ ] API integration tests
  - [ ] Form submission tests

- [ ] **E2E Tests**
  - [ ] Login flow
  - [ ] Setup wizard flow
  - [ ] User CRUD operations
  - [ ] Permission assignment
  - [ ] MFA enrollment

- [ ] **Storybook**
  - [ ] Stories for all UI components
  - [ ] Stories for auth components
  - [ ] Stories for admin components

- [ ] **Accessibility**
  - [ ] Keyboard navigation
  - [ ] ARIA attributes
  - [ ] Focus management
  - [ ] Screen reader testing

- [ ] **Performance**
  - [ ] Code splitting
  - [ ] Lazy loading
  - [ ] Image optimization
  - [ ] Bundle size optimization
  - [ ] Lighthouse audit

- [ ] **Documentation**
  - [ ] Component documentation
  - [ ] API documentation
  - [ ] User guide
  - [ ] Developer guide

---

## TypeScript Types

### Core Types

**Location:** `src/types/user.ts`

```tsx
export interface User {
  id: string
  username: string
  email: string
  first_name?: string
  last_name?: string
  avatar?: string
  is_root: boolean
  is_active: boolean
  mfa_enabled: boolean
  teams?: Team[]
  roles?: Role[]
  created_at: string
  updated_at: string
  last_login_at?: string
  last_login_ip?: string
}
```

**Location:** `src/types/permission.ts`

```tsx
export interface Permission {
  id: string
  module: string
  description: string
  depends_on?: string[]
  created_at: string
}

export interface Role {
  id: string
  name: string
  description: string
  is_system: boolean
  permissions?: Permission[]
  created_at: string
  updated_at: string
}
```

**Location:** `src/types/session.ts`

```tsx
export interface Session {
  id: string
  user_id: string
  ip_address: string
  user_agent: string
  device_name?: string
  expires_at: string
  last_used_at: string
  created_at: string
  revoked_at?: string
}
```

**Location:** `src/types/audit.ts`

```tsx
export interface AuditLog {
  id: string
  user_id?: string
  user?: User
  username: string
  action: string
  resource: string
  result: 'success' | 'failure'
  ip_address: string
  user_agent: string
  metadata?: Record<string, any>
  created_at: string
}
```

**Location:** `src/types/api.ts`

```tsx
export interface APIResponse<T> {
  success: boolean
  data?: T
  error?: {
    code: string
    message: string
  }
  meta?: {
    page?: number
    per_page?: number
    total?: number
    total_pages?: number
  }
}

export interface PaginatedResponse<T> {
  items: T[]
  total: number
  page: number
  per_page: number
  total_pages: number
}
```

---

## Dependencies

### package.json

```json
{
  "name": "shellcn-web",
  "version": "1.0.0",
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "tsc && vite build",
    "preview": "vite preview",
    "test": "vitest",
    "test:ui": "vitest --ui",
    "test:e2e": "cypress open",
    "lint": "eslint . --ext ts,tsx",
    "format": "prettier --write \"src/**/*.{ts,tsx}\"",
    "storybook": "storybook dev -p 6006",
    "build-storybook": "storybook build"
  },
  "dependencies": {
    "react": "^19.0.0",
    "react-dom": "^19.0.0",
    "react-router-dom": "^7.0.0",
    "@tanstack/react-query": "^5.0.0",
    "@tanstack/react-table": "^8.10.0",
    "zustand": "^4.4.0",
    "react-hook-form": "^7.48.0",
    "@hookform/resolvers": "^3.3.0",
    "zod": "^3.22.0",
    "axios": "^1.6.0",
    "@radix-ui/react-dialog": "^1.0.5",
    "@radix-ui/react-dropdown-menu": "^2.0.6",
    "@radix-ui/react-select": "^2.0.0",
    "@radix-ui/react-tabs": "^1.0.4",
    "@radix-ui/react-tooltip": "^1.0.7",
    "class-variance-authority": "^0.7.0",
    "clsx": "^2.0.0",
    "tailwind-merge": "^2.0.0",
    "lucide-react": "^0.294.0",
    "recharts": "^2.10.0",
    "dompurify": "^3.0.0",
    "date-fns": "^2.30.0"
  },
  "devDependencies": {
    "@types/react": "^19.0.0",
    "@types/react-dom": "^19.0.0",
    "@types/dompurify": "^3.0.0",
    "@vitejs/plugin-react": "^4.2.0",
    "vite": "^7.0.0",
    "typescript": "^5.3.0",
    "tailwindcss": "^4.0.0",
    "autoprefixer": "^10.4.0",
    "postcss": "^8.4.0",
    "eslint": "^8.55.0",
    "@typescript-eslint/eslint-plugin": "^6.15.0",
    "@typescript-eslint/parser": "^6.15.0",
    "prettier": "^3.1.0",
    "vitest": "^1.0.0",
    "@vitest/ui": "^1.0.0",
    "@testing-library/react": "^14.1.0",
    "@testing-library/jest-dom": "^6.1.0",
    "@testing-library/user-event": "^14.5.0",
    "cypress": "^13.6.0",
    "@storybook/react": "^7.6.0",
    "@storybook/react-vite": "^7.6.0",
    "husky": "^8.0.0",
    "lint-staged": "^15.2.0"
  }
}
```

---

## Configuration Files

### vite.config.ts

```typescript
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'path'

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    port: 3000,
    proxy: {
      '/api': {
        target: 'http://localhost:8000',
        changeOrigin: true,
      },
    },
  },
})
```

### Tailwind CSS v4 Configuration (src/index.css)

**Note:** Tailwind CSS v4 uses CSS-based configuration via `@theme` directive instead of `tailwind.config.ts`.

```css
@import 'tailwindcss';

@layer base {
  :root {
    /* Light mode theme variables using OKLCH color space */
    --background: oklch(0.9821 0 0);
    --foreground: oklch(0.3211 0 0);
    --primary: oklch(0.5676 0.2021 283.0838);
    --primary-foreground: oklch(1.0000 0 0);
    /* ... other theme variables */
  }

  .dark {
    /* Dark mode theme variables */
    --background: oklch(0.2303 0.0125 264.2926);
    --foreground: oklch(0.9219 0 0);
    /* ... other theme variables */
  }
}

@theme inline {
  /* Map CSS variables to Tailwind colors */
  --color-background: var(--background);
  --color-foreground: var(--foreground);
  --color-primary: var(--primary);
  /* ... other color mappings */
}
```

---

## Implementation Checklist

### Phase 1: Project Setup & Foundation (Week 1)
- [x] Initialize Vite 7 project with React 19 and TypeScript 5.9+
- [x] Configure Tailwind CSS v4 with custom theme using `@theme` directive in CSS (OKLCH color space)
- [x] Set up ESLint, Prettier, and TypeScript strict mode
- [x] Configure path aliases (@/ for src/)
- [x] Install and configure core dependencies (React Router 7, TanStack Query, Zustand)
- [x] Set up project structure (pages/, components/, hooks/, lib/, store/, types/)
- [x] Create base UI components (Button, Input, Card, Modal, Badge) using Radix UI
- [x] Implement class-variance-authority (CVA) for component variants
- [x] Configure Vitest for unit testing
- [x] Implement light/dark theme support with CSS custom properties

### Phase 2: Authentication & Setup Flow (Week 2)
- [x] Implement auth store (Zustand) with token management
- [x] Create API client (Axios) with interceptors for auth
- [x] Build Login page with form validation (react-hook-form + Zod)
- [x] Implement Setup wizard for first-time initialization
- [x] Create AuthLayout component
- [x] Build SSO provider buttons (OIDC, SAML, LDAP)
- [x] Implement MFA verification flow
- [x] Create Password reset flow
- [x] Build useAuth hook for authentication state
- [x] Implement token refresh logic
- [x] Add logout functionality
- [x] Create ProtectedRoute component
- [x] Write tests for authentication flows

### Phase 3: Dashboard & Layout (Week 3)
- [x] Create DashboardLayout with Sidebar and Header
- [x] Implement responsive navigation
- [x] Build Sidebar with permission-based menu items
- [x] Create Header with user profile dropdown
- [x] Implement Dashboard page with overview widgets
- [x] Build useCurrentUser hook
- [x] Create usePermissions hook
- [x] Implement PermissionGuard component
- [x] Add breadcrumb navigation
- [x] Create notification center UI
- [x] Implement WebSocket connection for real-time notifications
- [x] Write tests for layout components

### Phase 4: User Management (Week 4)
- [ ] Create Users list page with pagination
- [ ] Build UserTable component with TanStack Table
- [ ] Implement UserFilters component
- [ ] Create UserForm for create/edit
- [ ] Build UserDetailModal
- [ ] Implement user activation/deactivation
- [ ] Create password management UI
- [ ] Build useUsers hook with TanStack Query
- [ ] Add user search functionality
- [ ] Implement bulk operations
- [ ] Write tests for user management

### Phase 5: Team Management (Week 5)
- [ ] Implement Teams list page
- [ ] Create TeamForm component
- [ ] Build team member management UI
- [ ] Implement member assignment/removal
- [ ] Build useTeams hook
- [ ] Add hierarchical team view
- [ ] Write tests for team management

### Phase 6: Permission Management (Week 6)
- [ ] Create Permissions page
- [ ] Build PermissionMatrix component
- [ ] Implement RoleManager component
- [ ] Create role creation/editing forms
- [ ] Build permission dependency visualization
- [ ] Implement permission assignment UI
- [ ] Create usePermissions hook for registry
- [ ] Add role-based filtering
- [ ] Build permission search
- [ ] Write tests for permission management

### Phase 7: Auth Provider Administration (Week 7)
- [ ] Create AuthProviders page
- [ ] Build ProviderCard component
- [ ] Implement OIDCConfigForm
- [ ] Create SAMLConfigForm
- [ ] Build LDAPConfigForm
- [ ] Implement LocalSettingsForm
- [ ] Create InviteSettingsForm
- [ ] Build provider enable/disable toggle
- [ ] Implement provider test connection
- [ ] Create useAuthProviders hook
- [ ] Add provider configuration validation
- [ ] Write tests for provider management

### Phase 8: Session Management (Week 8)
- [ ] Create Sessions page
- [ ] Build ProfileSessionsPanel component
- [ ] Integrate sessions tab into profile settings
- [ ] Add session revocation functionality
- [ ] Create "Revoke All" feature
- [ ] Build device/browser detection display
- [ ] Implement session filtering
- [ ] Create useProfileSessions hook
- [ ] Add session activity timeline
- [ ] Write tests for session management

### Phase 9: Audit Log Viewer (Week 9)
- [ ] Create AuditLogs page
- [ ] Build AuditLogTable component
- [ ] Implement AuditFilters component
- [ ] Create AuditExport functionality (CSV)
- [ ] Build audit log detail modal
- [ ] Implement date range picker
- [ ] Create useAuditLogs hook
- [ ] Add audit log search
- [x] Build security audit view
- [x] Write tests for audit viewer

### Phase 10: Settings & Preferences (Week 10)
- [ ] Create Settings page with tabs
- [ ] Build user profile settings
- [ ] Implement password change form
- [ ] Create MFA setup flow with QR code
- [ ] Build appearance settings (theme, language)
- [ ] Implement notification preferences
- [ ] Create session preferences
- [ ] Build settings store (Zustand)
- [ ] Add settings persistence
- [ ] Write tests for settings

### Phase 11: Testing & Quality Assurance (Week 11)
- [ ] Achieve ≥80% unit test coverage
- [ ] Write integration tests for critical flows
- [ ] Set up Cypress for E2E testing
- [ ] Create E2E tests for authentication flow
- [ ] Test user management workflows
- [ ] Test permission assignment flows
- [ ] Verify accessibility (WCAG 2.1 AA)
- [ ] Test keyboard navigation
- [ ] Verify responsive design (mobile, tablet, desktop)
- [ ] Performance testing (Lighthouse score ≥90)

### Phase 12: Documentation & Polish (Week 12)
- [ ] Complete Storybook documentation for all components
- [ ] Write README with setup instructions
- [ ] Document API integration patterns
- [ ] Create component usage examples
- [ ] Add inline code documentation
- [ ] Build developer onboarding guide
- [ ] Create user guide for admin features
- [ ] Optimize bundle size
- [ ] Implement code splitting
- [ ] Final UI/UX polish

---

## Summary

This implementation plan provides a complete roadmap for building the Core Module frontend. The module follows best practices from `FRONTEND_GUIDELINES.md` and provides:

1. **Modern Tech Stack:** React 19, Vite 7, TypeScript 5.7+, Tailwind CSS v4
2. **Robust State Management:** TanStack Query for server state, Zustand for client state
3. **Type Safety:** Full TypeScript coverage with Zod validation
4. **Permission-Aware UI:** PermissionGuard component, permission-based rendering
5. **Secure Authentication:** Token management, refresh flow, MFA support, SSO integration
6. **Comprehensive Testing:** Unit, integration, E2E tests with ≥80% coverage
7. **Accessibility:** WCAG 2.1 AA compliance, keyboard navigation
8. **User Preferences:** All settings configurable, no hardcoded values
9. **Developer Experience:** Storybook, hot reload, TypeScript, ESLint
10. **Auth Provider Management:** UI-based configuration for OIDC, SAML, LDAP, Local, and Invite auth

The implementation is designed to be extended by other modules while maintaining consistency and code quality.

**Estimated Timeline:** 12 weeks (3 months) for complete implementation with testing and documentation.

---

**Next Steps:**
1. Review this plan with the team
2. Set up development environment (Node.js 20+, pnpm/npm)
3. Begin Phase 1 implementation
4. Coordinate with backend team for API contract alignment
5. Establish design system guidelines with UI/UX team
6. Set up CI/CD pipeline for automated testing
