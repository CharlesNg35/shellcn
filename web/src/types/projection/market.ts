import type { Icon, PluginCategoryInfo, Transport } from "./core";

export type ProtocolAvailability = "enabled" | "admin_only" | "disabled";

export interface ProtocolAdminItem {
  name: string;
  title: string;
  icon: Icon;
  category: PluginCategoryInfo;
  version: string;
  transports: Transport[];
  capabilities?: string[];
  risks?: string[];
  recording?: string[];
  external: boolean;
  healthy: boolean;
  availability: ProtocolAvailability;
}

export interface ProtocolAdminList {
  // dir is the server-configured external-plugin directory; empty when disabled.
  dir: string;
  protocols: ProtocolAdminItem[];
}

export interface MarketVersion {
  version: string;
  apiVersion: number;
  protocolVersion: number;
  platforms: string[];
  icon: Icon;
  snapshotUrl: string;
}

export interface MarketEntry {
  name: string;
  displayName: string;
  description: string;
  repo: string;
  homepage?: string;
  license: string;
  maintainers: string[];
  latest?: MarketVersion;
  compatible: boolean;
  installedVersion?: string;
  managed: boolean;
  updateAvailable: boolean;
}

export interface MarketList {
  enabled: boolean;
  plugins: MarketEntry[];
}
