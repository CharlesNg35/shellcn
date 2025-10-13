import { z } from 'zod'

export const profileUpdateSchema = z.object({
  username: z
    .string()
    .trim()
    .min(1, 'Username is required')
    .min(3, 'Username must be at least 3 characters')
    .max(64, 'Username cannot exceed 64 characters'),
  email: z.string().trim().min(1, 'Email is required').email('Enter a valid email address'),
  first_name: z
    .string()
    .trim()
    .max(128, 'First name cannot exceed 128 characters')
    .optional()
    .or(z.literal('')),
  last_name: z
    .string()
    .trim()
    .max(128, 'Last name cannot exceed 128 characters')
    .optional()
    .or(z.literal('')),
  avatar: z.string().trim().url('Avatar must be a valid URL').optional().or(z.literal('')),
})

export type ProfileUpdateFormValues = z.infer<typeof profileUpdateSchema>

export const passwordChangeSchema = z
  .object({
    current_password: z
      .string()
      .min(1, 'Current password is required')
      .min(8, 'Current password must be at least 8 characters'),
    new_password: z
      .string()
      .min(1, 'New password is required')
      .min(8, 'New password must be at least 8 characters'),
  })
  .refine((data) => data.current_password !== data.new_password, {
    path: ['new_password'],
    message: 'New password must differ from the current password',
  })

export type PasswordChangeFormValues = z.infer<typeof passwordChangeSchema>

export const totpCodeSchema = z.object({
  code: z
    .string()
    .trim()
    .min(1, 'Verification code is required')
    .regex(/^[0-9]{6}$/u, 'Enter the 6-digit code from your authenticator'),
})

export type TotpCodeFormValues = z.infer<typeof totpCodeSchema>
