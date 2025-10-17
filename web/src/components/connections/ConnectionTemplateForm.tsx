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
import type {
  ConnectionTemplate,
  ConnectionTemplateField,
  ConnectionTemplateFieldType,
} from '@/types/protocols'
import { isFieldVisible, type TemplateValueMap } from './connectionTemplateHelpers'

interface ConnectionTemplateFormProps {
  template: ConnectionTemplate
  values: TemplateValueMap
  errors: Record<string, string>
  disabled?: boolean
  onChange: (fieldKey: string, value: unknown) => void
  onFieldBlur?: (fieldKey: string) => void
}

export function ConnectionTemplateForm({
  template,
  values,
  errors,
  disabled = false,
  onChange,
  onFieldBlur,
}: ConnectionTemplateFormProps) {
  if (!template.sections.length) {
    return null
  }

  return (
    <div className="space-y-6">
      {template.sections.map((section) => {
        const visibleFields = section.fields.filter((field) => isFieldVisible(field, values))
        if (visibleFields.length === 0) {
          return null
        }
        return (
          <div
            key={section.id}
            className="rounded-lg border border-border/60 bg-muted/10 px-4 py-5 sm:px-5"
          >
            <div className="mb-4 space-y-1.5">
              <h3 className="text-sm font-semibold text-foreground">{section.label}</h3>
              {section.description ? (
                <p className="text-xs text-muted-foreground">{section.description}</p>
              ) : null}
            </div>
            <div className="space-y-4">
              {visibleFields.map((field) => (
                <FieldInput
                  key={field.key}
                  field={field}
                  value={values[field.key]}
                  error={errors[field.key]}
                  disabled={disabled}
                  onChange={onChange}
                  onBlur={onFieldBlur}
                />
              ))}
            </div>
          </div>
        )
      })}
    </div>
  )
}

interface FieldInputProps {
  field: ConnectionTemplateField
  value: unknown
  error?: string
  disabled: boolean
  onChange: (fieldKey: string, value: unknown) => void
  onBlur?: (fieldKey: string) => void
}

function FieldInput({ field, value, error, disabled, onChange, onBlur }: FieldInputProps) {
  const handleBlur = () => {
    onBlur?.(field.key)
  }

  const inputId = `template-field-${field.key}`

  switch (field.type as ConnectionTemplateFieldType) {
    case 'string':
    case 'target_host':
      return (
        <LabeledField field={field} error={error} inputId={inputId}>
          <Input
            id={inputId}
            value={stringValue(value)}
            onChange={(event) => onChange(field.key, event.target.value)}
            onBlur={handleBlur}
            placeholder={field.placeholder}
            disabled={disabled}
          />
        </LabeledField>
      )
    case 'multiline':
      return (
        <LabeledField field={field} error={error} inputId={inputId}>
          <Textarea
            id={inputId}
            value={stringValue(value)}
            onChange={(event) => onChange(field.key, event.target.value)}
            onBlur={handleBlur}
            placeholder={field.placeholder}
            disabled={disabled}
            rows={4}
          />
        </LabeledField>
      )
    case 'json':
      return (
        <LabeledField field={field} error={error} inputId={inputId}>
          <Textarea
            id={inputId}
            value={jsonStringValue(value)}
            onChange={(event) => onChange(field.key, event.target.value)}
            onBlur={handleBlur}
            placeholder={field.placeholder}
            disabled={disabled}
            rows={6}
          />
        </LabeledField>
      )
    case 'number':
    case 'target_port':
      return (
        <LabeledField field={field} error={error} inputId={inputId}>
          <Input
            type="number"
            id={inputId}
            value={numericValue(value)}
            onChange={(event) => {
              const inputValue = event.target.value
              if (inputValue === '') {
                onChange(field.key, undefined)
                return
              }
              const nextValue = Number(inputValue)
              if (Number.isNaN(nextValue)) {
                return
              }
              onChange(field.key, nextValue)
            }}
            onBlur={handleBlur}
            placeholder={field.placeholder}
            disabled={disabled}
          />
        </LabeledField>
      )
    case 'boolean':
      return (
        <div className="flex items-start justify-between rounded-md border border-border/60 bg-background px-3 py-3">
          <div className="space-y-1">
            <label className="text-sm font-medium text-foreground" htmlFor={inputId}>
              {field.label}
            </label>
            {field.helpText ? (
              <p className="text-xs text-muted-foreground">{field.helpText}</p>
            ) : null}
            {error ? <p className="text-xs text-rose-500">{error}</p> : null}
          </div>
          <Checkbox
            id={inputId}
            checked={Boolean(value ?? field.default ?? false)}
            onCheckedChange={(checked) => onChange(field.key, Boolean(checked))}
            onBlur={handleBlur}
            disabled={disabled}
            className="mt-1"
          />
        </div>
      )
    case 'select':
      return (
        <LabeledField field={field} error={error} inputId={inputId}>
          <Select
            value={stringValue(value) ?? ''}
            onValueChange={(next) => onChange(field.key, next || undefined)}
            disabled={disabled}
          >
            <SelectTrigger className="h-10 w-full justify-between">
              <SelectValue
                placeholder={field.placeholder ?? `Select ${field.label.toLowerCase()}`}
              />
            </SelectTrigger>
            <SelectContent align="start">
              {(field.options ?? []).map((option) => (
                <SelectItem key={option.value} value={option.value}>
                  {option.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </LabeledField>
      )
    default:
      return (
        <LabeledField field={field} error={error} inputId={inputId}>
          <Input
            id={inputId}
            value={stringValue(value)}
            onChange={(event) => onChange(field.key, event.target.value)}
            onBlur={handleBlur}
            placeholder={field.placeholder}
            disabled={disabled}
          />
        </LabeledField>
      )
  }
}

interface LabeledFieldProps {
  field: ConnectionTemplateField
  error?: string
  inputId?: string
  children: React.ReactNode
}

function LabeledField({ field, error, inputId, children }: LabeledFieldProps) {
  return (
    <div className="space-y-2">
      <div className="space-y-1">
        <label className="block text-sm font-medium text-foreground" htmlFor={inputId}>
          {field.label}
        </label>
        {field.helpText ? <p className="text-xs text-muted-foreground">{field.helpText}</p> : null}
      </div>
      {children}
      {error ? <p className="text-xs text-rose-500">{error}</p> : null}
    </div>
  )
}

function stringValue(value: unknown): string {
  if (typeof value === 'string') {
    return value
  }
  if (value === null || value === undefined) {
    return ''
  }
  return String(value)
}

function numericValue(value: unknown): string {
  if (typeof value === 'number' && Number.isFinite(value)) {
    return String(value)
  }
  if (typeof value === 'string' && value.trim().length > 0) {
    return value
  }
  return ''
}

function jsonStringValue(value: unknown): string {
  if (typeof value === 'string') {
    return value
  }
  if (value === null || value === undefined) {
    return ''
  }
  try {
    return JSON.stringify(value, null, 2)
  } catch {
    return ''
  }
}
