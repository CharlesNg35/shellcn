import { api } from "./client";

export interface TotpSetup {
  // secret is the base32 string for manual entry; qr is a ready-to-render PNG
  // data URL of the otpauth URL.
  secret: string;
  otpauthUrl: string;
  qr: string;
}

export interface RecoveryCodes {
  recoveryCodes: string[];
}

export const totpApi = {
  setup: () => api.post<TotpSetup>("/auth/totp/setup"),
  enable: (code: string) =>
    api.post<RecoveryCodes>("/auth/totp/enable", { code }),
  disable: (code: string) => api.post("/auth/totp/disable", { code }),
  regenerateRecoveryCodes: (code: string) =>
    api.post<RecoveryCodes>("/auth/totp/recovery-codes", { code }),
  remind: () => api.post("/auth/totp/remind"),
};
