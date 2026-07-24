import { TaxRegionDetail } from '@/components/tax-regions/tax-region-detail'

// `output: export` requires a non-empty param set for a dynamic segment. Tax
// regions are per-org runtime data with no build-time ids, so we emit a single
// throwaway placeholder shell; the real detail view resolves its id from the
// route params and fetches the record client-side.
export function generateStaticParams(): { id: string }[] {
  return [{ id: 'placeholder' }]
}

export default function TaxRegionDetailPage() {
  return <TaxRegionDetail />
}
