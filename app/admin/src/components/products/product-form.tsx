'use client'

import { useState } from 'react'
import dynamic from 'next/dynamic'
import { useRouter } from 'next/navigation'
import { useForm, Controller } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Button, Input, Textarea, Select, Switch, Text, Badge, toast, clx } from '@hanzo/commerce-ui'
import { useCreate, useUpdate, useDelete } from '@/lib/api/hooks'
import { Field, Fieldset } from '@/components/common/field'
import { ImageUpload } from '@/components/common/image-upload'
import { ConfirmButton } from '@/components/common/confirm-button'
import { OptionsEditor } from './options-editor'
import {
  productSchema,
  emptyForm,
  productToForm,
  formToPayload,
  slugify,
  statusOf,
  symbolFor,
  STATUS_COLOR,
  CURRENCIES,
  type Product,
  type ProductFormValues,
} from '@/lib/products/product'

// Variants editor is the heaviest section and unused on many products — split it
// out of the first paint and load on demand.
const VariantsEditor = dynamic(() => import('./variants-editor').then((m) => m.VariantsEditor), {
  ssr: false,
  loading: () => <Text size="small" className="text-ui-fg-muted">Loading variants…</Text>,
})

interface ProductFormProps {
  mode: 'create' | 'edit'
  product?: Product
  /** False renders a read-only view (no writes) for non-admins. */
  canWrite?: boolean
  /** Extra sections (e.g. collection assignment) rendered after variants. */
  extraSections?: React.ReactNode
}

const money = (symbol: string, props: React.ComponentProps<typeof Input>) => (
  <div className="relative">
    <span className="pointer-events-none absolute inset-y-0 left-2.5 flex items-center text-ui-fg-muted">
      {symbol}
    </span>
    <Input inputMode="decimal" placeholder="0.00" className="pl-6" {...props} />
  </div>
)

export function ProductForm({ mode, product, canWrite = true, extraSections }: ProductFormProps) {
  const router = useRouter()
  const readOnly = !canWrite

  const create = useCreate<Product>('product')
  const update = useUpdate<Product>('product')
  const del = useDelete('product')

  const {
    register,
    control,
    handleSubmit,
    setValue,
    watch,
    formState: { errors, isSubmitting },
  } = useForm<ProductFormValues>({
    resolver: zodResolver(productSchema),
    defaultValues: mode === 'edit' && product ? productToForm(product) : emptyForm(),
  })

  // Keep slug in lockstep with name until the user edits it by hand (never in edit mode).
  const [slugLocked, setSlugLocked] = useState(mode === 'edit')
  const nameReg = register('name')
  const slugReg = register('slug')

  const currency = watch('currency')
  const status = statusOf({ available: watch('available'), hidden: watch('hidden') })

  const onSubmit = handleSubmit(async (values) => {
    const payload = formToPayload(values)
    try {
      if (mode === 'create') {
        await create.mutateAsync(payload)
        toast.success('Product created')
      } else {
        await update.mutateAsync({ id: product!.id, data: payload })
        toast.success('Product updated')
      }
      router.push('/products')
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Could not save the product')
    }
  })

  const onDelete = async () => {
    await del.mutateAsync(product!.id)
    toast.success('Product deleted')
    router.push('/products')
  }

  const busy = isSubmitting || create.isPending || update.isPending

  return (
    <form onSubmit={onSubmit} className="mx-auto flex w-full max-w-3xl flex-col gap-y-6 px-4 py-8 sm:px-8">
      <fieldset disabled={readOnly} className="m-0 flex min-w-0 flex-col gap-y-6 border-0 p-0">
        <Fieldset title="General" description="The essentials shoppers see first.">
          <Field label="Name" error={errors.name?.message}>
            <Input
              autoFocus={mode === 'create'}
              placeholder="Premium coffee beans"
              {...nameReg}
              onChange={(e) => {
                nameReg.onChange(e)
                if (!slugLocked) setValue('slug', slugify(e.target.value))
              }}
            />
          </Field>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <Field label="Slug" error={errors.slug?.message} hint="Used in the storefront URL.">
              <Input
                placeholder="premium-coffee-beans"
                {...slugReg}
                onChange={(e) => {
                  setSlugLocked(true)
                  slugReg.onChange(e)
                }}
              />
            </Field>
            <Field label="SKU" error={errors.sku?.message}>
              <Input placeholder="COFFEE-001" {...register('sku')} />
            </Field>
          </div>
          <Field label="UPC / Barcode" optional>
            <Input placeholder="0123456789012" {...register('upc')} />
          </Field>
          <Field label="Headline" optional hint="A short marketing line.">
            <Input placeholder="Single-origin, small-batch roast" {...register('headline')} />
          </Field>
          <Field label="Description" optional>
            <Textarea rows={4} placeholder="What makes this product worth buying?" {...register('description')} />
          </Field>
        </Fieldset>

        <Fieldset
          title="Status & visibility"
          description="Control whether shoppers can see and buy this product."
          actions={<Badge size="2xsmall" color={STATUS_COLOR[status]}>{status}</Badge>}
        >
          <SwitchRow control={control} name="available" label="Available for sale" description="Customers can add it to cart." />
          <SwitchRow control={control} name="hidden" label="Hidden" description="Hide from the storefront entirely." />
          <SwitchRow control={control} name="preorder" label="Pre-order" description="Sell before inventory arrives." />
        </Fieldset>

        <Fieldset title="Pricing" description="Base price used when a variant has none.">
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
            <Field label="Currency">
              <Controller
                control={control}
                name="currency"
                render={({ field: { onChange, value, ref } }) => (
                  <Select value={value} onValueChange={onChange}>
                    <Select.Trigger ref={ref}>
                      <Select.Value placeholder="Currency" />
                    </Select.Trigger>
                    <Select.Content>
                      {CURRENCIES.map((c) => (
                        <Select.Item key={c} value={c}>{c}</Select.Item>
                      ))}
                    </Select.Content>
                  </Select>
                )}
              />
            </Field>
            <Field label="Price" error={errors.price?.message}>
              {money(symbolFor(currency), register('price'))}
            </Field>
            <Field label="Compare-at / MSRP" optional error={errors.msrp?.message}>
              {money(symbolFor(currency), register('msrp'))}
            </Field>
          </div>
          <SwitchRow control={control} name="taxable" label="Taxable" description="Apply tax at checkout." />
        </Fieldset>

        <Fieldset title="Media" description="Upload product images — stored on your Hanzo CDN.">
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <Field label="Primary image" optional hint="The main product thumbnail.">
              <Controller
                control={control}
                name="imageUrl"
                render={({ field: { value, onChange } }) => (
                  <ImageUpload
                    multiple={false}
                    disabled={readOnly}
                    value={value ? [value] : []}
                    onChange={(urls) => onChange(urls[0] ?? '')}
                  />
                )}
              />
            </Field>
            <Field label="Header / banner" optional hint="A wide banner for the product page.">
              <Controller
                control={control}
                name="headerUrl"
                render={({ field: { value, onChange } }) => (
                  <ImageUpload
                    multiple={false}
                    disabled={readOnly}
                    value={value ? [value] : []}
                    onChange={(urls) => onChange(urls[0] ?? '')}
                  />
                )}
              />
            </Field>
          </div>
          <Field label="Gallery" optional hint="Additional product photos.">
            <Controller
              control={control}
              name="gallery"
              render={({ field: { value, onChange } }) => (
                <ImageUpload
                  disabled={readOnly}
                  value={value ? value.split('\n').filter(Boolean) : []}
                  onChange={(urls) => onChange(urls.join('\n'))}
                />
              )}
            />
          </Field>
        </Fieldset>

        <Fieldset title="Options" description="Variant axes like size or color.">
          <OptionsEditor control={control} register={register} disabled={readOnly} />
        </Fieldset>

        <Fieldset title="Variants" description="Individually purchasable SKUs.">
          <VariantsEditor control={control} register={register} currency={currency} disabled={readOnly} />
        </Fieldset>

        {extraSections}
      </fieldset>

      <footer
        className={clx(
          'sticky bottom-0 z-10 -mx-4 flex items-center gap-x-2 border-t border-ui-border-base bg-ui-bg-base px-4 py-3 sm:-mx-8 sm:px-8',
        )}
      >
        {mode === 'edit' && !readOnly && (
          <ConfirmButton
            onConfirm={onDelete}
            title="Delete product"
            description="This permanently removes the product and its variants. This cannot be undone."
          >
            Delete
          </ConfirmButton>
        )}
        <div className="ml-auto flex items-center gap-x-2">
          <Button type="button" variant="secondary" size="small" onClick={() => router.push('/products')}>
            {readOnly ? 'Back' : 'Cancel'}
          </Button>
          {!readOnly && (
            <Button type="submit" size="small" isLoading={busy}>
              {mode === 'create' ? 'Create product' : 'Save changes'}
            </Button>
          )}
        </div>
      </footer>

      {readOnly && (
        <Text size="small" className="text-ui-fg-muted">
          You have read-only access to products. Ask an admin for write access to make changes.
        </Text>
      )}
    </form>
  )
}

// A labeled switch row — one control reused for every boolean on the form.
function SwitchRow({
  control,
  name,
  label,
  description,
}: {
  control: import('react-hook-form').Control<ProductFormValues>
  name: 'available' | 'hidden' | 'preorder' | 'taxable'
  label: string
  description: string
}) {
  return (
    <div className="flex items-start gap-x-3 rounded-lg border border-ui-border-base bg-ui-bg-base p-3">
      <Controller
        control={control}
        name={name}
        render={({ field: { value, onChange } }) => (
          <Switch checked={value} onCheckedChange={onChange} />
        )}
      />
      <div className="min-w-0">
        <Text size="small" weight="plus" className="text-ui-fg-base">{label}</Text>
        <Text size="small" leading="compact" className="text-ui-fg-subtle">{description}</Text>
      </div>
    </div>
  )
}
