import { useRef, type ChangeEvent, type ClipboardEvent, type MutableRefObject } from 'react'
import { Controller, type Control } from 'react-hook-form'
import { toast } from '@/lib/utils/toast'
import { cn } from '@/lib/utils/cn'
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
import type { CredentialField } from '@/types/vault'
import type { IdentityFormValues } from '../IdentityFormModal'

interface CredentialFieldInputProps {
  field: CredentialField
  control: Control<IdentityFormValues>
  disabled?: boolean
}

export function CredentialFieldInput({ field, control, disabled }: CredentialFieldInputProps) {
  const fieldName = `payload.${field.name}` as const
  const fileInputRef: MutableRefObject<HTMLInputElement | null> = useRef(null)

  if (field.type === 'boolean') {
    return (
      <Controller
        name={fieldName}
        control={control}
        defaultValue={Boolean(field.default_value)}
        render={({ field: controllerField }) => (
          <label
            className={cn(
              'flex items-start gap-3 rounded-md border border-border p-3',
              disabled ? 'bg-muted text-muted-foreground' : ''
            )}
          >
            <Checkbox
              checked={Boolean(controllerField.value)}
              onCheckedChange={(checked) => controllerField.onChange(Boolean(checked))}
              disabled={disabled}
            />
            <div className="space-y-1">
              <p className="text-sm font-medium text-foreground">{field.label ?? field.name}</p>
              {field.description ? (
                <p className="text-xs text-muted-foreground">{field.description}</p>
              ) : null}
              {field.metadata?.hint ? (
                <p className="text-xs text-muted-foreground">{String(field.metadata.hint)}</p>
              ) : null}
            </div>
          </label>
        )}
      />
    )
  }

  if (field.type === 'enum' && Array.isArray(field.options)) {
    const defaultValue = getEnumDefault(field)
    return (
      <div className="space-y-2">
        <label htmlFor={`field-${field.name}`} className="text-sm font-medium">
          {field.label ?? field.name}
        </label>
        <Controller
          name={fieldName}
          control={control}
          defaultValue={defaultValue}
          render={({ field: controllerField }) => (
            <Select
              disabled={disabled}
              value={(controllerField.value as string) ?? ''}
              onValueChange={(value) => controllerField.onChange(value)}
            >
              <SelectTrigger id={`field-${field.name}`} className="h-10 w-full justify-between">
                <SelectValue placeholder="Select option" />
              </SelectTrigger>
              <SelectContent align="start">
                {field.options?.map((option) => {
                  if (typeof option === 'string') {
                    return (
                      <SelectItem key={option} value={option}>
                        {option}
                      </SelectItem>
                    )
                  }
                  const rawValue = option.value
                  const label =
                    typeof option.label === 'string' ? option.label : String(option.value ?? '')
                  return (
                    <SelectItem key={String(rawValue)} value={String(rawValue ?? '')}>
                      {label}
                    </SelectItem>
                  )
                })}
              </SelectContent>
            </Select>
          )}
        />
        {field.description ? (
          <p className="text-xs text-muted-foreground">{field.description}</p>
        ) : null}
        {field.metadata?.hint ? (
          <p className="text-xs text-muted-foreground">{String(field.metadata.hint)}</p>
        ) : null}
      </div>
    )
  }

  const allowFileImport =
    field.input_modes?.includes('file') ||
    field.type === 'file' ||
    field.metadata?.allow_file_import === true
  const usesTextarea =
    field.input_modes?.includes('textarea') ||
    field.type === 'secret' ||
    field.type === 'file' ||
    (field.input_modes?.length ?? 0) === 0
  const InputComponent = usesTextarea ? Textarea : Input
  const inputProps = buildInputProps(field, usesTextarea)

  const handleClipboardBlock = (event: ClipboardEvent<HTMLInputElement | HTMLTextAreaElement>) => {
    if (field.type !== 'secret') {
      return
    }
    event.preventDefault()
    toast.warning('Copy disabled', {
      description: 'Secret values cannot be copied from the vault form.',
    })
  }

  const handleFileImport = (
    event: ChangeEvent<HTMLInputElement>,
    onChange: (value: string) => void
  ) => {
    const input = event.target
    const file = input.files?.[0]
    if (!file) {
      return
    }
    const reader = new FileReader()
    reader.onload = () => {
      const result = typeof reader.result === 'string' ? reader.result : ''
      onChange(result)
      input.value = ''
      toast.success('File imported', {
        description: `${file.name} loaded into ${field.label ?? field.name}.`,
      })
    }
    reader.onerror = () => {
      toast.error('Import failed', {
        description: `Unable to read ${file.name}.`,
      })
    }
    reader.readAsText(file)
  }

  return (
    <div className="space-y-2">
      <Controller
        name={fieldName}
        control={control}
        defaultValue={getDefaultValue(field)}
        render={({ field: controllerField }) => (
          <>
            <div className="flex items-center justify-between gap-3">
              <label htmlFor={`field-${field.name}`} className="text-sm font-medium">
                {field.label ?? field.name}
                {field.required ? <span className="ml-1 text-destructive">*</span> : null}
              </label>
              {allowFileImport ? (
                <>
                  <input
                    ref={fileInputRef}
                    type="file"
                    className="hidden"
                    accept={buildFileAccept(field)}
                    onChange={(event) => handleFileImport(event, controllerField.onChange)}
                  />
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    disabled={disabled}
                    onClick={() => fileInputRef.current?.click()}
                  >
                    Import file
                  </Button>
                </>
              ) : null}
            </div>

            <InputComponent
              id={`field-${field.name}`}
              disabled={disabled}
              placeholder={resolvePlaceholder(field)}
              value={(controllerField.value as string) ?? ''}
              onChange={(event: ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) =>
                controllerField.onChange(event.target.value)
              }
              autoComplete={field.type === 'secret' ? 'off' : inputProps.autoComplete}
              onCopy={field.type === 'secret' ? handleClipboardBlock : undefined}
              onCut={field.type === 'secret' ? handleClipboardBlock : undefined}
              {...inputProps.props}
            />
          </>
        )}
      />
      {field.description ? (
        <p className="text-xs text-muted-foreground">{field.description}</p>
      ) : null}
      {field.metadata?.hint ? (
        <p className="text-xs text-muted-foreground">{String(field.metadata.hint)}</p>
      ) : null}
      {field.type === 'secret' && disabled ? (
        <p className="text-xs text-muted-foreground">
          Hidden for security. Enable rotation to update this secret.
        </p>
      ) : null}
    </div>
  )
}

function getEnumDefault(field: CredentialField): string {
  if (typeof field.default_value === 'string') {
    return field.default_value
  }
  if (Array.isArray(field.options) && field.options.length > 0) {
    const first = field.options[0]
    if (typeof first === 'string') {
      return first
    }
    if (first && typeof first === 'object' && 'value' in first) {
      return String(first.value ?? '')
    }
  }
  return ''
}

function getDefaultValue(field: CredentialField): string {
  if (typeof field.default_value === 'string') {
    return field.default_value
  }
  if (typeof field.default_value === 'number' || typeof field.default_value === 'boolean') {
    return String(field.default_value)
  }
  return ''
}

function resolvePlaceholder(field: CredentialField): string {
  if (typeof field.placeholder === 'string' && field.placeholder.length > 0) {
    return field.placeholder
  }
  if (typeof field.description === 'string') {
    return field.description
  }
  return ''
}

function buildInputProps(
  field: CredentialField,
  usesTextarea: boolean
): {
  autoComplete?: string
  props: Record<string, unknown>
} {
  if (usesTextarea) {
    const rawRows = field.metadata?.rows as unknown
    const metadataRows = typeof rawRows === 'number' ? Math.max(3, rawRows) : undefined
    const rows =
      metadataRows ??
      (field.input_modes?.includes('textarea')
        ? 6
        : field.type === 'secret' || field.type === 'file'
          ? 6
          : 3)
    return { autoComplete: undefined, props: { rows } }
  }

  if (field.type === 'number') {
    return { autoComplete: 'off', props: { type: 'number', inputMode: 'numeric' } }
  }

  if (field.type === 'secret') {
    return { autoComplete: 'off', props: { type: 'password' } }
  }

  if (field.metadata?.input_type === 'email') {
    return { autoComplete: 'email', props: { type: 'email' } }
  }

  return { autoComplete: 'off', props: { type: 'text' } }
}

function buildFileAccept(field: CredentialField): string | undefined {
  if (!field.metadata) {
    return undefined
  }
  const accept = field.metadata.accept
  if (typeof accept === 'string') {
    return accept
  }
  if (Array.isArray(accept)) {
    return accept.filter((item) => typeof item === 'string').join(',')
  }
  return undefined
}
