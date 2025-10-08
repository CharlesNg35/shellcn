import { useState } from 'react'
import { Link } from 'react-router-dom'
import {
  Server,
  Monitor,
  Database,
  Container,
  Cloud,
  HardDrive,
  Plus,
  Search,
  Filter,
  MoreVertical,
} from 'lucide-react'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'

interface Connection {
  id: string
  name: string
  type: 'ssh' | 'rdp' | 'vnc' | 'docker' | 'kubernetes' | 'database' | 'proxmox'
  host: string
  port: number
  status: 'connected' | 'disconnected' | 'error'
  tags: string[]
  lastUsed?: Date
}

const connectionTypes = [
  { id: 'all', label: 'All Connections', icon: Server, count: 0 },
  { id: 'ssh', label: 'SSH / Telnet', icon: Server, count: 0 },
  { id: 'rdp', label: 'RDP', icon: Monitor, count: 0 },
  { id: 'vnc', label: 'VNC', icon: Monitor, count: 0 },
  { id: 'docker', label: 'Docker', icon: Container, count: 0 },
  { id: 'kubernetes', label: 'Kubernetes', icon: Cloud, count: 0 },
  { id: 'database', label: 'Databases', icon: Database, count: 0 },
  { id: 'proxmox', label: 'Proxmox', icon: HardDrive, count: 0 },
]

// Mock data - will be replaced with API calls
const mockConnections: Connection[] = [
  {
    id: '1',
    name: 'Production Server',
    type: 'ssh',
    host: '192.168.1.100',
    port: 22,
    status: 'disconnected',
    tags: ['production', 'web'],
  },
  {
    id: '2',
    name: 'Windows Desktop',
    type: 'rdp',
    host: '192.168.1.101',
    port: 3389,
    status: 'disconnected',
    tags: ['windows', 'dev'],
  },
]

export function Connections() {
  const [selectedType, setSelectedType] = useState('all')
  const [searchQuery, setSearchQuery] = useState('')

  const filteredConnections = mockConnections.filter((conn) => {
    const matchesType = selectedType === 'all' || conn.type === selectedType
    const matchesSearch =
      searchQuery === '' ||
      conn.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
      conn.host.toLowerCase().includes(searchQuery.toLowerCase()) ||
      conn.tags.some((tag) => tag.toLowerCase().includes(searchQuery.toLowerCase()))
    return matchesType && matchesSearch
  })

  return (
    <div className="space-y-4">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Connections</h1>
          <p className="text-sm text-muted-foreground">
            Manage your remote connections and infrastructure access
          </p>
        </div>
        <Button asChild>
          <Link to="/connections/new">
            <Plus className="h-4 w-4" />
            New Connection
          </Link>
        </Button>
      </div>

      {/* Search and Filter */}
      <div className="flex items-center gap-4">
        <div className="relative flex-1">
          <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            placeholder="Search connections..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="pl-9"
          />
        </div>
        <Button variant="outline" size="sm">
          <Filter className="h-4 w-4" />
          Filter
        </Button>
      </div>

      {/* Connection Type Tabs */}
      <div className="flex gap-2 overflow-x-auto pb-2">
        {connectionTypes.map((type) => {
          const Icon = type.icon
          const isActive = selectedType === type.id
          return (
            <button
              key={type.id}
              onClick={() => setSelectedType(type.id)}
              className={`flex items-center gap-2 whitespace-nowrap rounded-md px-4 py-2 text-sm font-medium transition-colors ${
                isActive
                  ? 'bg-primary text-primary-foreground shadow-sm'
                  : 'bg-muted text-muted-foreground hover:bg-accent hover:text-accent-foreground'
              }`}
            >
              <Icon className="h-4 w-4" />
              {type.label}
              {type.count > 0 && (
                <span className="ml-1 rounded-full bg-background/20 px-2 py-0.5 text-xs">
                  {type.count}
                </span>
              )}
            </button>
          )
        })}
      </div>

      {/* Connections Grid */}
      {filteredConnections.length === 0 ? (
        <div className="flex flex-col items-center justify-center rounded-lg border border-dashed border-border bg-muted/50 py-16">
          <Server className="mb-4 h-12 w-12 text-muted-foreground" />
          <h3 className="mb-2 text-lg font-semibold">No connections found</h3>
          <p className="mb-4 text-sm text-muted-foreground">
            {searchQuery
              ? 'Try adjusting your search or filters'
              : 'Get started by creating your first connection'}
          </p>
          {!searchQuery && (
            <Button asChild>
              <Link to="/connections/new">
                <Plus className="h-4 w-4" />
                Create Connection
              </Link>
            </Button>
          )}
        </div>
      ) : (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {filteredConnections.map((connection) => {
            const TypeIcon = connectionTypes.find((t) => t.id === connection.type)?.icon || Server
            return (
              <div
                key={connection.id}
                className="group relative rounded-lg border border-border bg-card p-4 shadow-sm transition-all hover:shadow-md"
              >
                <div className="flex items-start justify-between">
                  <div className="flex items-start gap-3">
                    <div className="flex h-10 w-10 items-center justify-center rounded-md bg-primary/10">
                      <TypeIcon className="h-5 w-5 text-primary" />
                    </div>
                    <div className="flex-1">
                      <h3 className="font-semibold">{connection.name}</h3>
                      <p className="text-sm text-muted-foreground">
                        {connection.host}:{connection.port}
                      </p>
                    </div>
                  </div>
                  <button className="rounded-md p-1 opacity-0 transition-opacity hover:bg-accent group-hover:opacity-100">
                    <MoreVertical className="h-4 w-4" />
                  </button>
                </div>

                <div className="mt-4 flex items-center gap-2">
                  <div
                    className={`h-2 w-2 rounded-full ${
                      connection.status === 'connected'
                        ? 'bg-green-500'
                        : connection.status === 'error'
                          ? 'bg-destructive'
                          : 'bg-muted-foreground'
                    }`}
                  />
                  <span className="text-xs text-muted-foreground capitalize">
                    {connection.status}
                  </span>
                </div>

                {connection.tags.length > 0 && (
                  <div className="mt-3 flex flex-wrap gap-1">
                    {connection.tags.map((tag) => (
                      <span
                        key={tag}
                        className="rounded-md bg-muted px-2 py-0.5 text-xs text-muted-foreground"
                      >
                        {tag}
                      </span>
                    ))}
                  </div>
                )}

                <div className="mt-4 flex gap-2">
                  <Button size="sm" className="flex-1" asChild>
                    <Link to={`/connections/${connection.id}`}>Connect</Link>
                  </Button>
                  <Button size="sm" variant="outline" asChild>
                    <Link to={`/connections/${connection.id}/edit`}>Edit</Link>
                  </Button>
                </div>
              </div>
            )
          })}
        </div>
      )}
    </div>
  )
}
