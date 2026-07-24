import { z } from 'zod'

// Domain module for the Hanzo Commerce collection resource (/v1/collection).
// Mirrors the Go model `models/collection` (camelCase: name, slug, description,
// published, available, productIds). One place owns the shape, the validation
// schema, the empty/record mappers, and slug derivation — so the create and edit
// surfaces never diverge. Pure (no React): unit-tests and imports anywhere.

export interface Collection {
  id: string
  name: string
  slug: string
  description?: string
  published: boolean
  available: boolean
  productIds?: string[]
  variantIds?: string[]
  createdAt?: string
  updatedAt?: string
}

export const collectionSchema = z.object({
  name: z.string().trim().min(1, 'Name is required'),
  slug: z
    .string()
    .trim()
    .min(1, 'Handle is required')
    .regex(/^[a-z0-9]+(?:-[a-z0-9]+)*$/, 'Use lowercase letters, numbers, and hyphens'),
  description: z.string(),
  published: z.boolean(),
  available: z.boolean(),
})

export type CollectionValues = z.infer<typeof collectionSchema>

export const emptyCollection: CollectionValues = {
  name: '',
  slug: '',
  description: '',
  published: false,
  available: true,
}

/** API record -> controlled form values (never undefined). */
export function toValues(collection: Collection): CollectionValues {
  return {
    name: collection.name ?? '',
    slug: collection.slug ?? '',
    description: collection.description ?? '',
    published: collection.published ?? false,
    available: collection.available ?? true,
  }
}

/** Form values -> trimmed API payload. Membership (productIds) is edited on its
 *  own panel, so it is intentionally not part of the general-details payload. */
export function toPayload(values: CollectionValues): Partial<Collection> {
  return {
    name: values.name.trim(),
    slug: values.slug.trim(),
    description: values.description.trim(),
    published: values.published,
    available: values.available,
  }
}

/** Human string -> url-safe handle. */
export function slugify(value: string): string {
  return value
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-|-$/g, '')
}
