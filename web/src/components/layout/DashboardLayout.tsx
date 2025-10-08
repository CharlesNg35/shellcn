import { Outlet } from 'react-router-dom'
import { Sidebar } from './Sidebar'
import { Header } from './Header'

export function DashboardLayout() {
  return (
    <div className="flex min-h-screen bg-background">
      <Sidebar />

      {/* Main content area - offset by sidebar width */}
      <div className="flex flex-1 flex-col pl-64 transition-all duration-300">
        <Header />

        <main className="flex-1 p-6">
          <Outlet />
        </main>
      </div>
    </div>
  )
}
