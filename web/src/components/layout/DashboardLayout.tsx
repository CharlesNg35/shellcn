import { Outlet } from 'react-router-dom'
import { Sidebar } from './Sidebar'
import { Header } from './Header'

export function DashboardLayout() {
  return (
    <div className="flex min-h-screen bg-background">
      <Sidebar />

      {/* Main content area - offset by sidebar width */}
      <div className="flex flex-1 flex-col lg:pl-64">
        <Header />

        <main className="flex flex-1 flex-col">
          <div className="flex flex-1 flex-col gap-4 p-4 lg:gap-6 lg:p-6">
            <Outlet />
          </div>
        </main>
      </div>
    </div>
  )
}
