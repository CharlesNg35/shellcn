import { z } from 'zod'

export const localAuthSettingsSchema = z.object({
  allowRegistration: z.boolean(),
  requireEmailVerification: z.boolean(),
  allowPasswordReset: z.boolean(),
})

export const oidcConfigSchema = z.object({
  issuer: z.string().trim().url('Issuer must be a valid URL'),
  clientId: z.string().trim().min(1, 'Client ID is required'),
  clientSecret: z.string().trim().min(1, 'Client secret is required'),
  redirectUrl: z.string().trim().url('Redirect URL must be a valid URL'),
  scopes: z.string().trim().min(1, 'At least one scope is required'),
  enabled: z.boolean(),
  allowRegistration: z.boolean(),
})

const optionalUrl = z
  .string()
  .optional()
  .transform((value) => value?.trim() ?? '')
  .refine((value) => !value || /^https?:\/\//i.test(value), {
    message: 'Must be a valid URL',
  })

export const samlConfigSchema = z.object({
  metadataUrl: optionalUrl,
  entityId: z.string().trim().min(1, 'Entity ID is required'),
  ssoUrl: z.string().trim().url('SSO URL must be a valid URL'),
  acsUrl: z.string().trim().url('ACS URL must be a valid URL'),
  certificate: z.string().trim().min(1, 'Certificate is required'),
  privateKey: z.string().trim().min(1, 'Private key is required'),
  attributeMapping: z.string().optional(),
  enabled: z.boolean(),
  allowRegistration: z.boolean(),
})

export const ldapConfigSchema = z.object({
  host: z.string().trim().min(1, 'Host is required'),
  port: z.coerce.number().int().positive('Port must be a positive number'),
  baseDn: z.string().trim().min(1, 'Base DN is required'),
  bindDn: z.string().trim().min(1, 'Bind DN is required'),
  bindPassword: z.string().trim().min(1, 'Bind password is required'),
  userFilter: z.string().trim().min(1, 'User filter is required'),
  useTls: z.boolean(),
  skipVerify: z.boolean(),
  attributeMapping: z.string().optional(),
  syncGroups: z.boolean(),
  enabled: z.boolean(),
  allowRegistration: z.boolean(),
})
