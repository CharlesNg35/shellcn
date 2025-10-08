import logo from './assets/logo.svg'
import './App.css'

function App() {
  return (
    <div className="flex min-h-screen flex-col items-center justify-center bg-gradient-to-br from-primary-50 to-secondary-100">
      <div className="text-center">
        <img src={logo} className="mx-auto mb-8 h-48 w-48" alt="ShellCN Logo" />
        <h1 className="mb-4 text-5xl font-bold text-primary-900">ShellCN</h1>
        <p className="mb-8 text-xl text-secondary-600">Enterprise Remote Access Platform</p>
        <div className="space-y-4">
          <p className="text-secondary-700">
            Secure, scalable remote access to your infrastructure
          </p>
          <div className="flex justify-center gap-4">
            <span className="rounded-full bg-primary-100 px-4 py-2 text-sm font-medium text-primary-700">
              SSH
            </span>
            <span className="rounded-full bg-primary-100 px-4 py-2 text-sm font-medium text-primary-700">
              RDP
            </span>
            <span className="rounded-full bg-primary-100 px-4 py-2 text-sm font-medium text-primary-700">
              VNC
            </span>
            <span className="rounded-full bg-primary-100 px-4 py-2 text-sm font-medium text-primary-700">
              Kubernetes
            </span>
          </div>
        </div>
      </div>
    </div>
  )
}

export default App
