# Module Features - Comprehensive Expansion

This document contains the complete, expanded feature sets for Core, Docker, Kubernetes, and Database modules.

---

## Core Module - Authentication & User Management

### 1.1 First-Time Setup
- No default credentials (security best practice)
- Setup wizard on first access
- Create first admin user with root/superuser privileges
- Password strength validation
- Auto-login after setup

### 1.2 Authentication Providers (UI-Configured)

**IMPORTANT:** All authentication providers are configured via UI by administrators, NOT through configuration files or environment variables.

**Local Authentication (Always Enabled):**
- Username and password authentication
- Password hashing with bcrypt
- Settings:
  - Allow Registration: Enable/disable user self-registration
  - Cannot be disabled (always available as fallback)

**Email Invitation (Optional):**
- Invite users via email
- Settings:
  - Enable/disable invitation system
  - Require email verification
- Email notification or shareable link
- System provider (cannot be deleted)

**OpenID Connect (OIDC) (Optional):**
- Single Sign-On via OpenID Connect
- UI Configuration:
  - Issuer URL
  - Client ID
  - Client Secret (encrypted)
  - Redirect URL
  - Scopes (e.g., `openid profile email`)
- Supported providers: Google, Azure AD, Okta, Keycloak, etc.
- Connection testing

**OAuth 2.0 (Optional):**
- Generic OAuth 2.0 authentication
- UI Configuration:
  - Authorization URL
  - Token URL
  - User Info URL
  - Client ID
  - Client Secret (encrypted)
  - Redirect URL
  - Scopes
- Supported providers: GitHub, GitLab, custom OAuth2 providers

**SAML 2.0 (Optional):**
- SAML 2.0 Single Sign-On
- UI Configuration:
  - Metadata URL
  - Entity ID
  - SSO URL
  - Certificate
  - Private Key (encrypted)
  - Attribute Mapping (SAML attributes → user fields)
- Supported providers: Azure AD, Okta, OneLogin

**LDAP / Active Directory (Optional):**
- LDAP or Active Directory authentication
- UI Configuration:
  - Host
  - Port
  - Base DN
  - Bind DN
  - Bind Password (encrypted)
  - User Filter
  - Use TLS
  - Skip Certificate Verification
  - Attribute Mapping (LDAP attributes → user fields)
- Connection testing
- Supported: Active Directory, OpenLDAP, FreeIPA

**Auth Provider Management Features:**
- List all available providers
- Configure provider settings via UI forms
- Enable/disable providers with toggle
- Test provider connections (LDAP, OIDC)
- Delete provider configurations (except local and invite)
- View provider status (enabled/disabled, configured/not configured)
- Audit logging for all configuration changes
- Permission-based access (`permission.manage` required)

### 1.3 Multi-Factor Authentication (MFA)
- TOTP (Time-based One-Time Password)
- QR code generation for authenticator apps
- Backup codes
- Optional per user
- Enforce MFA for specific roles

### 1.4 User Management
- Create, edit, delete users
- Activate/deactivate users
- Reset passwords
- Assign roles and permissions
- View user sessions
- User profile management

### 1.5 Organization & Team Management
- Multi-tenancy support
- Create organizations
- Create teams within organizations
- Assign users to teams
- Team-based permissions
- Organization-level settings

### 1.6 Permission System
- Fine-grained permission system
- Permission dependencies
- Root/superuser bypass
- Permission registry (global)
- Module-based permissions
- Dynamic permission checking
- Permission matrix UI

### 1.7 Session Management
- JWT-based authentication
- Access and refresh tokens
- Multi-device support
- Session listing and revocation
- Session timeout configuration
- Remember me functionality

### 1.8 Audit Logging
- Comprehensive audit trail
- Log all user actions
- Filter and search logs
- Export audit logs
- Retention policies
- Compliance reporting

### 1.9 Security Features
- Password hashing (bcrypt)
- Credential encryption (AES-256-GCM)
- Vault for sensitive data
- CSRF protection
- XSS prevention
- Rate limiting
- IP whitelisting (optional)

---

## Kubernetes Module - Complete Feature Set

### 9.1 Cluster Connection
- Connect to K8s cluster
- kubeconfig support (upload or paste)
- Token authentication
- Certificate authentication
- Service account authentication
- Multiple context management
- Namespace selection
- Cluster switching
- Connection pooling

### 9.2 Workload Resources

**Pod Management:**
- List pods (all namespaces or specific)
- View pod details
- Pod logs (real-time, follow, tail, timestamps)
- Execute in pod (kubectl exec)
- Pod terminal (multi-container support)
- Delete pods
- Pod describe (YAML/JSON)
- Pod events
- Pod metrics (CPU, Memory, Network)
- Restart pod
- Port forward to pod
- Copy files to/from pod

**Deployment Management:**
- List deployments
- Create deployment (YAML editor)
- Edit deployment (rolling update)
- Scale deployments (manual or autoscale)
- Update deployment
- Delete deployment
- Rollback deployment (revision history)
- Pause/Resume deployment
- Restart deployment
- View deployment status
- Deployment events
- Replica set management

**StatefulSet Management:**
- List StatefulSets
- Create StatefulSet
- Edit StatefulSet
- Scale StatefulSet
- Delete StatefulSet
- View StatefulSet status
- Rolling update management

**DaemonSet Management:**
- List DaemonSets
- Create DaemonSet
- Edit DaemonSet
- Delete DaemonSet
- View DaemonSet status
- Node selector management

**Job & CronJob Management:**
- List Jobs
- Create Job
- Delete Job
- View Job logs
- Job completion status
- List CronJobs
- Create CronJob
- Edit CronJob schedule
- Suspend/Resume CronJob
- Trigger CronJob manually

**ReplicaSet Management:**
- List ReplicaSets
- View ReplicaSet details
- Scale ReplicaSet
- Delete ReplicaSet

### 9.3 Service & Networking

**Service Management:**
- List services
- Create service (ClusterIP, NodePort, LoadBalancer)
- Edit service
- Delete service
- Service inspection (endpoints, selectors)
- Service port mapping
- Service type conversion
- External name services

**Ingress Management:**
- List Ingresses
- Create Ingress
- Edit Ingress rules
- Delete Ingress
- TLS certificate management
- Ingress class selection
- Path-based routing
- Host-based routing

**NetworkPolicy Management:**
- List NetworkPolicies
- Create NetworkPolicy
- Edit NetworkPolicy
- Delete NetworkPolicy
- Ingress/Egress rules
- Pod selector configuration

**Endpoints Management:**
- List Endpoints
- View Endpoint details
- Endpoint subset inspection

### 9.4 Configuration & Storage

**ConfigMap Management:**
- List ConfigMaps
- Create ConfigMap (from file, literal, YAML)
- Edit ConfigMap
- Delete ConfigMap
- View ConfigMap data
- ConfigMap versioning
- Use as environment variables
- Mount as volumes

**Secret Management:**
- List Secrets
- Create Secret (generic, docker-registry, TLS)
- Edit Secret
- Delete Secret
- View Secret data (base64 decoded)
- Secret types (Opaque, TLS, Docker config)
- Use in pods

**PersistentVolume Management:**
- List PersistentVolumes
- Create PersistentVolume
- Edit PersistentVolume
- Delete PersistentVolume
- View PV status and capacity
- Storage class association
- Reclaim policy management

**PersistentVolumeClaim Management:**
- List PersistentVolumeClaims
- Create PersistentVolumeClaim
- Edit PersistentVolumeClaim
- Delete PersistentVolumeClaim
- View PVC status
- Resize PVC
- Bind status

**StorageClass Management:**
- List StorageClasses
- Create StorageClass
- Edit StorageClass
- Delete StorageClass
- Provisioner configuration
- Volume binding mode

### 9.5 Cluster Resources

**Node Management:**
- List nodes
- View node details
- Node metrics (CPU, Memory, Disk)
- Node conditions
- Node labels and taints
- Cordon/Uncordon node
- Drain node
- Node capacity and allocatable resources
- Node events

**Namespace Management:**
- List namespaces
- Create namespace
- Delete namespace
- Set resource quotas
- Set limit ranges
- Namespace labels

**ServiceAccount Management:**
- List ServiceAccounts
- Create ServiceAccount
- Delete ServiceAccount
- Token management
- RBAC binding

**ResourceQuota Management:**
- List ResourceQuotas
- Create ResourceQuota
- Edit ResourceQuota
- Delete ResourceQuota
- View quota usage

**LimitRange Management:**
- List LimitRanges
- Create LimitRange
- Edit LimitRange
- Delete LimitRange

### 9.6 RBAC (Role-Based Access Control)

**Role Management:**
- List Roles
- Create Role
- Edit Role
- Delete Role
- View Role permissions

**ClusterRole Management:**
- List ClusterRoles
- Create ClusterRole
- Edit ClusterRole
- Delete ClusterRole

**RoleBinding Management:**
- List RoleBindings
- Create RoleBinding
- Edit RoleBinding
- Delete RoleBinding

**ClusterRoleBinding Management:**
- List ClusterRoleBindings
- Create ClusterRoleBinding
- Edit ClusterRoleBinding
- Delete ClusterRoleBinding

### 9.7 Advanced Features

**HorizontalPodAutoscaler (HPA):**
- List HPAs
- Create HPA
- Edit HPA (metrics, min/max replicas)
- Delete HPA
- View HPA status and metrics

**VerticalPodAutoscaler (VPA):**
- List VPAs
- Create VPA
- Edit VPA
- Delete VPA

**PodDisruptionBudget:**
- List PodDisruptionBudgets
- Create PodDisruptionBudget
- Edit PodDisruptionBudget
- Delete PodDisruptionBudget

**Custom Resource Definitions (CRD):**
- List CRDs
- View CRD details
- Create custom resources
- Edit custom resources
- Delete custom resources

**Events:**
- View cluster events
- Filter events by namespace/resource
- Event streaming (real-time)

**Port Forwarding:**
- Forward pod ports to local
- Forward service ports
- Multiple port forwards
- Port forward management
- Auto-reconnect

**Resource Metrics:**
- Cluster-wide metrics
- Node metrics
- Pod metrics
- Container metrics
- Integration with Metrics Server

**YAML/JSON Editor:**
- Edit any resource as YAML/JSON
- Syntax validation
- Apply changes
- Dry-run mode

### Kubernetes Permissions (Complete)

```go
K8S_PERMISSIONS = {
    // Connection
    "k8s.connect": {
        "module": "kubernetes",
        "depends_on": [],
        "description": "Connect to K8s clusters",
    },

    // Pods
    "k8s.pod.list": {
        "module": "kubernetes",
        "depends_on": ["k8s.connect"],
        "description": "List pods",
    },
    "k8s.pod.view": {
        "module": "kubernetes",
        "depends_on": ["k8s.pod.list"],
        "description": "View pod details",
    },
    "k8s.pod.exec": {
        "module": "kubernetes",
        "depends_on": ["k8s.pod.list"],
        "description": "Execute in pods",
    },
    "k8s.pod.logs": {
        "module": "kubernetes",
        "depends_on": ["k8s.pod.list"],
        "description": "View pod logs",
    },
    "k8s.pod.delete": {
        "module": "kubernetes",
        "depends_on": ["k8s.pod.list"],
        "description": "Delete pods",
    },
    "k8s.pod.portforward": {
        "module": "kubernetes",
        "depends_on": ["k8s.pod.list"],
        "description": "Port forward to pods",
    },

    // Deployments
    "k8s.deployment.list": {
        "module": "kubernetes",
        "depends_on": ["k8s.connect"],
        "description": "List deployments",
    },
    "k8s.deployment.create": {
        "module": "kubernetes",
        "depends_on": ["k8s.deployment.list"],
        "description": "Create deployments",
    },
    "k8s.deployment.edit": {
        "module": "kubernetes",
        "depends_on": ["k8s.deployment.list"],
        "description": "Edit deployments",
    },
    "k8s.deployment.scale": {
        "module": "kubernetes",
        "depends_on": ["k8s.deployment.list"],
        "description": "Scale deployments",
    },
    "k8s.deployment.delete": {
        "module": "kubernetes",
        "depends_on": ["k8s.deployment.list"],
        "description": "Delete deployments",
    },
    "k8s.deployment.rollback": {
        "module": "kubernetes",
        "depends_on": ["k8s.deployment.edit"],
        "description": "Rollback deployments",
    },

    // StatefulSets
    "k8s.statefulset.list": {
        "module": "kubernetes",
        "depends_on": ["k8s.connect"],
        "description": "List StatefulSets",
    },
    "k8s.statefulset.manage": {
        "module": "kubernetes",
        "depends_on": ["k8s.statefulset.list"],
        "description": "Manage StatefulSets",
    },

    // DaemonSets
    "k8s.daemonset.list": {
        "module": "kubernetes",
        "depends_on": ["k8s.connect"],
        "description": "List DaemonSets",
    },
    "k8s.daemonset.manage": {
        "module": "kubernetes",
        "depends_on": ["k8s.daemonset.list"],
        "description": "Manage DaemonSets",
    },

    // Jobs & CronJobs
    "k8s.job.list": {
        "module": "kubernetes",
        "depends_on": ["k8s.connect"],
        "description": "List Jobs",
    },
    "k8s.job.manage": {
        "module": "kubernetes",
        "depends_on": ["k8s.job.list"],
        "description": "Manage Jobs",
    },
    "k8s.cronjob.list": {
        "module": "kubernetes",
        "depends_on": ["k8s.connect"],
        "description": "List CronJobs",
    },
    "k8s.cronjob.manage": {
        "module": "kubernetes",
        "depends_on": ["k8s.cronjob.list"],
        "description": "Manage CronJobs",
    },

    // Services
    "k8s.service.list": {
        "module": "kubernetes",
        "depends_on": ["k8s.connect"],
        "description": "List services",
    },
    "k8s.service.create": {
        "module": "kubernetes",
        "depends_on": ["k8s.service.list"],
        "description": "Create services",
    },
    "k8s.service.edit": {
        "module": "kubernetes",
        "depends_on": ["k8s.service.list"],
        "description": "Edit services",
    },
    "k8s.service.delete": {
        "module": "kubernetes",
        "depends_on": ["k8s.service.list"],
        "description": "Delete services",
    },

    // Ingress
    "k8s.ingress.list": {
        "module": "kubernetes",
        "depends_on": ["k8s.connect"],
        "description": "List Ingresses",
    },
    "k8s.ingress.manage": {
        "module": "kubernetes",
        "depends_on": ["k8s.ingress.list"],
        "description": "Manage Ingresses",
    },

    // NetworkPolicy
    "k8s.networkpolicy.list": {
        "module": "kubernetes",
        "depends_on": ["k8s.connect"],
        "description": "List NetworkPolicies",
    },
    "k8s.networkpolicy.manage": {
        "module": "kubernetes",
        "depends_on": ["k8s.networkpolicy.list"],
        "description": "Manage NetworkPolicies",
    },

    // ConfigMaps
    "k8s.configmap.list": {
        "module": "kubernetes",
        "depends_on": ["k8s.connect"],
        "description": "List ConfigMaps",
    },
    "k8s.configmap.create": {
        "module": "kubernetes",
        "depends_on": ["k8s.configmap.list"],
        "description": "Create ConfigMaps",
    },
    "k8s.configmap.edit": {
        "module": "kubernetes",
        "depends_on": ["k8s.configmap.list"],
        "description": "Edit ConfigMaps",
    },
    "k8s.configmap.delete": {
        "module": "kubernetes",
        "depends_on": ["k8s.configmap.list"],
        "description": "Delete ConfigMaps",
    },

    // Secrets
    "k8s.secret.list": {
        "module": "kubernetes",
        "depends_on": ["k8s.connect"],
        "description": "List secrets",
    },
    "k8s.secret.view": {
        "module": "kubernetes",
        "depends_on": ["k8s.secret.list"],
        "description": "View secret data",
    },
    "k8s.secret.create": {
        "module": "kubernetes",
        "depends_on": ["k8s.secret.list"],
        "description": "Create secrets",
    },
    "k8s.secret.edit": {
        "module": "kubernetes",
        "depends_on": ["k8s.secret.view"],
        "description": "Edit secrets",
    },
    "k8s.secret.delete": {
        "module": "kubernetes",
        "depends_on": ["k8s.secret.list"],
        "description": "Delete secrets",
    },

    // Storage
    "k8s.pv.list": {
        "module": "kubernetes",
        "depends_on": ["k8s.connect"],
        "description": "List PersistentVolumes",
    },
    "k8s.pv.manage": {
        "module": "kubernetes",
        "depends_on": ["k8s.pv.list"],
        "description": "Manage PersistentVolumes",
    },
    "k8s.pvc.list": {
        "module": "kubernetes",
        "depends_on": ["k8s.connect"],
        "description": "List PersistentVolumeClaims",
    },
    "k8s.pvc.manage": {
        "module": "kubernetes",
        "depends_on": ["k8s.pvc.list"],
        "description": "Manage PersistentVolumeClaims",
    },
    "k8s.storageclass.list": {
        "module": "kubernetes",
        "depends_on": ["k8s.connect"],
        "description": "List StorageClasses",
    },
    "k8s.storageclass.manage": {
        "module": "kubernetes",
        "depends_on": ["k8s.storageclass.list"],
        "description": "Manage StorageClasses",
    },

    // Nodes
    "k8s.node.list": {
        "module": "kubernetes",
        "depends_on": ["k8s.connect"],
        "description": "List nodes",
    },
    "k8s.node.view": {
        "module": "kubernetes",
        "depends_on": ["k8s.node.list"],
        "description": "View node details",
    },
    "k8s.node.manage": {
        "module": "kubernetes",
        "depends_on": ["k8s.node.list"],
        "description": "Manage nodes (cordon, drain)",
    },

    // Namespaces
    "k8s.namespace.list": {
        "module": "kubernetes",
        "depends_on": ["k8s.connect"],
        "description": "List namespaces",
    },
    "k8s.namespace.create": {
        "module": "kubernetes",
        "depends_on": ["k8s.namespace.list"],
        "description": "Create namespaces",
    },
    "k8s.namespace.delete": {
        "module": "kubernetes",
        "depends_on": ["k8s.namespace.list"],
        "description": "Delete namespaces",
    },

    // RBAC
    "k8s.rbac.view": {
        "module": "kubernetes",
        "depends_on": ["k8s.connect"],
        "description": "View RBAC resources",
    },
    "k8s.rbac.manage": {
        "module": "kubernetes",
        "depends_on": ["k8s.rbac.view"],
        "description": "Manage RBAC resources",
    },

    // Advanced
    "k8s.hpa.list": {
        "module": "kubernetes",
        "depends_on": ["k8s.connect"],
        "description": "List HPAs",
    },
    "k8s.hpa.manage": {
        "module": "kubernetes",
        "depends_on": ["k8s.hpa.list"],
        "description": "Manage HPAs",
    },
    "k8s.events.view": {
        "module": "kubernetes",
        "depends_on": ["k8s.connect"],
        "description": "View cluster events",
    },
    "k8s.metrics.view": {
        "module": "kubernetes",
        "depends_on": ["k8s.connect"],
        "description": "View resource metrics",
    },
}
```

---

## Docker Module - Complete Feature Set

### 8.1 Docker Host Connection
- Connect to Docker daemon
- TCP connection
- Unix socket connection
- SSH tunnel support
- TLS authentication (client certificates)
- Docker context support
- Connection health monitoring

### 8.2 Container Management

**Container Operations:**
- List containers (all, running, stopped, paused)
- Create new containers
- Start containers
- Stop containers
- Restart containers
- Pause/Unpause containers
- Kill containers (SIGKILL)
- Rename containers
- Delete containers
- Prune stopped containers

**Container Inspection:**
- View container details
- Container logs (real-time, follow, tail, timestamps)
- Container stats (CPU, memory, network, disk I/O)
- Container processes (top)
- Container file system changes
- Container port mappings
- Container environment variables
- Container labels

**Container Interaction:**
- Execute commands in container (docker exec)
- Attach to container
- Container terminal (interactive shell)
- Copy files to/from container
- Export container filesystem
- Commit container to image
- Update container configuration (resource limits)

**Container Networking:**
- View container networks
- Connect container to network
- Disconnect container from network
- Inspect network settings

### 8.3 Image Management

**Image Operations:**
- List images (all, dangling)
- Pull images from registry
- Push images to registry
- Build images from Dockerfile
- Tag images
- Delete images
- Prune unused images
- Save images to tar
- Load images from tar
- Import filesystem as image

**Image Inspection:**
- View image details
- Image history (layers)
- Image size
- Image labels
- Image environment variables

**Image Registry:**
- Login to registry
- Logout from registry
- Search images in registry
- Private registry support

### 8.4 Volume Management

**Volume Operations:**
- List volumes
- Create volumes
- Delete volumes
- Prune unused volumes
- Volume driver support

**Volume Inspection:**
- View volume details
- Volume mount points
- Volume size
- Volume driver options
- Volume labels

### 8.5 Network Management

**Network Operations:**
- List networks
- Create networks (bridge, host, overlay, macvlan)
- Delete networks
- Prune unused networks
- Connect containers to network
- Disconnect containers from network

**Network Inspection:**
- View network details
- Network driver
- Network subnet/gateway
- Connected containers
- Network options
- IPAM configuration

### 8.6 System & Administration

**System Information:**
- Docker version
- System info (OS, architecture, CPU, memory)
- Disk usage
- Data root directory
- Storage driver
- Logging driver
- Runtime information

**System Operations:**
- Docker events (real-time monitoring)
- System-wide prune (containers, images, volumes, networks)
- Docker swarm status (if enabled)

**Resource Management:**
- Set container resource limits (CPU, memory)
- View resource usage across all containers
- Container restart policies

### 8.7 Docker Compose (Optional)

**Compose Operations:**
- List compose projects
- Deploy compose stack
- Stop compose stack
- Remove compose stack
- Scale services
- View compose logs
- Restart compose services

### 8.8 Docker Swarm (Optional)

**Swarm Management:**
- Initialize swarm
- Join swarm
- Leave swarm
- View swarm nodes
- Promote/Demote nodes

**Service Management:**
- List services
- Create services
- Scale services
- Update services
- Delete services
- View service logs
- Service tasks/replicas

### Docker Permissions (Complete)

```go
DOCKER_PERMISSIONS = {
    // Connection
    "docker.connect": {
        "module": "docker",
        "depends_on": [],
        "description": "Connect to Docker hosts",
    },

    // Container - Read
    "docker.container.list": {
        "module": "docker",
        "depends_on": ["docker.connect"],
        "description": "List containers",
    },
    "docker.container.view": {
        "module": "docker",
        "depends_on": ["docker.container.list"],
        "description": "View container details",
    },
    "docker.container.logs": {
        "module": "docker",
        "depends_on": ["docker.container.list"],
        "description": "View container logs",
    },
    "docker.container.stats": {
        "module": "docker",
        "depends_on": ["docker.container.list"],
        "description": "View container stats",
    },

    // Container - Execute
    "docker.container.exec": {
        "module": "docker",
        "depends_on": ["docker.container.list"],
        "description": "Execute in containers",
    },
    "docker.container.attach": {
        "module": "docker",
        "depends_on": ["docker.container.list"],
        "description": "Attach to containers",
    },
    "docker.container.copy": {
        "module": "docker",
        "depends_on": ["docker.container.list"],
        "description": "Copy files to/from containers",
    },

    // Container - Manage
    "docker.container.create": {
        "module": "docker",
        "depends_on": ["docker.container.list"],
        "description": "Create containers",
    },
    "docker.container.start": {
        "module": "docker",
        "depends_on": ["docker.container.list"],
        "description": "Start containers",
    },
    "docker.container.stop": {
        "module": "docker",
        "depends_on": ["docker.container.list"],
        "description": "Stop containers",
    },
    "docker.container.restart": {
        "module": "docker",
        "depends_on": ["docker.container.list"],
        "description": "Restart containers",
    },
    "docker.container.pause": {
        "module": "docker",
        "depends_on": ["docker.container.list"],
        "description": "Pause/Unpause containers",
    },
    "docker.container.kill": {
        "module": "docker",
        "depends_on": ["docker.container.list"],
        "description": "Kill containers",
    },
    "docker.container.delete": {
        "module": "docker",
        "depends_on": ["docker.container.list"],
        "description": "Delete containers",
    },
    "docker.container.update": {
        "module": "docker",
        "depends_on": ["docker.container.list"],
        "description": "Update container config",
    },
    "docker.container.prune": {
        "module": "docker",
        "depends_on": ["docker.container.delete"],
        "description": "Prune stopped containers",
    },

    // Image - Read
    "docker.image.list": {
        "module": "docker",
        "depends_on": ["docker.connect"],
        "description": "List images",
    },
    "docker.image.view": {
        "module": "docker",
        "depends_on": ["docker.image.list"],
        "description": "View image details",
    },
    "docker.image.history": {
        "module": "docker",
        "depends_on": ["docker.image.list"],
        "description": "View image history",
    },

    // Image - Manage
    "docker.image.pull": {
        "module": "docker",
        "depends_on": ["docker.image.list"],
        "description": "Pull images",
    },
    "docker.image.push": {
        "module": "docker",
        "depends_on": ["docker.image.list"],
        "description": "Push images",
    },
    "docker.image.build": {
        "module": "docker",
        "depends_on": ["docker.image.list"],
        "description": "Build images",
    },
    "docker.image.tag": {
        "module": "docker",
        "depends_on": ["docker.image.list"],
        "description": "Tag images",
    },
    "docker.image.delete": {
        "module": "docker",
        "depends_on": ["docker.image.list"],
        "description": "Delete images",
    },
    "docker.image.prune": {
        "module": "docker",
        "depends_on": ["docker.image.delete"],
        "description": "Prune unused images",
    },
    "docker.image.import": {
        "module": "docker",
        "depends_on": ["docker.image.list"],
        "description": "Import/Export images",
    },

    // Volume
    "docker.volume.list": {
        "module": "docker",
        "depends_on": ["docker.connect"],
        "description": "List volumes",
    },
    "docker.volume.view": {
        "module": "docker",
        "depends_on": ["docker.volume.list"],
        "description": "View volume details",
    },
    "docker.volume.create": {
        "module": "docker",
        "depends_on": ["docker.volume.list"],
        "description": "Create volumes",
    },
    "docker.volume.delete": {
        "module": "docker",
        "depends_on": ["docker.volume.list"],
        "description": "Delete volumes",
    },
    "docker.volume.prune": {
        "module": "docker",
        "depends_on": ["docker.volume.delete"],
        "description": "Prune unused volumes",
    },

    // Network
    "docker.network.list": {
        "module": "docker",
        "depends_on": ["docker.connect"],
        "description": "List networks",
    },
    "docker.network.view": {
        "module": "docker",
        "depends_on": ["docker.network.list"],
        "description": "View network details",
    },
    "docker.network.create": {
        "module": "docker",
        "depends_on": ["docker.network.list"],
        "description": "Create networks",
    },
    "docker.network.delete": {
        "module": "docker",
        "depends_on": ["docker.network.list"],
        "description": "Delete networks",
    },
    "docker.network.connect": {
        "module": "docker",
        "depends_on": ["docker.network.list", "docker.container.list"],
        "description": "Connect containers to networks",
    },
    "docker.network.disconnect": {
        "module": "docker",
        "depends_on": ["docker.network.list", "docker.container.list"],
        "description": "Disconnect containers from networks",
    },
    "docker.network.prune": {
        "module": "docker",
        "depends_on": ["docker.network.delete"],
        "description": "Prune unused networks",
    },

    // System
    "docker.system.info": {
        "module": "docker",
        "depends_on": ["docker.connect"],
        "description": "View system info",
    },
    "docker.system.events": {
        "module": "docker",
        "depends_on": ["docker.connect"],
        "description": "Monitor system events",
    },
    "docker.system.df": {
        "module": "docker",
        "depends_on": ["docker.connect"],
        "description": "View disk usage",
    },
    "docker.system.prune": {
        "module": "docker",
        "depends_on": ["docker.connect"],
        "description": "System-wide prune",
    },

    // Registry
    "docker.registry.login": {
        "module": "docker",
        "depends_on": ["docker.connect"],
        "description": "Login to registry",
    },
    "docker.registry.search": {
        "module": "docker",
        "depends_on": ["docker.connect"],
        "description": "Search registry",
    },
}
```

---

## Database Module - Complete Feature Set

### 10.1 MySQL Features

**Connection Management:**
- Connect to MySQL server
- Use vault identities for credentials
- SSL/TLS support
- SSH tunnel support
- Connection pooling
- Multiple database connections

**Query & Execution:**
- SQL query editor with syntax highlighting
- Execute SELECT queries
- Execute INSERT/UPDATE/DELETE
- Execute DDL (CREATE/ALTER/DROP)
- Multiple query execution
- Query timeout configuration
- Explain query execution plan
- Query profiling

**Database Browser:**
- List databases
- Create database
- Drop database
- Switch database
- View database size
- Character set and collation

**Table Management:**
- List tables
- Create table
- Alter table structure
- Drop table
- Rename table
- Truncate table
- View table structure (columns, types, attributes)
- View table indexes
- View table foreign keys
- View table triggers
- View table size
- Table statistics

**Data Browser:**
- Browse table data with pagination
- Filter rows (WHERE conditions)
- Sort columns
- Search across columns
- Edit rows inline
- Insert new rows
- Delete rows
- Bulk operations

**Schema Tools:**
- View indexes (create, drop, analyze)
- View foreign keys
- View triggers (create, drop, enable/disable)
- View stored procedures
- View functions
- View views (create, drop)
- View events

**Import/Export:**
- Export results to CSV
- Export results to JSON
- Export results to Excel
- Export results to SQL
- Import data from CSV
- SQL dump export
- SQL dump import

**User & Permissions:**
- List MySQL users
- Create users
- Grant/Revoke privileges
- View user permissions
- Change user password

**Server Management:**
- View server status
- View server variables
- View processlist (kill queries)
- View slow query log
- View binary logs
- Flush logs/privileges/tables

### 10.2 PostgreSQL Features

**Connection Management:**
- Connect to PostgreSQL server
- Use vault identities
- SSL/TLS modes (disable, require, verify-ca, verify-full)
- SSH tunnel support
- Connection pooling
- Multiple schemas

**Query & Execution:**
- SQL query editor with PostgreSQL syntax
- Execute queries
- Transaction management (BEGIN, COMMIT, ROLLBACK)
- Savepoints
- Prepared statements
- Query explain/analyze
- Query planner visualization

**Database Browser:**
- List databases
- Create database
- Drop database
- Database templates
- View database encoding
- Database statistics

**Schema Management:**
- List schemas
- Create schema
- Drop schema
- Set search path
- Schema permissions

**Table Management:**
- List tables (public and all schemas)
- Create table with constraints
- Alter table
- Drop table
- Table inheritance
- Partitioned tables
- View table statistics
- Analyze table

**Data Types:**
- Support for PostgreSQL-specific types (ARRAY, JSON, JSONB, HSTORE, UUID, etc.)
- Custom types/domains
- Enum types
- Range types

**Advanced Features:**
- Sequences (create, alter, drop, currval, nextval)
- Views (create, drop, materialized views)
- Functions (PL/pgSQL, SQL, etc.)
- Stored procedures
- Triggers
- Rules
- Constraints (check, unique, primary key, foreign key)

**Extensions:**
- List installed extensions
- Install extensions
- View available extensions
- PostGIS support (if installed)

**User & Roles:**
- List roles/users
- Create role
- Grant/Revoke privileges
- Role membership
- Row-level security policies

**Import/Export:**
- Export to CSV/JSON/Excel
- pg_dump integration
- COPY command support
- pg_restore integration

**Server Management:**
- View server settings
- View active connections
- Kill connections
- View locks
- View statistics (pg_stat views)
- Vacuum/Analyze
- Reindex

### 10.3 MongoDB Features

**Connection Management:**
- Connect to MongoDB server/cluster
- Use vault identities
- Replica set support
- Sharded cluster support
- SSL/TLS support
- SSH tunnel support
- Authentication mechanisms (SCRAM, X.509, LDAP)

**Database Browser:**
- List databases
- Create database
- Drop database
- Database statistics
- Storage engine info

**Collection Management:**
- List collections
- Create collection
- Drop collection
- Rename collection
- Collection statistics
- Capped collections
- View collection indexes
- Create/Drop indexes (single, compound, text, geospatial)
- Index statistics

**Document Operations:**
- Browse documents with pagination
- Insert document (JSON editor)
- Update document
- Delete document
- Bulk operations
- Find with query (MongoDB query syntax)
- Sort/Limit/Skip
- Projection

**Query Features:**
- MongoDB query editor
- Aggregation pipeline builder (visual)
- Find operations
- Count documents
- Distinct values
- Map-Reduce operations
- Text search

**Aggregation:**
- Visual pipeline builder
- Stage-by-stage execution
- Pipeline templates
- Export pipeline as code
- Aggregation explain

**Schema Tools:**
- Schema analyzer (infer schema from documents)
- Schema validation rules
- JSON schema support

**Import/Export:**
- Export to JSON
- Export to CSV
- Import from JSON
- Import from CSV
- mongodump/mongorestore support

**User & Security:**
- List users
- Create user
- Update user roles
- Drop user
- Built-in roles
- Custom roles

**Server Management:**
- Server status
- Current operations
- Kill operations
- Profiler
- Server logs

### 10.4 Redis Features

**Connection Management:**
- Connect to Redis server
- Use vault identities
- Redis Sentinel support
- Redis Cluster support
- SSL/TLS support
- SSH tunnel support
- Connection pooling

**Key Browser:**
- List keys (with pattern matching)
- Search keys (SCAN command)
- Key count
- Key type detection
- Key TTL display
- Key memory usage
- Database selector (DB 0-15)

**Key Operations:**
- Get key value
- Set key value
- Delete key
- Rename key
- Set TTL/Expire
- Persist key (remove TTL)
- Type-specific operations

**Data Type Support:**
- String (GET, SET, APPEND, INCR, DECR)
- Hash (HGET, HSET, HDEL, HGETALL, HINCRBY)
- List (LPUSH, RPUSH, LPOP, RPOP, LRANGE, LINDEX)
- Set (SADD, SREM, SMEMBERS, SINTER, SUNION, SDIFF)
- Sorted Set (ZADD, ZREM, ZRANGE, ZRANK, ZSCORE)
- Bitmap operations
- HyperLogLog
- Geospatial indexes
- Stream (XADD, XREAD, XRANGE)

**Command Execution:**
- Redis CLI (execute any Redis command)
- Command history
- Command auto-complete
- Command documentation

**Pub/Sub:**
- Subscribe to channels
- Publish messages
- Pattern subscriptions
- Monitor pub/sub activity

**Server Management:**
- Server info
- Memory stats
- CPU stats
- Keyspace statistics
- Replication info
- Client list
- Kill client connections
- Config get/set
- Slow log
- Monitor command execution

**Persistence:**
- View RDB/AOF status
- Trigger BGSAVE
- Trigger BGREWRITEAOF
- View last save time

**Import/Export:**
- Export keys to JSON
- Import keys from JSON
- RDB file download/upload

### Database Permissions (Complete)

```go
DATABASE_PERMISSIONS = {
    // Connection
    "database.connect": {
        "module": "database",
        "depends_on": ["vault.view"],
        "description": "Connect to databases",
    },

    // Query - Read
    "database.query.read": {
        "module": "database",
        "depends_on": ["database.connect"],
        "description": "Execute SELECT queries",
    },
    "database.query.explain": {
        "module": "database",
        "depends_on": ["database.query.read"],
        "description": "Explain query execution",
    },

    // Query - Write
    "database.query.write": {
        "module": "database",
        "depends_on": ["database.query.read"],
        "description": "Execute INSERT/UPDATE/DELETE",
    },
    "database.query.transaction": {
        "module": "database",
        "depends_on": ["database.query.write"],
        "description": "Manage transactions",
    },

    // Query - DDL
    "database.query.ddl": {
        "module": "database",
        "depends_on": ["database.query.write"],
        "description": "Execute DDL (CREATE/ALTER/DROP)",
    },

    // Schema - View
    "database.schema.view": {
        "module": "database",
        "depends_on": ["database.connect"],
        "description": "View database schema",
    },
    "database.table.list": {
        "module": "database",
        "depends_on": ["database.schema.view"],
        "description": "List tables",
    },
    "database.table.structure": {
        "module": "database",
        "depends_on": ["database.table.list"],
        "description": "View table structure",
    },

    // Schema - Manage
    "database.table.create": {
        "module": "database",
        "depends_on": ["database.schema.view"],
        "description": "Create tables",
    },
    "database.table.alter": {
        "module": "database",
        "depends_on": ["database.table.structure"],
        "description": "Alter tables",
    },
    "database.table.drop": {
        "module": "database",
        "depends_on": ["database.table.list"],
        "description": "Drop tables",
    },
    "database.index.manage": {
        "module": "database",
        "depends_on": ["database.table.structure"],
        "description": "Manage indexes",
    },

    // Data Browser
    "database.data.browse": {
        "module": "database",
        "depends_on": ["database.table.list"],
        "description": "Browse table data",
    },
    "database.data.edit": {
        "module": "database",
        "depends_on": ["database.data.browse", "database.query.write"],
        "description": "Edit data inline",
    },
    "database.data.delete": {
        "module": "database",
        "depends_on": ["database.data.browse", "database.query.write"],
        "description": "Delete data",
    },

    // Import/Export
    "database.export": {
        "module": "database",
        "depends_on": ["database.query.read"],
        "description": "Export query results/data",
    },
    "database.import": {
        "module": "database",
        "depends_on": ["database.query.write"],
        "description": "Import data",
    },
    "database.dump": {
        "module": "database",
        "depends_on": ["database.schema.view", "database.query.read"],
        "description": "Database dump (backup)",
    },
    "database.restore": {
        "module": "database",
        "depends_on": ["database.query.ddl", "database.query.write"],
        "description": "Database restore",
    },

    // User Management
    "database.user.list": {
        "module": "database",
        "depends_on": ["database.connect"],
        "description": "List database users",
    },
    "database.user.create": {
        "module": "database",
        "depends_on": ["database.user.list"],
        "description": "Create database users",
    },
    "database.user.grant": {
        "module": "database",
        "depends_on": ["database.user.list"],
        "description": "Grant/Revoke privileges",
    },

    // Server Management
    "database.server.status": {
        "module": "database",
        "depends_on": ["database.connect"],
        "description": "View server status",
    },
    "database.server.variables": {
        "module": "database",
        "depends_on": ["database.connect"],
        "description": "View server variables",
    },
    "database.server.processes": {
        "module": "database",
        "depends_on": ["database.server.status"],
        "description": "View/Kill processes",
    },
    "database.server.logs": {
        "module": "database",
        "depends_on": ["database.server.status"],
        "description": "View server logs",
    },

    // MongoDB-specific
    "database.mongodb.aggregate": {
        "module": "database",
        "depends_on": ["database.query.read"],
        "description": "Execute aggregation pipelines",
    },
    "database.mongodb.index": {
        "module": "database",
        "depends_on": ["database.table.structure"],
        "description": "Manage MongoDB indexes",
    },

    // Redis-specific
    "database.redis.cli": {
        "module": "database",
        "depends_on": ["database.connect"],
        "description": "Execute Redis commands",
    },
    "database.redis.pubsub": {
        "module": "database",
        "depends_on": ["database.connect"],
        "description": "Pub/Sub operations",
    },
    "database.redis.keys": {
        "module": "database",
        "depends_on": ["database.connect"],
        "description": "Manage Redis keys",
    },
}
```

---

**End of Expanded Features**
