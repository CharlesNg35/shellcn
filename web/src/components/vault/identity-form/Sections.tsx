import { type ReactNode } from 'react'
import { Controller, useWatch, type Control, type UseFormRegister } from 'react-hook-form'
import { Input } from '@/components/ui/Input'
import { Textarea } from '@/components/ui/Textarea'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/Select'
import { Checkbox } from '@/components/ui/Checkbox'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { cn } from '@/lib/utils/cn'
import { CredentialFieldInput } from './CredentialFieldInput'
import { isCredentialFieldVisible } from '@/lib/vault/credentialFieldVisibility'
import type { CredentialField, CredentialTemplateRecord, IdentityScope } from '@/types/vault'
import type { IdentityFormErrors, IdentityFormValues } from '../IdentityFormModal'

interface FormSectionProps {
  title: string
  description?: string
  actions?: ReactNode
  children: ReactNode
}

export function FormSection({ title, description, actions, children }: FormSectionProps) {
  return (
    <section className="rounded-lg border border-border bg-card p-5 shadow-sm">
      <div className="mb-4 flex flex-wrap items-start justify-between gap-3">
        <div className="space-y-1">
          <h2 className="text-base font-semibold text-foreground">{title}</h2>
          {description ? <p className="text-sm text-muted-foreground">{description}</p> : null}
        </div>
        {actions ? <div className="flex-shrink-0">{actions}</div> : null}
      </div>
      <div className="space-y-4">{children}</div>
    </section>
  )
}

interface IdentityDetailsSectionProps {
  register: UseFormRegister<IdentityFormValues>
  control: Control<IdentityFormValues>
  errors: IdentityFormErrors
  defaultScope: IdentityScope
  mode: 'create' | 'edit'
}

export function IdentityDetailsSection({
  register,
  control,
  errors,
  defaultScope,
  mode,
}: IdentityDetailsSectionProps) {
  return (
    <FormSection
      title="Identity Details"
      description="Define how this credential is labeled and shared across teams."
    >
      <div className="grid gap-4 sm:grid-cols-2">
        <div className="space-y-2">
          <label htmlFor="identity-name" className="text-sm font-medium">
            Name
          </label>
          <Input id="identity-name" placeholder="Production SSH key" {...register('name')} />
          {errors.name ? (
            <p className="text-xs text-destructive">{errors.name.message}</p>
          ) : (
            <p className="text-xs text-muted-foreground">
              Choose a descriptive name so collaborators can quickly identify this credential.
            </p>
          )}
        </div>
        <div className="space-y-2">
          <label htmlFor="identity-scope" className="text-sm font-medium">
            Scope
          </label>
          <Controller
            name="scope"
            control={control}
            render={({ field }) => (
              <Select
                disabled={mode === 'edit'}
                value={(field.value as IdentityScope) ?? defaultScope}
                onValueChange={(value) => field.onChange(value as IdentityScope)}
              >
                <SelectTrigger id="identity-scope" className="h-10 w-full justify-between">
                  <SelectValue placeholder="Select scope" />
                </SelectTrigger>
                <SelectContent align="start">
                  <SelectItem value="global">Global</SelectItem>
                  <SelectItem value="team">Team</SelectItem>
                  <SelectItem value="connection">Connection</SelectItem>
                </SelectContent>
              </Select>
            )}
          />
          <p className="text-xs text-muted-foreground">
            {mode === 'edit'
              ? 'Scope cannot be changed after creation.'
              : 'Scopes control who can access the credential by default.'}
          </p>
        </div>
      </div>
      <div className="space-y-2">
        <label htmlFor="identity-description" className="text-sm font-medium">
          Description
        </label>
        <Textarea
          id="identity-description"
          rows={3}
          placeholder="Optional summary to help collaborators understand this credential"
          {...register('description')}
        />
        {errors.description ? (
          <p className="text-xs text-destructive">{errors.description.message}</p>
        ) : (
          <p className="text-xs text-muted-foreground">
            Include notes or rotation policies to give extra context to other operators.
          </p>
        )}
      </div>
    </FormSection>
  )
}

interface TemplateSelectionSectionProps {
  control: Control<IdentityFormValues>
  templates: CredentialTemplateRecord[]
  teams: Array<{ id: string; name: string }>
  disableTeamSelect: boolean
  selectedTemplate?: CredentialTemplateRecord
}

export function TemplateSelectionSection({
  control,
  templates,
  teams,
  disableTeamSelect,
  selectedTemplate,
}: TemplateSelectionSectionProps) {
  return (
    <FormSection
      title="Template & Ownership"
      description="Select a credential template and assign a default owner when needed."
    >
      <div className="grid gap-4 sm:grid-cols-2">
        <div className="space-y-2">
          <label htmlFor="identity-template" className="text-sm font-medium">
            Credential template
          </label>
          <Controller
            name="template_id"
            control={control}
            render={({ field }) => (
              <Select value={field.value ?? ''} onValueChange={field.onChange}>
                <SelectTrigger id="identity-template" className="h-10 w-full justify-between">
                  <SelectValue placeholder="Custom (JSON)" />
                </SelectTrigger>
                <SelectContent align="start">
                  <SelectItem value="">Custom (JSON)</SelectItem>
                  {templates.map((tpl) => (
                    <SelectItem key={tpl.id} value={tpl.id}>
                      {tpl.display_name} · {tpl.driver_id}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            )}
          />
        </div>
        <div className="space-y-2">
          <label htmlFor="identity-team" className="text-sm font-medium">
            Team (team scope)
          </label>
          <Controller
            name="team_id"
            control={control}
            render={({ field }) => (
              <Select
                disabled={disableTeamSelect}
                value={field.value ?? ''}
                onValueChange={field.onChange}
              >
                <SelectTrigger
                  id="identity-team"
                  className={cn(
                    'h-10 w-full justify-between',
                    disableTeamSelect && 'bg-muted text-muted-foreground'
                  )}
                >
                  <SelectValue placeholder="No team" />
                </SelectTrigger>
                <SelectContent align="start">
                  <SelectItem value="">No team</SelectItem>
                  {teams.map((team) => (
                    <SelectItem key={team.id} value={team.id}>
                      {team.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            )}
          />
          <p className="text-xs text-muted-foreground">
            {disableTeamSelect
              ? 'Switch to the Team scope to select a default owner.'
              : 'Choose which team manages this credential by default.'}
          </p>
        </div>
      </div>
      {selectedTemplate ? (
        <TemplateSummary template={selectedTemplate} />
      ) : (
        <p className="text-sm text-muted-foreground">
          No template selected. Provide a fully custom credential payload below.
        </p>
      )}
    </FormSection>
  )
}

interface TemplateFieldsSectionProps {
  control: Control<IdentityFormValues>
  template: CredentialTemplateRecord
  mode: 'create' | 'edit'
  rotatePayload: boolean
}

export function TemplateFieldsSection({
  control,
  template,
  mode,
  rotatePayload,
}: TemplateFieldsSectionProps) {
  const payloadValues = useWatch({ control, name: 'payload' }) ?? {}
  const visibleFields = template.fields.filter((field) =>
    isCredentialFieldVisible(field, payloadValues as Record<string, unknown>)
  )

  return (
    <FormSection
      title="Credential Fields"
      description="Supply the values required by the template. Secrets remain encrypted at rest."
      actions={
        mode === 'edit' ? (
          <Controller
            name="rotate_payload"
            control={control}
            render={({ field }) => (
              <label className="flex items-center gap-2 text-sm text-foreground">
                <Checkbox
                  checked={Boolean(field.value)}
                  onCheckedChange={(checked) => field.onChange(Boolean(checked))}
                />
                Rotate credentials
              </label>
            )}
          />
        ) : null
      }
    >
      {visibleFields.length ? (
        <div className="grid gap-4 sm:grid-cols-2">
          {visibleFields.map((field: CredentialField) => (
            <CredentialFieldInput
              key={field.name}
              field={field}
              control={control}
              disabled={mode === 'edit' && !rotatePayload}
            />
          ))}
        </div>
      ) : (
        <p className="text-sm text-muted-foreground">
          No credential fields are required for the current configuration.
        </p>
      )}
    </FormSection>
  )
}

interface CustomPayloadSectionProps {
  register: UseFormRegister<IdentityFormValues>
}

export function CustomPayloadSection({ register }: CustomPayloadSectionProps) {
  return (
    <FormSection
      title="Credential Payload"
      description="No template selected. Provide the full credential payload as JSON."
    >
      <div className="space-y-2">
        <Textarea
          id="identity-custom-payload"
          rows={6}
          placeholder='{"username":"admin","password":"••••"}'
          {...register('customPayload')}
        />
        <p className="text-xs text-muted-foreground">
          The payload must be valid JSON. Secrets are encrypted after submission.
        </p>
      </div>
    </FormSection>
  )
}

interface MetadataSectionProps {
  register: UseFormRegister<IdentityFormValues>
  error?: string
}

export function MetadataSection({ register, error }: MetadataSectionProps) {
  return (
    <FormSection
      title="Metadata"
      description="Attach optional metadata (JSON) to aid discovery, notes, or automation."
    >
      <div className="space-y-2">
        <Textarea
          id="identity-metadata"
          rows={4}
          placeholder='{"notes":"Optional metadata"}'
          {...register('metadata')}
        />
        {error ? (
          <p className="text-xs text-destructive">{error}</p>
        ) : (
          <p className="text-xs text-muted-foreground">
            Provide a valid JSON object. This metadata is visible to collaborators with access.
          </p>
        )}
      </div>
    </FormSection>
  )
}

interface FormActionsProps {
  onCancel: () => void
  isSubmitting: boolean
  mode: 'create' | 'edit'
  formError: string | null
}

export function FormActions({ onCancel, isSubmitting, mode, formError }: FormActionsProps) {
  return (
    <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      {formError ? (
        <p className="text-sm text-destructive">{formError}</p>
      ) : (
        <p className="text-xs text-muted-foreground">
          All credentials are encrypted using workspace-grade key management.
        </p>
      )}
      <div className="flex justify-end gap-3 sm:justify-start">
        <Button type="button" variant="outline" onClick={onCancel} disabled={isSubmitting}>
          Cancel
        </Button>
        <Button type="submit" disabled={isSubmitting}>
          {isSubmitting ? 'Saving…' : mode === 'create' ? 'Create identity' : 'Save changes'}
        </Button>
      </div>
    </div>
  )
}

function TemplateSummary({ template }: { template: CredentialTemplateRecord }) {
  const metadata = (template.metadata ?? {}) as Record<string, unknown>
  const metadataEntries = Object.entries(metadata).filter(
    ([, value]) => value !== undefined && value !== null
  )

  const sections = Array.isArray(metadata.sections)
    ? (metadata.sections as Array<Record<string, string>>)
    : []

  return (
    <div className="rounded-md border border-dashed border-border/70 bg-muted/40 p-4 text-sm">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <p className="text-sm font-semibold text-foreground">{template.display_name}</p>
          <p className="text-xs text-muted-foreground">
            Driver {template.driver_id} · Version {template.version}
          </p>
        </div>
        {template.compatible_protocols.length ? (
          <div className="flex flex-wrap gap-2">
            {template.compatible_protocols.map((protocol) => (
              <Badge key={protocol} variant="secondary">
                {protocol.toUpperCase()}
              </Badge>
            ))}
          </div>
        ) : null}
      </div>
      {template.description ? (
        <p className="mt-3 text-sm text-muted-foreground">{template.description}</p>
      ) : null}
      {sections.length ? (
        <dl className="mt-4 grid gap-3 sm:grid-cols-2">
          {sections.map((section) => (
            <div key={section.id} className="space-y-1">
              <dt className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                {section.label ?? formatMetadataKey(section.id ?? '')}
              </dt>
              <dd className="text-sm text-foreground">{section.description ?? '—'}</dd>
            </div>
          ))}
        </dl>
      ) : null}
      {metadataEntries.length ? (
        <dl className="mt-4 grid gap-3 sm:grid-cols-2">
          {metadataEntries
            .filter(([key]) => key !== 'sections' && key !== 'defaults')
            .map(([key, value]) => (
              <div key={key} className="space-y-1">
                <dt className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                  {formatMetadataKey(key)}
                </dt>
                <dd className="text-sm text-foreground">{formatMetadataValue(value)}</dd>
              </div>
            ))}
        </dl>
      ) : null}
    </div>
  )
}

function formatMetadataKey(key: string): string {
  return key
    .replace(/[_-]+/g, ' ')
    .replace(/\s+/g, ' ')
    .trim()
    .replace(/\b\w/g, (char) => char.toUpperCase())
}

function formatMetadataValue(value: unknown): string {
  if (value === null || value === undefined) {
    return 'Not set'
  }
  if (typeof value === 'string') {
    return value
  }
  if (typeof value === 'number') {
    return value.toString()
  }
  if (typeof value === 'boolean') {
    return value ? 'Yes' : 'No'
  }
  if (Array.isArray(value)) {
    if (value.length === 0) {
      return 'Not set'
    }
    return value.map((item) => (typeof item === 'string' ? item : JSON.stringify(item))).join(', ')
  }
  return JSON.stringify(value)
}
