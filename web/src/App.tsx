import { Suspense, lazy } from 'react'
import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom'
import { ErrorBoundary } from '@/components/errors/ErrorBoundary'
import { ThemeProvider } from '@/components/theme/ThemeProvider'
import { BreadcrumbProvider } from '@/contexts/BreadcrumbContext'
import { Toaster } from '@/components/ui/Toaster'
import { AuthLayout } from '@/components/layout/AuthLayout'
import { DashboardLayout } from '@/components/layout/DashboardLayout'
import { SetupGuard } from '@/components/auth/SetupGuard'
import { ProtectedRoute } from '@/components/auth/ProtectedRoute'
import { RouteLoader } from '@/components/layout/RouteLoader'

const Login = lazy(() => import('@/pages/auth/Login').then((module) => ({ default: module.Login })))
const Register = lazy(() =>
  import('@/pages/auth/Register').then((module) => ({ default: module.Register }))
)
const Setup = lazy(() => import('@/pages/auth/Setup').then((module) => ({ default: module.Setup })))
const PasswordResetRequest = lazy(() =>
  import('@/pages/auth/PasswordResetRequest').then((module) => ({
    default: module.PasswordResetRequest,
  }))
)
const PasswordResetConfirm = lazy(() =>
  import('@/pages/auth/PasswordResetConfirm').then((module) => ({
    default: module.PasswordResetConfirm,
  }))
)
const MfaVerification = lazy(() =>
  import('@/pages/auth/MfaVerification').then((module) => ({
    default: module.MfaVerification,
  }))
)
const InviteAccept = lazy(() =>
  import('@/pages/auth/InviteAccept').then((module) => ({ default: module.InviteAccept }))
)
const Dashboard = lazy(() =>
  import('@/pages/dashboard/Dashboard').then((module) => ({ default: module.Dashboard }))
)
const Connections = lazy(() =>
  import('@/pages/connections/Connections').then((module) => ({ default: module.Connections }))
)
const Settings = lazy(() =>
  import('@/pages/settings/Settings').then((module) => ({ default: module.Settings }))
)
const Identities = lazy(() =>
  import('@/pages/settings/Identities').then((module) => ({ default: module.Identities }))
)
const Users = lazy(() =>
  import('@/pages/settings/Users').then((module) => ({ default: module.Users }))
)
const Teams = lazy(() =>
  import('@/pages/settings/Teams').then((module) => ({ default: module.Teams }))
)
const TeamDetail = lazy(() =>
  import('@/pages/settings/TeamDetail').then((module) => ({ default: module.TeamDetail }))
)
const Permissions = lazy(() =>
  import('@/pages/settings/Permissions').then((module) => ({ default: module.Permissions }))
)
const AuditLogs = lazy(() =>
  import('@/pages/settings/AuditLogs').then((module) => ({ default: module.AuditLogs }))
)
const AuthProviders = lazy(() =>
  import('@/pages/settings/AuthProviders').then((module) => ({ default: module.AuthProviders }))
)
const Security = lazy(() =>
  import('@/pages/settings/Security').then((module) => ({ default: module.Security }))
)
const SessionsPage = lazy(() =>
  import('@/pages/settings/Sessions').then((module) => ({ default: module.Sessions }))
)
const ProtocolSettingsPage = lazy(() =>
  import('@/pages/settings/ProtocolSettings').then((module) => ({
    default: module.ProtocolSettings,
  }))
)
const ProtocolWorkspaceRoute = lazy(() =>
  import('@/pages/sessions/ProtocolWorkspaceRoute').then((module) => ({
    default: module.ProtocolWorkspaceRoute,
  }))
)

function AppRoutes() {
  return (
    <Suspense fallback={<RouteLoader />}>
      <Routes>
        <Route element={<SetupGuard />}>
          <Route element={<AuthLayout />}>
            <Route path="/login" element={<Login />} />
            <Route path="/register" element={<Register />} />
            <Route path="/setup" element={<Setup />} />
            <Route path="/password-reset" element={<PasswordResetRequest />} />
            <Route path="/password-reset/confirm" element={<PasswordResetConfirm />} />
            <Route path="/mfa" element={<MfaVerification />} />
            <Route path="/invite/accept" element={<InviteAccept />} />
          </Route>

          <Route element={<ProtectedRoute />}>
            <Route element={<DashboardLayout />}>
              <Route path="/" element={<Navigate to="/dashboard" replace />} />
              <Route path="/dashboard" element={<Dashboard />} />
              <Route path="/settings" element={<Settings />} />
              <Route path="/connections" element={<Connections />} />

              {/* Settings */}
              <Route path="/settings/identities" element={<Identities />} />
              <Route path="/settings/users" element={<Users />} />
              <Route path="/settings/teams" element={<Teams />} />
              <Route path="/settings/teams/:teamId" element={<TeamDetail />} />
              <Route path="/settings/permissions" element={<Permissions />} />
              <Route path="/settings/audit" element={<AuditLogs />} />
              <Route path="/settings/auth-providers" element={<AuthProviders />} />
              <Route path="/settings/security" element={<Security />} />
              <Route path="/settings/sessions" element={<SessionsPage />} />
              <Route path="/settings/protocols/ssh" element={<ProtocolSettingsPage />} />
              <Route path="/active-sessions/:sessionId" element={<ProtocolWorkspaceRoute />} />
            </Route>
          </Route>
        </Route>

        <Route path="*" element={<Navigate to="/login" replace />} />
      </Routes>
    </Suspense>
  )
}

function App() {
  return (
    <ErrorBoundary>
      <ThemeProvider>
        <BrowserRouter>
          <BreadcrumbProvider>
            <AppRoutes />
          </BreadcrumbProvider>
        </BrowserRouter>
        <Toaster />
      </ThemeProvider>
    </ErrorBoundary>
  )
}

export default App
