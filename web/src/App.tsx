import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom'
import { AuthLayout } from '@/components/layout/AuthLayout'
import { DashboardLayout } from '@/components/layout/DashboardLayout'
import { Login } from '@/pages/auth/Login'
import { Setup } from '@/pages/auth/Setup'
import { PasswordResetRequest } from '@/pages/auth/PasswordResetRequest'
import { PasswordResetConfirm } from '@/pages/auth/PasswordResetConfirm'
import { MfaVerification } from '@/pages/auth/MfaVerification'
import { ProtectedRoute } from '@/components/auth/ProtectedRoute'
import { Dashboard } from '@/pages/dashboard/Dashboard'
import { Users } from '@/pages/settings/Users'
import { Organizations } from '@/pages/settings/Organizations'
import { Teams } from '@/pages/settings/Teams'
import { Permissions } from '@/pages/settings/Permissions'
import { Sessions } from '@/pages/settings/Sessions'
import { AuditLogs } from '@/pages/settings/AuditLogs'
import { AuthProviders } from '@/pages/settings/AuthProviders'
import { Security } from '@/pages/settings/Security'

function AppRoutes() {
  return (
    <Routes>
      <Route element={<AuthLayout />}>
        <Route path="/login" element={<Login />} />
        <Route path="/setup" element={<Setup />} />
        <Route path="/password-reset" element={<PasswordResetRequest />} />
        <Route path="/password-reset/confirm" element={<PasswordResetConfirm />} />
        <Route path="/mfa" element={<MfaVerification />} />
      </Route>

      <Route element={<ProtectedRoute />}>
        <Route element={<DashboardLayout />}>
          <Route path="/" element={<Navigate to="/dashboard" replace />} />
          <Route path="/dashboard" element={<Dashboard />} />
          <Route path="/settings/users" element={<Users />} />
          <Route path="/settings/organizations" element={<Organizations />} />
          <Route path="/settings/teams" element={<Teams />} />
          <Route path="/settings/permissions" element={<Permissions />} />
          <Route path="/settings/sessions" element={<Sessions />} />
          <Route path="/settings/audit" element={<AuditLogs />} />
          <Route path="/settings/auth-providers" element={<AuthProviders />} />
          <Route path="/settings/security" element={<Security />} />
        </Route>
      </Route>

      <Route path="*" element={<Navigate to="/login" replace />} />
    </Routes>
  )
}

function App() {
  return (
    <BrowserRouter>
      <AppRoutes />
    </BrowserRouter>
  )
}

export default App
