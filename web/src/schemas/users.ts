import { z } from 'zod'

const optionalString = z
  .string()
  .trim()
  .max(256)
  .optional()
  .transform((value) => (value === '' ? undefined : value))

export const userBaseSchema = z.object({
  username: z.string().min(3).max(64),
  email: z.string().email(),
  first_name: optionalString,
  last_name: optionalString,
  avatar: optionalString,
  is_active: z.boolean().optional(),
  is_root: z.boolean().optional(),
})

export const userCreateSchema = userBaseSchema.extend({
  password: z.string().min(8),
})

export const userUpdateSchema = userBaseSchema.partial().extend({
  username: z.string().min(3).max(64).optional(),
  email: z.string().email().optional(),
  password: z.string().min(8).optional(),
})

export type UserFormValues = z.infer<typeof userCreateSchema>
export type UserUpdateValues = z.infer<typeof userUpdateSchema>
