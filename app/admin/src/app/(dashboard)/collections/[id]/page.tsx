import { CollectionDetail } from '@/components/collections/collection-detail'

// `output: export` requires a non-empty param set for a dynamic segment (an empty
// array fails the export with "missing generateStaticParams()"). Collections are
// per-org runtime data with no build-time ids, so we emit a single throwaway
// placeholder shell; the real detail view resolves its id from the route params
// and fetches the record client-side. Any unknown id renders the not-found state.
export function generateStaticParams(): { id: string }[] {
  return [{ id: 'placeholder' }]
}

export default function CollectionDetailPage() {
  return <CollectionDetail />
}
