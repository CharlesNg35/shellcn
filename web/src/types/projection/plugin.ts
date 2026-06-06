import type { Icon, Layout, PluginCategoryInfo, Transport } from "./core";
import type { CredentialKindInfo, Schema } from "./schema";
import type { Action, PanelConfigSchema, Stream, Tab } from "./panels";
import type { RecordingCapability } from "./recording";
import type { ResourceType, ScopeFilter, TreeGroup } from "./resources";

export interface AgentProfile {
  modes: string[];
  riskNote?: string;
}

export interface PluginSummary {
  name: string;
  title: string;
  icon: Icon;
  category: PluginCategoryInfo;
  description?: string;
}

export interface PluginProjection {
  apiVersion: number;
  name: string;
  version: string;
  title: string;
  description: string;
  icon: Icon;
  category: PluginCategoryInfo;
  config: Schema;
  capabilities: string[];
  credentialKinds?: CredentialKindInfo[];
  supportedTransports: Transport[];
  agent?: AgentProfile;
  layout: Layout;
  tabs?: Tab[];
  tree?: TreeGroup[];
  resources?: ResourceType[];
  actions?: Action[];
  headerActions?: string[];
  scope?: ScopeFilter[];
  streams?: Stream[];
  recording?: RecordingCapability[];
  panelConfigSchemas?: Record<string, PanelConfigSchema>;
}
