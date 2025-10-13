import { useState } from 'react'
import { Controller, useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { Input } from '@/components/ui/Input'
import { Button } from '@/components/ui/Button'
import { invitesApi } from '@/lib/api/invites'
import { INVITES_QUERY_KEY } from '@/hooks/useInvites'
import { useQueryClient } from '@tanstack/react-query'
import { toast } from '@/lib/utils/toast'
import { toApiError } from '@/lib/api/http'
import type { InviteCreatePayload, InviteCreateResponse } from '@/types/invites'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/Select'
import { useTeams } from '@/hooks/useTeams'

const inviteSchema = z.object({
  email: z.string().email('A valid email is required').trim(),
  team_id: z.union([z.string().uuid('Select a valid team'), z.literal('')]).optional(),
})

type InviteFormValues = z.infer<typeof inviteSchema>

interface UserInviteFormProps {
  onClose?: () => void
  onCreated?: (invite: InviteCreateResponse) => void
}

export function UserInviteForm({ onClose, onCreated }: UserInviteFormProps) {
  const queryClient = useQueryClient()
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [errorMessage, setErrorMessage] = useState<string | null>(null)
  const [fallbackLink, setFallbackLink] = useState<string | null>(null)
  const { data: teamsData, isLoading: isTeamsLoading } = useTeams({
    staleTime: 60_000,
  })
  const teams = teamsData?.data ?? []

  const {
    register,
    control,
    handleSubmit,
    formState: { errors },
    reset,
  } = useForm<InviteFormValues>({
    resolver: zodResolver(inviteSchema),
    defaultValues: {
      email: '',
      team_id: '',
    },
  })

  const buildInviteLink = (result: InviteCreateResponse) => {
    const base =
      result.link && result.link.startsWith('http')
        ? result.link
        : `${window.location.origin}${result.link ?? `/invite/accept?token=${encodeURIComponent(result.token)}`}`
    return base
  }

  const onSubmit = handleSubmit(async (values) => {
    setErrorMessage(null)
    setFallbackLink(null)
    setIsSubmitting(true)
    try {
      const payload: InviteCreatePayload = {
        email: values.email.trim(),
      }
      const teamId = values.team_id?.trim()
      if (teamId) {
        payload.team_id = teamId
      }

      const result = await invitesApi.create(payload)
      await queryClient.invalidateQueries({ queryKey: INVITES_QUERY_KEY })

      const inviteLink = buildInviteLink(result)
      try {
        await navigator.clipboard.writeText(inviteLink)
        toast.success('Invitation created', {
          description: 'Invite link copied to clipboard',
        })
      } catch {
        toast.success('Invitation created', {
          description: 'Copy the link below to share the invite.',
        })
        setFallbackLink(inviteLink)
      }

      onCreated?.(result)
      reset()
      onClose?.()
    } catch (error) {
      const apiError = toApiError(error)
      setErrorMessage(apiError.message)
    } finally {
      setIsSubmitting(false)
    }
  })

  return (
    <form onSubmit={onSubmit} className="space-y-4">
      <Input
        label="Email"
        type="email"
        placeholder="user@example.com"
        {...register('email')}
        error={errors.email?.message}
      />

      <Controller
        name="team_id"
        control={control}
        render={({ field }) => {
          const value = field.value ?? ''
          const hasTeams = teams.length > 0

          return (
            <div className="space-y-2">
              <label htmlFor="invite-team" className="text-sm font-medium text-foreground">
                Team (optional)
              </label>
              <Select value={value} onValueChange={field.onChange} disabled={isTeamsLoading}>
                <SelectTrigger
                  id="invite-team"
                  className="w-full justify-between"
                  aria-invalid={Boolean(errors.team_id?.message)}
                >
                  <SelectValue
                    placeholder={
                      isTeamsLoading
                        ? 'Loading teams…'
                        : hasTeams
                          ? 'No team selected'
                          : 'No teams available'
                    }
                  />
                </SelectTrigger>
                <SelectContent align="start">
                  <SelectItem value="">No team (default access)</SelectItem>
                  {hasTeams ? null : (
                    <SelectItem value="__no_teams__" disabled>
                      No teams available
                    </SelectItem>
                  )}
                  {teams.map((team) => (
                    <SelectItem key={team.id} value={team.id}>
                      {team.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              {errors.team_id?.message ? (
                <p className="text-sm text-destructive">{errors.team_id.message}</p>
              ) : null}
            </div>
          )
        }}
      />

      {errorMessage ? (
        <div className="rounded border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {errorMessage}
        </div>
      ) : null}

      {fallbackLink ? (
        <div className="rounded border border-border/60 bg-muted/40 px-3 py-2 text-sm text-foreground">
          Invite link: <span className="break-all">{fallbackLink}</span>
        </div>
      ) : null}

      <div className="flex justify-end gap-2">
        <Button type="button" variant="outline" onClick={onClose} disabled={isSubmitting}>
          Cancel
        </Button>
        <Button type="submit" loading={isSubmitting}>
          Send Invite
        </Button>
      </div>
    </form>
  )
}
