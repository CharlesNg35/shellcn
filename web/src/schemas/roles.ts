import { z } from 'zod'

const nameSchema = z
  .string()
  .trim()
  .min(2, 'Role name must be at least 2 characters')
  .max(64, 'Role name must be 64 characters or fewer')

const descriptionSchema = z
  .string()
  .trim()
  .max(240, 'Description must be 240 characters or fewer')
  .optional()
  .transform((value) => (value ? value : undefined))

export const roleCreateSchema = z.object({
  name: nameSchema,
  description: descriptionSchema,
  is_system: z.boolean().optional(),
})

export const roleUpdateSchema = roleCreateSchema
  .partial({
    name: true,
    description: true,
    is_system: true,
  })
  .refine(
    (value) => Boolean(value.name) || Boolean(value.description),
    'Provide a name or description to update'
  )

export type RoleCreateSchema = z.infer<typeof roleCreateSchema>
export type RoleUpdateSchema = z.infer<typeof roleUpdateSchema>
