export interface ProfileUpdatePayload {
  username?: string
  email?: string
  first_name?: string
  last_name?: string
  avatar?: string
}

export interface PasswordChangePayload {
  current_password: string
  new_password: string
}

export interface TotpCodePayload {
  code: string
}

export interface MfaSetupResponse {
  secret: string
  otpauth_url: string
  qr_code: string
  backup_codes: string[]
}
