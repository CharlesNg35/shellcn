# ShellCN Web Frontend

Modern React-based frontend for the ShellCN platform.

## Tech Stack

- **React 19** - Latest React with Compiler support
- **TypeScript 5.9+** - Type safety
- **Vite 7** - Next-generation build tool
- **Tailwind CSS v4** - Utility-first CSS with new Oxide engine
- **React Router 7** - SPA routing
- **TanStack Query** - Server state management
- **Zustand** - Client state management
- **React Hook Form + Zod** - Form handling and validation
- **Radix UI** - Headless accessible components
- **Lucide React** - Icon library
- **Vitest** - Unit testing

## Getting Started

### Prerequisites

- Node.js 18+
- PNPM package manager

### Installation

```bash
# Install dependencies
pnpm install

# Start development server
pnpm dev

# Run tests
pnpm test

# Run tests with UI
pnpm test:ui

# Build for production
pnpm build
```

## Project Structure

```
src/
├── pages/              # Page components
├── components/         # Reusable components
│   └── ui/            # Base UI components
├── hooks/             # Custom React hooks
├── lib/               # Utilities and libraries
│   ├── api/          # API client
│   └── utils/        # Utility functions
├── store/             # Zustand stores
├── types/             # TypeScript types
├── schemas/           # Zod validation schemas
└── test/              # Test setup
```

## Available Scripts

- `pnpm dev` - Start development server on port 3000
- `pnpm build` - Build for production
- `pnpm preview` - Preview production build
- `pnpm test` - Run tests
- `pnpm test:ui` - Run tests with UI
- `pnpm test:coverage` - Run tests with coverage
- `pnpm lint` - Lint code
- `pnpm format` - Format code with Prettier

## Development

### Path Aliases

The project uses `@/` as an alias for the `src/` directory:

```typescript
import { Button } from '@/components/ui/Button'
import { cn } from '@/lib/utils/cn'
```

### Tailwind CSS v4

This project uses Tailwind CSS v4 with CSS-based configuration. Custom theme values are defined in `src/index.css` using the `@theme` directive.

### Component Development

1. Create component in `src/components/`
2. Add tests in `*.test.tsx`
3. Document with JSDoc comments

### Testing

Tests are written using Vitest and React Testing Library:

```typescript
import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'

describe('MyComponent', () => {
  it('renders correctly', () => {
    render(<MyComponent />)
    expect(screen.getByText('Hello')).toBeInTheDocument()
  })
})
```

## API Integration

The frontend proxies API requests to the backend server:

- Development: `http://localhost:8080`
- All `/api/*` requests are proxied automatically

## License

Proprietary

