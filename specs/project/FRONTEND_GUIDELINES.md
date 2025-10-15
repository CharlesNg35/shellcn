# ShellCN Platform - Frontend Development Guidelines

## Project Stack

### Frontend
- **React 19** with latest features
- **Vite 7** as build tool
- **TypeScript** for type safety
- **Tailwind CSS v4** for styling
- **PNPM** as package manager
- **xterm.js** for terminal emulation

### Key Libraries
- **React Router 7** for SPA routing
- **TanStack Query** (React Query) for server state
- **Zustand** for client state management
- **react-hook-form** for form handling
- **Zod** for validation schemas
- **lucide-react** for icons
- **class-variance-authority** for component variants
- **@monaco-editor/react** for any rich text/code editing surfaces
- **react-dropzone** for drag-and-drop and file uploads

---

## 1. Frontend Architecture

### 1.1 Project Structure

```
web/src/
├── main.tsx               # Application entry
├── App.tsx                # Root component
├── pages/                 # Page components
│   ├── Dashboard.tsx
│   ├── Login.tsx
│   ├── vault/             # Vault pages
│   ├── connections/       # Connection pages
│   ├── docker/
│   ├── kubernetes/
│   └── settings/          # User preferences & settings
├── components/            # Reusable components
│   ├── ui/               # Base UI components
│   ├── terminal/         # Terminal components
│   ├── file-manager/     # File browser
│   └── vault/            # Vault-specific
├── hooks/                # Custom React hooks
├── lib/                  # Utilities & API client
├── store/                # State management (Zustand)
└── types/                # TypeScript types
```

---

## 2. Component Development

### 2.1 Component Organization

**Keep components small and focused:**

```tsx
// ✅ Good: Small, focused component
// components/vault/IdentitySelector.tsx
import { Select, SelectContent, SelectItem, SelectTrigger } from '@/components/ui/select';
import { useIdentities } from '@/hooks/useIdentities';

interface IdentitySelectorProps {
    value: string | null;
    onChange: (value: string | null) => void;
    type?: 'ssh' | 'database' | 'all';
}

export function IdentitySelector({ value, onChange, type = 'all' }: IdentitySelectorProps) {
    const { data: identities, isLoading } = useIdentities({ type });

    return (
        <div className="space-y-2">
            <label className="text-sm font-medium">
                Identity
                <a href="/settings/identities" className="ml-2 text-blue-600 hover:underline">
                    (Manage)
                </a>
            </label>

            <Select value={value || 'custom'} onValueChange={onChange}>
                <SelectTrigger>
                    <span>{value ? identities?.find(i => i.id === value)?.name : 'Custom Identity'}</span>
                </SelectTrigger>
                <SelectContent>
                    <SelectItem value="custom">
                        <em>Custom Identity</em>
                    </SelectItem>
                    {identities?.map(identity => (
                        <SelectItem key={identity.id} value={identity.id}>
                            {identity.name}
                        </SelectItem>
                    ))}
                </SelectContent>
            </Select>
        </div>
    );
}
```

### 2.2 UI Components with Variants

**Build reusable components with variants:**

```tsx
// components/ui/button.tsx
import { type ButtonHTMLAttributes, forwardRef } from 'react';
import { cva, type VariantProps } from 'class-variance-authority';
import { cn } from '@/lib/utils';

const buttonVariants = cva(
    // Base styles
    'inline-flex items-center justify-center rounded-lg font-medium transition-all',
    {
        variants: {
            variant: {
                primary: 'bg-blue-600 text-white hover:bg-blue-700',
                secondary: 'bg-gray-100 text-gray-900 hover:bg-gray-200',
                danger: 'bg-red-600 text-white hover:bg-red-700',
                ghost: 'hover:bg-gray-100',
            },
            size: {
                sm: 'px-3 py-1.5 text-sm',
                md: 'px-4 py-2 text-base',
                lg: 'px-6 py-3 text-lg',
            },
        },
        defaultVariants: {
            variant: 'primary',
            size: 'md',
        },
    }
);

export interface ButtonProps
    extends ButtonHTMLAttributes<HTMLButtonElement>,
        VariantProps<typeof buttonVariants> {}

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(
    ({ className, variant, size, ...props }, ref) => {
        return (
            <button
                ref={ref}
                className={cn(buttonVariants({ variant, size, className }))}
                {...props}
            />
        );
    }
);

Button.displayName = 'Button';
```

### 2.3 Permission Handling

- Always reference platform permissions through the typed `PERMISSIONS` map defined in `web/src/constants/permissions.ts`; never hardcode permission strings.
- `PermissionGuard` accepts `PermissionId` values and optional `anyOf` / `allOf` arrays. Prefer colocating permission checks close to the UI they protect.
- Utilities and configs (`navigation`, `features`, etc.) must use the same constants to stay in sync with backend definitions (`internal/permissions/core.go`). Update the shared constants whenever new permissions are introduced.
- Never render an action (buttons, links, menu items, empty‑state CTAs, header actions, etc.) unless the current user has permission to execute it. Wrap privileged UI in `PermissionGuard` (or feature-aware components) and omit it entirely when the user lacks access.

### 2.4 Use Proven Packages Instead of Hand-Rolled Widgets

- Reach for `@monaco-editor/react` whenever we need an in-browser editor. Do not build textarea-based editors for configuration or code—Monaco gives syntax highlighting, diffing, and accessibility out of the box.
- Persist shared client state in Zustand stores and rely on React Query for server/cache orchestration. Avoid bespoke context providers or ad-hoc `useState` caches for workspace/session data.
- For drag-and-drop, uploads, and other complex DOM interactions, adopt community packages such as `react-dropzone` rather than wiring raw `dragenter`/`drop` listeners repeatedly. This keeps behavior consistent and well-tested.
- When a new UX need appears, audit the existing stack first; if a capability fits one of these packages, extend the shared implementation instead of re-inventing it in a leaf component.

---

## 3. API Communication

### 3.1 Centralized API Client

**Use a centralized API client:**

```typescript
// lib/api/client.ts
import axios, { AxiosError } from 'axios';

const apiClient = axios.create({
    baseURL: '/api',
    headers: {
        'Content-Type': 'application/json',
    },
});

// Request interceptor (add auth token)
apiClient.interceptors.request.use((config) => {
    const token = localStorage.getItem('auth_token');
    if (token) {
        config.headers.Authorization = `Bearer ${token}`;
    }
    return config;
});

// Response interceptor (handle errors)
apiClient.interceptors.response.use(
    (response) => response,
    (error: AxiosError) => {
        if (error.response?.status === 401) {
            // Redirect to login
            window.location.href = '/login';
        }
        return Promise.reject(error);
    }
);

export default apiClient;
```

### 3.2 API Modules

**Create API modules per domain:**

```typescript
// lib/api/vault.ts
import apiClient from './client';
import type { Identity, CreateIdentityRequest } from '@/types/vault';

export const vaultAPI = {
    // List identities
    listIdentities: async (type?: string): Promise<Identity[]> => {
        const params = type ? { type } : {};
        const response = await apiClient.get<Identity[]>('/vault/identities', { params });
        return response.data;
    },

    // Create identity
    createIdentity: async (data: CreateIdentityRequest): Promise<Identity> => {
        const response = await apiClient.post<Identity>('/vault/identities', data);
        return response.data;
    },

    // Update identity
    updateIdentity: async (id: string, data: Partial<CreateIdentityRequest>): Promise<Identity> => {
        const response = await apiClient.put<Identity>(`/vault/identities/${id}`, data);
        return response.data;
    },

    // Delete identity
    deleteIdentity: async (id: string): Promise<void> => {
        await apiClient.delete(`/vault/identities/${id}`);
    },
};
```

---

## 4. State Management

### 4.1 Server State with TanStack Query

**Use TanStack Query for server state:**

```typescript
// hooks/useIdentities.ts
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { vaultAPI } from '@/lib/api/vault';
import type { Identity, CreateIdentityRequest } from '@/types/vault';

export function useIdentities(params?: { type?: string }) {
    return useQuery({
        queryKey: ['identities', params],
        queryFn: () => vaultAPI.listIdentities(params?.type),
    });
}

export function useCreateIdentity() {
    const queryClient = useQueryClient();

    return useMutation({
        mutationFn: (data: CreateIdentityRequest) => vaultAPI.createIdentity(data),
        onSuccess: () => {
            // Invalidate and refetch
            queryClient.invalidateQueries({ queryKey: ['identities'] });
        },
    });
}

export function useDeleteIdentity() {
    const queryClient = useQueryClient();

    return useMutation({
        mutationFn: (id: string) => vaultAPI.deleteIdentity(id),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['identities'] });
        },
    });
}
```

**Usage in components:**

```tsx
// pages/vault/IdentityList.tsx
import { useIdentities, useDeleteIdentity } from '@/hooks/useIdentities';

export function IdentityList() {
    const { data: identities, isLoading, error } = useIdentities();
    const deleteMutation = useDeleteIdentity();

    const handleDelete = async (id: string) => {
        if (confirm('Delete this identity?')) {
            await deleteMutation.mutateAsync(id);
        }
    };

    if (isLoading) return <LoadingSpinner />;
    if (error) return <ErrorMessage error={error} />;

    return (
        <div>
            {identities?.map(identity => (
                <IdentityCard
                    key={identity.id}
                    identity={identity}
                    onDelete={handleDelete}
                />
            ))}
        </div>
    );
}
```

### 4.2 Client State with Zustand

**Use Zustand for client-side state:**

```typescript
// store/useSettingsStore.ts
import { create } from 'zustand';
import { persist } from 'zustand/middleware';

export interface UserPreferences {
    // Terminal preferences
    terminal: {
        fontFamily: string;
        fontSize: number;
        cursorStyle: 'block' | 'underline' | 'bar';
        cursorBlink: boolean;
        theme: 'dark' | 'light' | 'custom';
        customTheme?: {
            background: string;
            foreground: string;
            cursor: string;
            selection: string;
            // ... other colors
        };
    };
    // File manager preferences
    fileManager: {
        showHiddenFiles: boolean;
        defaultView: 'list' | 'grid';
        sortBy: 'name' | 'size' | 'date';
    };
    // General UI preferences
    ui: {
        sidebarCollapsed: boolean;
        theme: 'dark' | 'light' | 'system';
    };
}

interface SettingsStore {
    preferences: UserPreferences;
    updateTerminalPreferences: (prefs: Partial<UserPreferences['terminal']>) => void;
    updateFileManagerPreferences: (prefs: Partial<UserPreferences['fileManager']>) => void;
    updateUIPreferences: (prefs: Partial<UserPreferences['ui']>) => void;
    resetPreferences: () => void;
}

const defaultPreferences: UserPreferences = {
    terminal: {
        fontFamily: 'Menlo, Monaco, "Courier New", monospace',
        fontSize: 14,
        cursorStyle: 'block',
        cursorBlink: true,
        theme: 'dark',
    },
    fileManager: {
        showHiddenFiles: false,
        defaultView: 'list',
        sortBy: 'name',
    },
    ui: {
        sidebarCollapsed: false,
        theme: 'system',
    },
};

export const useSettingsStore = create<SettingsStore>()(
    persist(
        (set) => ({
            preferences: defaultPreferences,

            updateTerminalPreferences: (prefs) =>
                set((state) => ({
                    preferences: {
                        ...state.preferences,
                        terminal: { ...state.preferences.terminal, ...prefs },
                    },
                })),

            updateFileManagerPreferences: (prefs) =>
                set((state) => ({
                    preferences: {
                        ...state.preferences,
                        fileManager: { ...state.preferences.fileManager, ...prefs },
                    },
                })),

            updateUIPreferences: (prefs) =>
                set((state) => ({
                    preferences: {
                        ...state.preferences,
                        ui: { ...state.preferences.ui, ...prefs },
                    },
                })),

            resetPreferences: () => set({ preferences: defaultPreferences }),
        }),
        {
            name: 'shellcn-settings',
        }
    )
);
```

---

## 5. User Preferences & Settings

### 5.1 Settings Page Structure

**IMPORTANT: Never hardcode user preferences like terminal font, theme, etc.**

All user-facing settings should be configurable through the Settings page:

```tsx
// pages/settings/TerminalSettings.tsx
import { useSettingsStore } from '@/store/useSettingsStore';
import { Input, Select, Switch } from '@/components/ui';

export function TerminalSettings() {
    const { preferences, updateTerminalPreferences } = useSettingsStore();
    const { terminal } = preferences;

    return (
        <div className="space-y-6">
            <h2 className="text-2xl font-bold">Terminal Settings</h2>

            <div className="space-y-4">
                <div>
                    <label className="block text-sm font-medium mb-2">
                        Font Family
                    </label>
                    <Input
                        value={terminal.fontFamily}
                        onChange={(e) =>
                            updateTerminalPreferences({ fontFamily: e.target.value })
                        }
                        placeholder="Menlo, Monaco, Courier New"
                    />
                    <p className="text-sm text-gray-500 mt-1">
                        Use comma-separated list of font families
                    </p>
                </div>

                <div>
                    <label className="block text-sm font-medium mb-2">
                        Font Size
                    </label>
                    <Input
                        type="number"
                        min={8}
                        max={32}
                        value={terminal.fontSize}
                        onChange={(e) =>
                            updateTerminalPreferences({ fontSize: Number(e.target.value) })
                        }
                    />
                </div>

                <div>
                    <label className="block text-sm font-medium mb-2">
                        Cursor Style
                    </label>
                    <Select
                        value={terminal.cursorStyle}
                        onChange={(e) =>
                            updateTerminalPreferences({
                                cursorStyle: e.target.value as 'block' | 'underline' | 'bar',
                            })
                        }
                    >
                        <option value="block">Block</option>
                        <option value="underline">Underline</option>
                        <option value="bar">Bar</option>
                    </Select>
                </div>

                <div className="flex items-center justify-between">
                    <label className="text-sm font-medium">Cursor Blink</label>
                    <Switch
                        checked={terminal.cursorBlink}
                        onCheckedChange={(checked) =>
                            updateTerminalPreferences({ cursorBlink: checked })
                        }
                    />
                </div>

                <div>
                    <label className="block text-sm font-medium mb-2">Theme</label>
                    <Select
                        value={terminal.theme}
                        onChange={(e) =>
                            updateTerminalPreferences({
                                theme: e.target.value as 'dark' | 'light' | 'custom',
                            })
                        }
                    >
                        <option value="dark">Dark</option>
                        <option value="light">Light</option>
                        <option value="custom">Custom</option>
                    </Select>
                </div>

                {terminal.theme === 'custom' && (
                    <CustomThemeEditor
                        theme={terminal.customTheme}
                        onChange={(theme) =>
                            updateTerminalPreferences({ customTheme: theme })
                        }
                    />
                )}
            </div>
        </div>
    );
}
```

### 5.2 Terminal Component with User Preferences

**❌ BAD: Hardcoded terminal settings**

```tsx
// DON'T DO THIS!
const terminal = new Terminal({
    cursorBlink: true,
    fontSize: 14,
    fontFamily: 'Menlo, Monaco, "Courier New", monospace',
    theme: {
        background: '#1e1e1e',
        foreground: '#d4d4d4',
    },
});
```

**✅ GOOD: Use user preferences**

```tsx
// components/terminal/SSHTerminal.tsx
import { useEffect, useRef } from 'react';
import { Terminal } from 'xterm';
import { FitAddon } from 'xterm-addon-fit';
import { WebLinksAddon } from 'xterm-addon-web-links';
import { useWebSocket } from '@/hooks/useWebSocket';
import { useSettingsStore } from '@/store/useSettingsStore';
import { getTerminalTheme } from '@/lib/terminal-themes';
import 'xterm/css/xterm.css';

interface SSHTerminalProps {
    connectionId: string;
}

export function SSHTerminal({ connectionId }: SSHTerminalProps) {
    const terminalRef = useRef<HTMLDivElement>(null);
    const xtermRef = useRef<Terminal | null>(null);

    // Get user preferences
    const { preferences } = useSettingsStore();
    const { terminal: terminalPrefs } = preferences;

    const { isConnected, send } = useWebSocket(
        `/ws/ssh/${connectionId}`,
        {
            onMessage: (data) => {
                if (data.type === 'output') {
                    xtermRef.current?.write(data.content);
                }
            },
            autoReconnect: true,
        }
    );

    useEffect(() => {
        if (!terminalRef.current) return;

        // Create terminal with user preferences
        const terminal = new Terminal({
            cursorBlink: terminalPrefs.cursorBlink,
            cursorStyle: terminalPrefs.cursorStyle,
            fontSize: terminalPrefs.fontSize,
            fontFamily: terminalPrefs.fontFamily,
            theme: getTerminalTheme(terminalPrefs.theme, terminalPrefs.customTheme),
        });

        const fitAddon = new FitAddon();
        const webLinksAddon = new WebLinksAddon();

        terminal.loadAddon(fitAddon);
        terminal.loadAddon(webLinksAddon);

        terminal.open(terminalRef.current);
        fitAddon.fit();

        terminal.onData((data) => {
            send({ type: 'input', content: data });
        });

        xtermRef.current = terminal;

        // Handle window resize
        const handleResize = () => fitAddon.fit();
        window.addEventListener('resize', handleResize);

        return () => {
            window.removeEventListener('resize', handleResize);
            terminal.dispose();
        };
    }, [terminalPrefs]); // Re-create terminal when preferences change

    return (
        <div className="h-full flex flex-col">
            <div className="bg-gray-900 p-2 flex items-center justify-between">
                <div className="flex items-center gap-2">
                    <span
                        className={cn(
                            'inline-block w-2 h-2 rounded-full',
                            isConnected ? 'bg-green-500' : 'bg-red-500'
                        )}
                    />
                    <span className="text-white text-sm">
                        {isConnected ? 'Connected' : 'Disconnected'}
                    </span>
                </div>
            </div>
            <div ref={terminalRef} className="flex-1" />
        </div>
    );
}
```

### 5.3 Terminal Theme Helper

```typescript
// lib/terminal-themes.ts
import type { ITheme } from 'xterm';

export const darkTheme: ITheme = {
    background: '#1e1e1e',
    foreground: '#d4d4d4',
    cursor: '#ffffff',
    cursorAccent: '#000000',
    selection: '#264f78',
    black: '#000000',
    red: '#cd3131',
    green: '#0dbc79',
    yellow: '#e5e510',
    blue: '#2472c8',
    magenta: '#bc3fbc',
    cyan: '#11a8cd',
    white: '#e5e5e5',
    brightBlack: '#666666',
    brightRed: '#f14c4c',
    brightGreen: '#23d18b',
    brightYellow: '#f5f543',
    brightBlue: '#3b8eea',
    brightMagenta: '#d670d6',
    brightCyan: '#29b8db',
    brightWhite: '#e5e5e5',
};

export const lightTheme: ITheme = {
    background: '#ffffff',
    foreground: '#333333',
    cursor: '#000000',
    cursorAccent: '#ffffff',
    selection: '#add6ff',
    black: '#000000',
    red: '#cd3131',
    green: '#00bc00',
    yellow: '#949800',
    blue: '#0451a5',
    magenta: '#bc05bc',
    cyan: '#0598bc',
    white: '#555555',
    brightBlack: '#666666',
    brightRed: '#cd3131',
    brightGreen: '#14ce14',
    brightYellow: '#b5ba00',
    brightBlue: '#0451a5',
    brightMagenta: '#bc05bc',
    brightCyan: '#0598bc',
    brightWhite: '#a5a5a5',
};

export function getTerminalTheme(
    theme: 'dark' | 'light' | 'custom',
    customTheme?: Partial<ITheme>
): ITheme {
    switch (theme) {
        case 'light':
            return lightTheme;
        case 'custom':
            return customTheme ? { ...darkTheme, ...customTheme } : darkTheme;
        case 'dark':
        default:
            return darkTheme;
    }
}
```

---

## 6. WebSocket Integration

### 6.1 WebSocket Hook

```typescript
// hooks/useWebSocket.ts
import { useEffect, useRef, useState } from 'react';

interface UseWebSocketOptions {
    onMessage?: (data: any) => void;
    onConnect?: () => void;
    onDisconnect?: () => void;
    onError?: (error: Event) => void;
    autoReconnect?: boolean;
    reconnectInterval?: number;
}

export function useWebSocket(url: string, options: UseWebSocketOptions = {}) {
    const [isConnected, setIsConnected] = useState(false);
    const wsRef = useRef<WebSocket | null>(null);
    const reconnectTimeoutRef = useRef<NodeJS.Timeout>();

    useEffect(() => {
        const connect = () => {
            try {
                const ws = new WebSocket(url);

                ws.onopen = () => {
                    setIsConnected(true);
                    options.onConnect?.();
                };

                ws.onmessage = (event) => {
                    try {
                        const data = JSON.parse(event.data);
                        options.onMessage?.(data);
                    } catch (error) {
                        console.error('Failed to parse WebSocket message:', error);
                    }
                };

                ws.onerror = (error) => {
                    options.onError?.(error);
                };

                ws.onclose = () => {
                    setIsConnected(false);
                    options.onDisconnect?.();

                    // Auto-reconnect
                    if (options.autoReconnect) {
                        reconnectTimeoutRef.current = setTimeout(
                            connect,
                            options.reconnectInterval || 3000
                        );
                    }
                };

                wsRef.current = ws;
            } catch (error) {
                console.error('Failed to create WebSocket:', error);
            }
        };

        connect();

        return () => {
            if (reconnectTimeoutRef.current) {
                clearTimeout(reconnectTimeoutRef.current);
            }
            wsRef.current?.close();
        };
    }, [url]);

    const send = (data: any) => {
        if (wsRef.current?.readyState === WebSocket.OPEN) {
            wsRef.current.send(JSON.stringify(data));
        } else {
            console.warn('WebSocket is not connected');
        }
    };

    const close = () => {
        if (reconnectTimeoutRef.current) {
            clearTimeout(reconnectTimeoutRef.current);
        }
        wsRef.current?.close();
    };

    return { isConnected, send, close };
}
```

---

## 7. Form Handling

### 7.1 Validation with Zod

**Define validation schemas:**

```typescript
// lib/validations/vault.ts
import { z } from 'zod';

export const createIdentitySchema = z.object({
    name: z.string().min(1, 'Name is required').min(3, 'Name must be at least 3 characters'),
    type: z.enum(['ssh', 'database', 'generic']),
    username: z.string().min(1, 'Username is required'),
    password: z.string().optional(),
    privateKey: z.string().optional(),
    passphrase: z.string().optional(),
    notes: z.string().optional(),
}).refine(data => data.password || data.privateKey, {
    message: 'Either password or private key is required',
    path: ['password'],
});

export type CreateIdentityInput = z.infer<typeof createIdentitySchema>;
```

### 7.2 Form Implementation

**Use react-hook-form with Zod:**

```tsx
// pages/vault/NewIdentity.tsx
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { createIdentitySchema, type CreateIdentityInput } from '@/lib/validations/vault';
import { useCreateIdentity } from '@/hooks/useIdentities';
import { Button, Input, Select, Textarea } from '@/components/ui';

export function NewIdentity() {
    const createMutation = useCreateIdentity();

    const {
        register,
        handleSubmit,
        formState: { errors, isSubmitting },
        watch,
    } = useForm<CreateIdentityInput>({
        resolver: zodResolver(createIdentitySchema),
    });

    const onSubmit = async (data: CreateIdentityInput) => {
        try {
            await createMutation.mutateAsync(data);
            // Navigate to list page
        } catch (error) {
            console.error('Failed to create identity:', error);
        }
    };

    return (
        <form onSubmit={handleSubmit(onSubmit)} className="space-y-4 max-w-2xl">
            <h1 className="text-2xl font-bold">Create New Identity</h1>

            <Input
                label="Name"
                {...register('name')}
                error={errors.name?.message}
                placeholder="My SSH Key"
            />

            <Select
                label="Type"
                {...register('type')}
                error={errors.type?.message}
            >
                <option value="">Select type...</option>
                <option value="ssh">SSH</option>
                <option value="database">Database</option>
                <option value="generic">Generic</option>
            </Select>

            <Input
                label="Username"
                {...register('username')}
                error={errors.username?.message}
                placeholder="username"
            />

            <Input
                label="Password"
                type="password"
                {...register('password')}
                error={errors.password?.message}
            />

            <Textarea
                label="Private Key"
                {...register('privateKey')}
                error={errors.privateKey?.message}
                placeholder="-----BEGIN OPENSSH PRIVATE KEY-----"
                rows={6}
            />

            <Input
                label="Passphrase (if key is encrypted)"
                type="password"
                {...register('passphrase')}
                error={errors.passphrase?.message}
            />

            <Textarea
                label="Notes"
                {...register('notes')}
                error={errors.notes?.message}
                rows={3}
            />

            <div className="flex gap-2">
                <Button type="submit" disabled={isSubmitting}>
                    {isSubmitting ? 'Creating...' : 'Create Identity'}
                </Button>
                <Button type="button" variant="secondary" onClick={() => history.back()}>
                    Cancel
                </Button>
            </div>
        </form>
    );
}
```

---

## 8. TypeScript Best Practices

### 8.1 Type Definitions

**Create comprehensive type definitions:**

```typescript
// types/vault.ts
export interface Identity {
    id: string;
    name: string;
    type: 'ssh' | 'database' | 'generic';
    username: string;
    userId: string;
    createdAt: string;
    updatedAt: string;
    sharedWith?: string[];
}

export interface CreateIdentityRequest {
    name: string;
    type: 'ssh' | 'database' | 'generic';
    username: string;
    password?: string;
    privateKey?: string;
    passphrase?: string;
    notes?: string;
}

export interface SSHConnection {
    id: string;
    name: string;
    protocol: 'ssh' | 'auto';
    icon?: string;
    host: string;
    port: number;
    identityId?: string;
    identity?: Identity;
    authMethod: 'password' | 'publickey' | 'keyboard-interactive';
    username?: string;
    config: SSHConnectionConfig;
    createdAt: string;
    updatedAt: string;
}

export interface SSHConnectionConfig {
    receiveEncoding: string;
    terminalEncoding: string;
    altGrMode: string;
    enableReconnect: boolean;
    reconnectAttempts: number;
    reconnectDelay: number;
    clipboardEnabled: boolean;
    sessionRecording: boolean;
}
```

### 8.2 Avoid `any` Type

```typescript
// ❌ Bad
function processData(data: any) {
    return data.value;
}

// ✅ Good
interface DataPayload {
    value: string;
    timestamp: number;
}

function processData(data: DataPayload): string {
    return data.value;
}

// ✅ Good: Use generics when type is flexible
function processResponse<T>(response: { data: T }): T {
    return response.data;
}
```

---

## 9. Styling with Tailwind CSS v4

### 9.1 Best Practices

**Use semantic class names:**

```tsx
// ✅ Good: Readable and maintainable
<div className="flex items-center justify-between p-4 bg-white rounded-lg shadow">
    <h2 className="text-lg font-semibold text-gray-900">Title</h2>
    <button className="px-4 py-2 text-sm text-white bg-blue-600 rounded hover:bg-blue-700">
        Action
    </button>
</div>

// ❌ Avoid: Too many classes, hard to read
<div className="flex items-center justify-between p-4 bg-white rounded-lg shadow-sm border border-gray-200 hover:shadow-md transition-shadow duration-200">
    ...
</div>

// ✅ Better: Extract to component with variant
<Card>
    <CardHeader>
        <CardTitle>Title</CardTitle>
    </CardHeader>
    <CardContent>
        <Button variant="primary">Action</Button>
    </CardContent>
</Card>
```

### 9.2 Use `cn` Utility

```typescript
// lib/utils/cn.ts
import { type ClassValue, clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';

export function cn(...inputs: ClassValue[]) {
    return twMerge(clsx(inputs));
}

// Usage
<div className={cn(
    'base-class',
    isActive && 'active-class',
    className  // Allow override
)}>
```

---

## 10. Security Best Practices

### 10.1 Input Sanitization

**Sanitize user input before rendering:**

```tsx
// ❌ Dangerous: XSS vulnerability
<div dangerouslySetInnerHTML={{ __html: userInput }} />

// ✅ Good: React auto-escapes
<div>{userInput}</div>

// ✅ Good: Sanitize HTML if needed
import DOMPurify from 'dompurify';

<div dangerouslySetInnerHTML={{
    __html: DOMPurify.sanitize(userInput)
}} />
```

### 10.2 Storage Security

**Never store sensitive data in localStorage:**

```typescript
// ❌ Bad
localStorage.setItem('password', password);
localStorage.setItem('privateKey', privateKey);

// ✅ Good: Only store non-sensitive data
localStorage.setItem('theme', theme);
localStorage.setItem('auth_token', token);  // JWT is OK (short-lived)
localStorage.setItem('preferences', JSON.stringify(preferences));
```

---

## 11. Testing

### 11.1 Component Testing (Vitest + React Testing Library)

```tsx
// components/vault/IdentitySelector.test.tsx
import { render, screen, fireEvent } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { IdentitySelector } from './IdentitySelector';

const createWrapper = () => {
    const queryClient = new QueryClient({
        defaultOptions: {
            queries: { retry: false },
        },
    });

    return ({ children }: { children: React.ReactNode }) => (
        <QueryClientProvider client={queryClient}>
            {children}
        </QueryClientProvider>
    );
};

describe('IdentitySelector', () => {
    it('renders custom identity option', () => {
        render(
            <IdentitySelector value={null} onChange={() => {}} />,
            { wrapper: createWrapper() }
        );

        expect(screen.getByText(/custom identity/i)).toBeInTheDocument();
    });

    it('calls onChange when selection changes', () => {
        const onChange = vi.fn();

        render(
            <IdentitySelector value={null} onChange={onChange} />,
            { wrapper: createWrapper() }
        );

        fireEvent.click(screen.getByRole('combobox'));
        fireEvent.click(screen.getByText('Test Identity'));

        expect(onChange).toHaveBeenCalledWith('identity-123');
    });
});
```

---

## 12. Code Quality Checklist

### Before Committing - ALWAYS Run These Commands:

**IMPORTANT**: Always run these commands before committing to ensure code quality:

```bash
# 1. Run type checking and build
pnpm run build

# 2. Run linting
pnpm run lint

# 3. Run tests
pnpm run test

# Fix any errors before committing!
```

If any of these commands fail, **fix the errors before committing**. Never commit broken code.

### Code Quality Checklist:

- [ ] **Build & Tests**
  - [ ] ✅ `pnpm run build` passes with no errors
  - [ ] ✅ `pnpm run lint` passes with no errors
  - [ ] ✅ `pnpm run test` passes with no failures
  - [ ] All TypeScript errors resolved
  - [ ] No console errors in browser

- [ ] **TypeScript**
  - [ ] All types properly defined (no `any`)
  - [ ] Interfaces created for complex objects
  - [ ] Generics used where appropriate
  - [ ] No implicit any warnings

- [ ] **Components**
  - [ ] Small and focused (single responsibility)
  - [ ] Props properly typed
  - [ ] Error states handled
  - [ ] Loading states added (with skeleton loaders)
  - [ ] Empty states with EmptyState component
  - [ ] User preferences respected (no hardcoded values)
  - [ ] Toast notifications for user actions

- [ ] **State Management**
  - [ ] Server state managed with TanStack Query
  - [ ] Client state managed with Zustand (if needed)
  - [ ] Cache invalidation handled correctly
  - [ ] Optimistic updates for better UX (optional)

- [ ] **Forms**
  - [ ] Validation with Zod schemas
  - [ ] Error messages user-friendly
  - [ ] Loading/submitting states shown
  - [ ] Success feedback provided (with toast)
  - [ ] Error feedback provided (with toast)

- [ ] **Styling**
  - [ ] Responsive design tested (mobile, tablet, desktop)
  - [ ] Accessibility considered (ARIA labels, keyboard navigation)
  - [ ] Dark/light theme support verified
  - [ ] No layout shift issues
  - [ ] Proper focus states

- [ ] **Testing**
  - [ ] Component tests added
  - [ ] User interactions tested
  - [ ] Edge cases covered
  - [ ] Error states tested
  - [ ] Loading states tested

---

## 13. Common Patterns

### 13.1 Connection Form Pattern

All connection forms follow this structure:

1. **Basic Tab**
   - Name, Icon, Protocol
   - Host, Port
   - Identity Selector (links to `/settings/identities`)
   - Authentication (if custom identity)
   - Notes

2. **Advanced Tab**
   - Protocol-specific settings
   - Encoding, Keyboard, Scrolling
   - Reconnection settings

### 13.2 Permission-Based Rendering

```tsx
// components/PermissionGuard.tsx
import { usePermissions } from '@/hooks/usePermissions';

interface PermissionGuardProps {
    permission: string;
    fallback?: React.ReactNode;
    children: React.ReactNode;
}

export function PermissionGuard({
    permission,
    fallback = null,
    children
}: PermissionGuardProps) {
    const { hasPermission } = usePermissions();

    if (!hasPermission(permission)) {
        return <>{fallback}</>;
    }

    return <>{children}</>;
}

// Usage
<PermissionGuard permission="vault.create">
    <Button onClick={handleCreate}>Create Identity</Button>
</PermissionGuard>
```

### 13.3 Toast Notifications Pattern

**Always provide user feedback for actions using toast notifications.**

```tsx
// lib/utils/toast.ts
import { toast } from '@/lib/utils/toast';

// Success notifications
toast.success('Connection created successfully');

// Error notifications with description
toast.error('Failed to connect to server', {
    description: 'Invalid credentials provided'
});

// Info notifications
toast.info('Session recording started');

// Warning notifications
toast.warning('Connection unstable', {
    description: 'Network latency detected'
});

// Loading notifications
const toastId = toast.loading('Connecting to server...');
// Later dismiss it
toast.dismiss(toastId);

// Promise-based notifications (auto-handles loading, success, error)
toast.promise(
    connectToServer(id),
    {
        loading: 'Connecting to server...',
        success: (data) => `Connected to ${data.name}`,
        error: (err) => `Failed: ${err.message}`
    }
);

// Toast with action button
toast.success('Session terminated', {
    description: 'Session ended by administrator',
    action: {
        label: 'Reconnect',
        onClick: () => reconnect()
    }
});
```

**Use toast notifications in mutation hooks:**

```tsx
// hooks/useUsers.ts
import { toast } from '@/lib/utils/toast';

export function useUserMutations() {
    const queryClient = useQueryClient();

    const create = useMutation({
        mutationFn: (payload: UserCreatePayload) => createUser(payload),
        onSuccess: async (user) => {
            await queryClient.invalidateQueries({ queryKey: ['users'] });
            toast.success('User created successfully', {
                description: `${user.username} has been added to the system`
            });
        },
        onError: (error: ApiError) => {
            toast.error('Failed to create user', {
                description: error.message || 'Please try again'
            });
        }
    });

    return { create };
}
```

### 13.4 Empty States Pattern

**Use descriptive empty states with icons and actions.**

```tsx
// components/ui/EmptyState.tsx
import { EmptyState } from '@/components/ui/EmptyState';
import { Server } from 'lucide-react';

// Usage in components
export function ConnectionList() {
    const { data: connections } = useConnections();

    if (connections.length === 0) {
        return (
            <EmptyState
                icon={Server}
                title="No connections yet"
                description="Get started by creating your first connection profile"
                action={
                    <Button asChild>
                        <Link to="/connections?create=true">
                            <Plus className="mr-1 h-4 w-4" />
                            Create Connection
                        </Link>
                    </Button>
                }
            />
        );
    }

    return <ConnectionTable connections={connections} />;
}
```

### 13.5 Skeleton Loaders Pattern

**Use skeleton loaders instead of full-page loading states for better perceived performance.**

```tsx
// components/ui/Skeleton.tsx
import { Skeleton, ConnectionCardSkeleton, StatCardSkeleton } from '@/components/ui/Skeleton';

// Usage for loading states
export function Dashboard() {
    const { data: connections, isLoading } = useConnections();

    if (isLoading) {
        return (
            <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
                <StatCardSkeleton />
                <StatCardSkeleton />
                <StatCardSkeleton />
                <StatCardSkeleton />
            </div>
        );
    }

    return <DashboardContent connections={connections} />;
}

// Custom skeleton for specific components
export function ConnectionCard({ connection }: { connection?: Connection }) {
    if (!connection) {
        return <ConnectionCardSkeleton />;
    }

    return (
        <div className="rounded-lg border border-border bg-card p-4">
            {/* Card content */}
        </div>
    );
}
```

**Common skeleton patterns:**

```tsx
// Simple skeleton
<Skeleton className="h-12 w-full" />
<Skeleton className="h-4 w-[250px]" />

// Connection card skeleton
<ConnectionCardSkeleton />

// Table skeleton
{Array.from({ length: 10 }).map((_, i) => (
    <TableRowSkeleton key={i} columns={5} />
))}

// List skeleton
{Array.from({ length: 5 }).map((_, i) => (
    <ListItemSkeleton key={i} />
))}
```

### 13.6 Inline SVG Components Pattern

**For logos and critical assets, use inline SVG components to avoid path resolution issues in embedded builds.**

```tsx
// components/ui/Logo.tsx
export function Logo({ size = 'md' }: { size?: 'sm' | 'md' | 'lg' | 'xl' }) {
    return (
        <svg
            className={sizes[size]}
            viewBox="0 0 200 200"
            fill="none"
            xmlns="http://www.w3.org/2000/svg"
            aria-label="ShellCN Logo"
        >
            {/* SVG content */}
        </svg>
    );
}

// Usage
<Logo size="md" />
```

**Benefits:**
- ✅ Works in dev, production, and embedded Go builds
- ✅ No external file dependencies
- ✅ No CSP img-src issues
- ✅ Better performance (no HTTP request)
- ✅ Scalable without quality loss

---

## Quick Reference

### File Paths
- Frontend: `web/src/`
- Components: `web/src/components/`
- Pages: `web/src/pages/`
- Types: `web/src/types/`
- API client: `web/src/lib/api/`
- Store: `web/src/store/`

### Key Technologies
- **React 19** + **Vite 7** + **TypeScript**
- **Tailwind CSS v4** for styling
- **TanStack Query** for server state
- **Zustand** for client state
- **react-hook-form** + **Zod** for forms
- **xterm.js** for terminals
- **lucide-react** for icons
- **sonner** for toast notifications

### Important Principles
- ✅ Always use user preferences (never hardcode terminal settings, themes, etc.)
- ✅ Store all user preferences in Zustand with persistence
- ✅ Make all UI elements configurable through Settings
- ✅ Provide sensible defaults but allow full customization
- ✅ Re-render components when preferences change
- ✅ Test with different preference combinations
- ✅ Always provide user feedback with toast notifications for actions
- ✅ Use skeleton loaders for better perceived performance
- ✅ Show descriptive empty states with icons and call-to-action
- ✅ Use inline SVG components for critical assets (logos, icons)

---

**End of Guidelines**
