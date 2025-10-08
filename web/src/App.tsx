import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom'
import { AuthLayout } from '@/components/layout/AuthLayout'
import { Login } from '@/pages/auth/Login'
import { Setup } from '@/pages/auth/Setup'
import { PasswordResetRequest } from '@/pages/auth/PasswordResetRequest'
import { PasswordResetConfirm } from '@/pages/auth/PasswordResetConfirm'
import { MfaVerification } from '@/pages/auth/MfaVerification'
import { ProtectedRoute } from '@/components/auth/ProtectedRoute'
import { Dashboard } from '@/pages/dashboard/Dashboard'

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
        <Route path="/" element={<Navigate to="/dashboard" replace />} />
        <Route path="/dashboard" element={<Dashboard />} />
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
