import { z } from 'zod'

const nameSchema = z
  .string()
  .trim()
  .min(2, 'Team name must be at least 2 characters')
  .max(128, 'Team name must be at most 128 characters')

const optionalString = z
  .string()
  .trim()
  .max(512)
  .optional()
  .transform((value) => {
    if (!value) {
      return undefined
    }
    const trimmed = value.trim()
    return trimmed.length ? trimmed : undefined
  })

export const teamCreateSchema = z.object({
  name: nameSchema,
  description: optionalString,
})

export const teamUpdateSchema = z
  .object({
    name: nameSchema.optional(),
    description: optionalString,
  })
  .refine((data) => Boolean(data.name) || Boolean(data.description), {
    message: 'Provide at least one field to update',
    path: ['name'],
  })

export type TeamCreateValues = z.infer<typeof teamCreateSchema>
export type TeamUpdateValues = z.infer<typeof teamUpdateSchema>
