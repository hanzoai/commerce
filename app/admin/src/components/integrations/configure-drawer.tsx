'use client'

import { useEffect, useMemo } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { Button, Drawer, Input, Label, Select, Text, Textarea, toast } from '@hanzo/commerce-ui'
import type { Provider } from '@/lib/integrations/catalog'
import type { CommerceIntegration, IntegrationInput } from '@/lib/api/data-provider'

type Values = Record<string, string>

function schemaFor(provider: Provider) {
  const shape: Record<string, z.ZodTypeAny> = {}
  for (const field of provider.fields) {
    shape[field.key] = field.required
      ? z.string().min(1, `${field.label} is required`)
      : z.string().optional().or(z.literal(''))
  }
  return z.object(shape)
}

export interface ConfigureDrawerProps {
  provider: Provider | null
  integration?: CommerceIntegration
  open: boolean
  onOpenChange: (open: boolean) => void
  onSubmit: (input: IntegrationInput) => Promise<unknown>
  pending?: boolean
}

export function ConfigureDrawer({
  provider,
  integration,
  open,
  onOpenChange,
  onSubmit,
  pending,
}: ConfigureDrawerProps) {
  const resolver = useMemo(() => (provider ? zodResolver(schemaFor(provider)) : undefined), [provider])

  const form = useForm<Values>({ resolver, defaultValues: {} })
  const { register, handleSubmit, reset, setValue, watch, formState } = form

  // Re-seed the form each time the drawer opens for a provider. Secret values are
  // never returned by the API, so credential fields start blank on re-configure;
  // non-secret fields (IDs, hosts, environment) rehydrate from the saved row.
  useEffect(() => {
    if (!open || !provider) return
    const saved = (integration?.data ?? {}) as Record<string, unknown>
    const seed: Values = {}
    for (const field of provider.fields) {
      const value = saved[field.key]
      seed[field.key] = field.secret || value == null ? '' : String(value)
    }
    reset(seed)
  }, [open, provider, integration, reset])

  if (!provider) return null

  const submit = handleSubmit(async (values) => {
    // Drop blank optionals so a re-configure that leaves a secret untouched does
    // not overwrite the stored KMS value with an empty string.
    const data: Record<string, string> = {}
    for (const [key, value] of Object.entries(values)) {
      if (value !== undefined && value !== '') data[key] = value
    }
    try {
      await onSubmit({
        id: integration?.id,
        type: provider.type,
        enabled: true,
        data,
      })
      toast.success(`${provider.name} connected`)
      onOpenChange(false)
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : `Could not connect ${provider.name}`)
    }
  })

  return (
    <Drawer open={open} onOpenChange={onOpenChange}>
      <Drawer.Content>
        <Drawer.Header>
          <Drawer.Title>
            <span className="mr-2" aria-hidden>{provider.emoji}</span>
            {integration ? 'Configure' : 'Connect'} {provider.name}
          </Drawer.Title>
          <Drawer.Description>
            Credentials are encrypted in Hanzo KMS at /tenants/&#123;org&#125;/{provider.type}/*. They are
            never stored in plaintext.
          </Drawer.Description>
        </Drawer.Header>
        <Drawer.Body className="flex flex-col gap-y-5 overflow-y-auto">
          {provider.fields.map((field) => {
            const error = formState.errors[field.key]
            if (field.kind === 'select') {
              const value = watch(field.key)
              return (
                <div key={field.key} className="flex flex-col gap-y-2">
                  <Label size="small" weight="plus">{field.label}</Label>
                  <Select value={value} onValueChange={(v) => setValue(field.key, v, { shouldValidate: true })}>
                    <Select.Trigger>
                      <Select.Value placeholder="Select…" />
                    </Select.Trigger>
                    <Select.Content>
                      {(field.options ?? []).map((opt) => (
                        <Select.Item key={opt.value} value={opt.value}>{opt.label}</Select.Item>
                      ))}
                    </Select.Content>
                  </Select>
                  {field.help && <Text size="xsmall" className="text-ui-fg-muted">{field.help}</Text>}
                  {error && <Text size="xsmall" className="text-ui-fg-error">{String(error.message)}</Text>}
                </div>
              )
            }
            return (
              <div key={field.key} className="flex flex-col gap-y-2">
                <Label size="small" weight="plus">{field.label}</Label>
                <Input
                  type={field.secret ? 'password' : 'text'}
                  autoComplete={field.secret ? 'new-password' : 'off'}
                  placeholder={field.secret && integration ? '•••••• (unchanged)' : field.placeholder}
                  {...register(field.key)}
                />
                {field.help && <Text size="xsmall" className="text-ui-fg-muted">{field.help}</Text>}
                {error && <Text size="xsmall" className="text-ui-fg-error">{String(error.message)}</Text>}
              </div>
            )
          })}
          {provider.fields.length === 0 && (
            <Text size="small" className="text-ui-fg-muted">
              No credentials required. Enabling connects {provider.name} with platform defaults.
            </Text>
          )}
        </Drawer.Body>
        <Drawer.Footer>
          <Drawer.Close asChild>
            <Button variant="secondary" size="small">Cancel</Button>
          </Drawer.Close>
          <Button size="small" isLoading={pending} onClick={submit}>
            {integration ? 'Save' : 'Connect'}
          </Button>
        </Drawer.Footer>
      </Drawer.Content>
    </Drawer>
  )
}
