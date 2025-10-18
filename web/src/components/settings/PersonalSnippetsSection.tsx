import { useEffect, useMemo, useState } from 'react'
import { useForm } from 'react-hook-form'
import { z } from 'zod'
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import { Textarea } from '@/components/ui/Textarea'
import { Modal } from '@/components/ui/Modal'
import { Badge } from '@/components/ui/Badge'
import { useSnippets, SNIPPETS_QUERY_KEY } from '@/hooks/useSnippets'
import { createSnippet, deleteSnippet, updateSnippet, type SnippetRecord } from '@/lib/api/snippets'
import { toast } from 'sonner'

const snippetSchema = z.object({
  name: z.string().trim().min(1, 'Name is required').max(120, 'Name is too long'),
  description: z
    .string()
    .trim()
    .max(500, 'Description must be 500 characters or less')
    .optional()
    .or(z.literal('')),
  command: z.string().trim().min(1, 'Command is required'),
})

type SnippetFormValues = z.infer<typeof snippetSchema>

function toValues(snippet?: SnippetRecord | null): SnippetFormValues {
  if (!snippet) {
    return { name: '', description: '', command: '' }
  }
  return {
    name: snippet.name,
    description: snippet.description ?? '',
    command: snippet.command,
  }
}

export function PersonalSnippetsSection() {
  const { data: snippets = [], isLoading } = useSnippets({ scope: 'user' })
  const queryClient = useQueryClient()
  const [modalOpen, setModalOpen] = useState(false)
  const [editingSnippet, setEditingSnippet] = useState<SnippetRecord | null>(null)

  const form = useForm<SnippetFormValues>({
    resolver: zodResolver(snippetSchema),
    defaultValues: toValues(),
  })

  useEffect(() => {
    if (!modalOpen) {
      form.reset(toValues())
      setEditingSnippet(null)
    }
  }, [form, modalOpen])

  const invalidateSnippets = async () => {
    await queryClient.invalidateQueries({ queryKey: SNIPPETS_QUERY_KEY })
  }

  const createMutation = useMutation({
    mutationFn: (values: SnippetFormValues) =>
      createSnippet({
        name: values.name.trim(),
        description: values.description?.trim() || undefined,
        command: values.command.trim(),
        scope: 'user',
      }),
    onSuccess: async () => {
      await invalidateSnippets()
      toast.success('Snippet created', {
        description: 'Your personal snippet is ready to use.',
      })
    },
    onError: (error: unknown) => {
      toast.error('Unable to create snippet', {
        description: error instanceof Error ? error.message : 'Unexpected error occurred.',
      })
    },
  })

  const updateMutation = useMutation({
    mutationFn: (values: SnippetFormValues) =>
      updateSnippet(editingSnippet!.id, {
        name: values.name.trim(),
        description: values.description?.trim() || undefined,
        command: values.command.trim(),
        scope: 'user',
      }),
    onSuccess: async () => {
      await invalidateSnippets()
      toast.success('Snippet updated', {
        description: 'Changes saved successfully.',
      })
    },
    onError: (error: unknown) => {
      toast.error('Unable to update snippet', {
        description: error instanceof Error ? error.message : 'Unexpected error occurred.',
      })
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (snippetId: string) => deleteSnippet(snippetId),
    onSuccess: async () => {
      await invalidateSnippets()
      toast.success('Snippet removed', {
        description: 'The snippet was deleted successfully.',
      })
    },
    onError: (error: unknown) => {
      toast.error('Unable to delete snippet', {
        description: error instanceof Error ? error.message : 'Unexpected error occurred.',
      })
    },
  })

  const submitting = createMutation.isPending || updateMutation.isPending

  const handleOpenCreate = () => {
    setEditingSnippet(null)
    form.reset(toValues())
    setModalOpen(true)
  }

  const handleEdit = (snippet: SnippetRecord) => {
    setEditingSnippet(snippet)
    form.reset(toValues(snippet))
    setModalOpen(true)
  }

  const handleDelete = async (snippet: SnippetRecord) => {
    await deleteMutation.mutateAsync(snippet.id)
  }

  const onSubmit = form.handleSubmit(async (values) => {
    if (editingSnippet) {
      await updateMutation.mutateAsync(values)
    } else {
      await createMutation.mutateAsync(values)
    }
    setModalOpen(false)
  })

  const emptyState = useMemo(() => snippets.length === 0, [snippets.length])

  return (
    <section className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h3 className="text-sm font-semibold text-foreground">Personal snippets</h3>
          <p className="text-xs text-muted-foreground">
            Store quick commands that only you can run. They appear in the SSH session toolbar.
          </p>
        </div>
        <Button size="sm" variant="outline" onClick={handleOpenCreate}>
          Add snippet
        </Button>
      </div>

      {isLoading ? (
        <div className="rounded-md border border-border/60 bg-muted/10 px-4 py-6 text-sm text-muted-foreground">
          Loading your snippets…
        </div>
      ) : emptyState ? (
        <div className="rounded-md border border-dashed border-border/60 bg-muted/10 px-4 py-6 text-sm text-muted-foreground">
          You haven’t created any personal snippets yet. Use “Add snippet” to capture frequently
          used commands.
        </div>
      ) : (
        <div className="space-y-3">
          {snippets.map((snippet) => (
            <div
              key={snippet.id}
              className="rounded-md border border-border/60 bg-card px-4 py-3 shadow-sm"
            >
              <div className="flex flex-wrap items-start justify-between gap-3">
                <div>
                  <h4 className="text-sm font-semibold text-foreground">{snippet.name}</h4>
                  {snippet.description ? (
                    <p className="text-xs text-muted-foreground">{snippet.description}</p>
                  ) : null}
                </div>
                <div className="flex items-center gap-2">
                  <Badge variant="outline" className="text-[10px] uppercase tracking-wide">
                    Personal
                  </Badge>
                  <Button size="sm" variant="ghost" onClick={() => handleEdit(snippet)}>
                    Edit
                  </Button>
                  <Button
                    size="sm"
                    variant="ghost"
                    className="text-destructive hover:text-destructive"
                    onClick={() => handleDelete(snippet)}
                    disabled={deleteMutation.isPending}
                  >
                    Delete
                  </Button>
                </div>
              </div>
              <pre className="mt-3 rounded bg-muted/40 p-3 text-xs font-mono text-foreground">
                {snippet.command}
              </pre>
            </div>
          ))}
        </div>
      )}

      <Modal
        open={modalOpen}
        onClose={() => setModalOpen(false)}
        title={editingSnippet ? 'Edit personal snippet' : 'Add personal snippet'}
        description="Snippets let you quickly run reusable commands during SSH sessions."
        size="lg"
      >
        <form className="space-y-4" onSubmit={onSubmit} noValidate>
          <Input
            label="Name"
            placeholder="e.g. Tail logs"
            value={form.watch('name')}
            onChange={(event) => form.setValue('name', event.target.value, { shouldDirty: true })}
            error={form.formState.errors.name?.message}
            disabled={submitting}
          />
          <Textarea
            label="Description"
            placeholder="Optional context for this command."
            rows={2}
            value={form.watch('description')}
            onChange={(event) =>
              form.setValue('description', event.target.value, { shouldDirty: true })
            }
            error={form.formState.errors.description?.message}
            disabled={submitting}
          />
          <Textarea
            label="Command"
            placeholder="Enter the command to run…"
            rows={3}
            value={form.watch('command')}
            onChange={(event) =>
              form.setValue('command', event.target.value, { shouldDirty: true })
            }
            error={form.formState.errors.command?.message}
            disabled={submitting}
          />
          <div className="flex justify-end gap-2">
            <Button
              type="button"
              variant="outline"
              onClick={() => setModalOpen(false)}
              disabled={submitting}
            >
              Cancel
            </Button>
            <Button type="submit" loading={submitting}>
              {editingSnippet ? 'Save snippet' : 'Create snippet'}
            </Button>
          </div>
        </form>
      </Modal>
    </section>
  )
}
