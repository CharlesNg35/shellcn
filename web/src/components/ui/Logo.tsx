/**
 * Logo component with inline SVG for reliable display in embedded builds.
 * This approach ensures the logo displays correctly whether served by Vite dev server
 * or embedded in the Go binary, avoiding path resolution issues while maintaining
 * strict CSP compliance.
 */
export interface LogoProps {
  className?: string
  size?: 'sm' | 'md' | 'lg' | 'xl'
}

const sizes = {
  sm: 'h-4 w-4',
  md: 'h-6 w-6',
  lg: 'h-10 w-10',
  xl: 'h-12 w-12',
}

export function Logo({ className, size = 'md' }: LogoProps) {
  return (
    <svg
      className={className || sizes[size]}
      viewBox="0 0 200 200"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      aria-label="ShellCN Logo"
    >
      <defs>
        <linearGradient id="logo-gradient" x1="0%" y1="0%" x2="100%" y2="100%">
          <stop offset="0%" style={{ stopColor: '#8b5cf6', stopOpacity: 1 }} />
          <stop offset="100%" style={{ stopColor: '#a855f7', stopOpacity: 1 }} />
        </linearGradient>
      </defs>
      <rect width="200" height="200" rx="40" fill="url(#logo-gradient)" />

      {/* Shell/Terminal Symbol - Stylized "S" with terminal prompt */}
      <g transform="translate(100, 100)">
        {/* Main "S" shape */}
        <path
          d="M 20 -40 Q 40 -40 40 -20 Q 40 0 20 0 L -20 0 Q -40 0 -40 20 Q -40 40 -20 40 L 20 40"
          stroke="#ffffff"
          strokeWidth="12"
          strokeLinecap="round"
          fill="none"
        />

        {/* Terminal prompt chevron integrated */}
        <path
          d="M -35 -35 L -25 -25 L -35 -15"
          stroke="#86efac"
          strokeWidth="6"
          strokeLinecap="round"
          strokeLinejoin="round"
          fill="none"
        />
      </g>
    </svg>
  )
}

/**
 * Favicon-sized logo component for smaller displays
 */
export function LogoIcon({ className }: { className?: string }) {
  return (
    <svg
      className={className || 'h-8 w-8'}
      viewBox="0 0 32 32"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      aria-label="ShellCN Icon"
    >
      <defs>
        <linearGradient id="favicon-gradient" x1="0%" y1="0%" x2="100%" y2="100%">
          <stop offset="0%" style={{ stopColor: '#8b5cf6', stopOpacity: 1 }} />
          <stop offset="100%" style={{ stopColor: '#a855f7', stopOpacity: 1 }} />
        </linearGradient>
      </defs>
      <rect width="32" height="32" rx="6" fill="url(#favicon-gradient)" />

      {/* Stylized "S" with terminal prompt */}
      <g transform="translate(16, 16)">
        {/* Main "S" shape */}
        <path
          d="M 3 -6 Q 6 -6 6 -3 Q 6 0 3 0 L -3 0 Q -6 0 -6 3 Q -6 6 -3 6 L 3 6"
          stroke="#ffffff"
          strokeWidth="2"
          strokeLinecap="round"
          fill="none"
        />

        {/* Terminal prompt chevron */}
        <path
          d="M -5 -5 L -3.5 -3.5 L -5 -2"
          stroke="#86efac"
          strokeWidth="1"
          strokeLinecap="round"
          strokeLinejoin="round"
          fill="none"
        />
      </g>
    </svg>
  )
}
