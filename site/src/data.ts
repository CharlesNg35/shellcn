export const CAT_COLOR: Record<string, string> = {
  Shell: "#38bdf8",
  Files: "#22d3ee",
  Containers: "#60a5fa",
  Virtualization: "#a78bfa",
  Desktop: "#f472b6",
  Databases: "#34d399",
  Observability: "#fbbf24",
  Directory: "#94a3b8",
};

export const PROTOCOLS: [string, string][] = [
  ["SSH", "Shell"], ["SFTP", "Files"], ["FTP", "Files"], ["FTPS", "Files"],
  ["SMB", "Files"], ["WebDAV", "Files"], ["S3", "Files"], ["Docker", "Containers"],
  ["Swarm", "Containers"], ["Podman", "Containers"], ["Kubernetes", "Containers"],
  ["Proxmox", "Virtualization"], ["VNC", "Desktop"], ["RDP", "Desktop"],
  ["PostgreSQL", "Databases"], ["MySQL", "Databases"], ["MongoDB", "Databases"],
  ["Redis", "Databases"], ["Monitoring", "Observability"], ["LDAP", "Directory"],
];

export type Feature = { icon: string; title: string; body: string };
export const FEATURES: Feature[] = [
  { icon: "shield", title: "RBAC & ownership grants", body: "Every route carries a permission and risk tier, enforced by Casbin plus per-connection ownership and sharing grants. Secrets are AES-256-GCM encrypted and write-only." },
  { icon: "record", title: "Session recording", body: "Terminal sessions record as asciicast v2; remote desktops capture graphically. Plugin-declared, off by default, and private to their creator." },
  { icon: "network", title: "Reach private networks", body: "A tiny reverse-tunnel agent dials back from inside a target behind NAT or a firewall — transport is orthogonal to protocol, invisible to the plugin." },
  { icon: "binary", title: "Single self-contained binary", body: "Pure-Go dependencies, an embedded Vue frontend, SQLite by default. Even RDP decodes in-process — no external daemon required." },
  { icon: "globe", title: "Browser-native everything", body: "xterm.js terminals, noVNC desktops, CodeMirror editors, live metrics and log tails — all rendered from the manifest projection." },
  { icon: "plug", title: "Extensible & audited", body: "Out-of-tree plugins run as sandboxed gRPC subprocesses with no ambient authority. Every action is authn → authz → validate → audit → handler." },
];

export type Tab = { id: string; label: string; code: string };
export const TABS: Tab[] = [
  {
    id: "docker",
    label: "Docker",
    code: `docker run -d --name shellcn -p 8081:8081 \\
  -v shellcn-data:/data \\
  -e SHELLCN_MASTER_KEY="$(openssl rand -base64 32)" \\
  -e SHELLCN_BOOTSTRAP_ADMIN_USERNAME=admin \\
  -e SHELLCN_BOOTSTRAP_ADMIN_PASSWORD=change-me \\
  ghcr.io/charlesng35/shellcn:latest`,
  },
  {
    id: "compose",
    label: "Docker Compose",
    code: `services:
  shellcn:
    image: ghcr.io/charlesng35/shellcn:latest
    ports: ["8081:8081"]
    environment:
      SHELLCN_MASTER_KEY: \${SHELLCN_MASTER_KEY}
      SHELLCN_BOOTSTRAP_ADMIN_USERNAME: admin
      SHELLCN_BOOTSTRAP_ADMIN_PASSWORD: change-me
    volumes: ["shellcn-data:/data"]
    restart: unless-stopped

volumes:
  shellcn-data:`,
  },
  {
    id: "binary",
    label: "Binary",
    code: `export SHELLCN_MASTER_KEY="$(openssl rand -base64 32)"
export SHELLCN_BOOTSTRAP_ADMIN_USERNAME=admin
export SHELLCN_BOOTSTRAP_ADMIN_PASSWORD=change-me
./shellcn      # serves on :8081, data in the working dir`,
  },
];

export const REPO = "https://github.com/CharlesNg35/shellcn";
