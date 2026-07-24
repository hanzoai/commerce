import { PromotionDetail } from '@/components/promotions/promotion-detail'

// `output: export` requires a non-empty param set for a dynamic segment. Promotions
// are per-org runtime data with no build-time ids, so we emit a single throwaway
// placeholder shell; the real detail view resolves its id from the route params
// and fetches the record client-side. Any unknown id renders the not-found state.
export function generateStaticParams(): { id: string }[] {
  return [{ id: 'placeholder' }]
}

export default function PromotionDetailPage() {
  return <PromotionDetail />
}
