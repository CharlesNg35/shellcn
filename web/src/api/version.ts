import { api } from "./client";

export interface VersionInfo {
  current: string;
  dev: boolean;
  latest?: string;
  updateAvailable: boolean;
  releaseUrl?: string;
  checkDisabled?: boolean;
  checkedAt?: string;
  error?: string;
}

export const versionApi = {
  get: (refresh = false) =>
    api.get<VersionInfo>(`/version${refresh ? "?refresh=1" : ""}`),
};
