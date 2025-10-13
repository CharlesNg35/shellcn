import { z } from 'zod'

export const loginSchema = z.object({
  identifier: z.string().min(1, 'Username or email is required'),
  password: z.string().min(1, 'Password is required'),
  remember_device: z.boolean().optional(),
})

export const setupSchema = z
  .object({
    username: z.string().min(3, 'Username must be at least 3 characters'),
    email: z.string().email('Invalid email address'),
    password: z
      .string()
      .min(8, 'Password must be at least 8 characters')
      .regex(/[A-Z]/, 'Password must contain at least one uppercase letter')
      .regex(/[a-z]/, 'Password must contain at least one lowercase letter')
      .regex(/[0-9]/, 'Password must contain at least one number'),
    confirmPassword: z.string(),
    firstName: z.string().optional(),
    lastName: z.string().optional(),
  })
  .refine((data) => data.password === data.confirmPassword, {
    message: "Passwords don't match",
    path: ['confirmPassword'],
  })

export const passwordResetRequestSchema = z.object({
  email: z.string().email('Invalid email address'),
})

export const passwordResetConfirmSchema = z
  .object({
    token: z.string().min(1, 'Reset token is required'),
    password: z.string().min(8, 'Password must be at least 8 characters'),
    confirmPassword: z.string().min(1, 'Please confirm your password'),
  })
  .refine((data) => data.password === data.confirmPassword, {
    message: "Passwords don't match",
    path: ['confirmPassword'],
  })

export const mfaVerificationSchema = z.object({
  code: z
    .string()
    .min(6, 'Code must be at least 6 digits')
    .max(10, 'Code must be at most 10 digits'),
})

export const inviteAcceptSchema = z
  .object({
    token: z.string().min(1, 'Invite token is required'),
    existingAccount: z.boolean().optional(),
    username: z.string().max(64).optional(),
    password: z.string().optional(),
    confirmPassword: z.string().optional(),
    firstName: z.string().trim().max(128).optional(),
    lastName: z.string().trim().max(128).optional(),
  })
  .superRefine((data, ctx) => {
    const isExisting = Boolean(data.existingAccount)

    if (isExisting) {
      return
    }

    const username = data.username?.trim() ?? ''
    if (username.length < 3) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'Username must be at least 3 characters',
        path: ['username'],
      })
    }

    if (username.length > 64) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'Username must be at most 64 characters',
        path: ['username'],
      })
    }

    if (!data.password || data.password.length < 8) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'Password must be at least 8 characters',
        path: ['password'],
      })
    }

    if (!data.confirmPassword || data.confirmPassword.length < 8) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'Confirm password is required',
        path: ['confirmPassword'],
      })
      return
    }

    if (data.password !== data.confirmPassword) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: "Passwords don't match",
        path: ['confirmPassword'],
      })
    }
  })
