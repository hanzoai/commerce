import { ClaimDetail } from './claim-detail'

// `output: export` requires a dynamic segment to declare its params at build
// time. Claim IDs are only known at runtime, so we prerender none: the list
// navigates here client-side and the claim is fetched in the browser (the static
// host's `--spa` fallback serves this route for deep links). A server component
// is required to export this — the detail UI is the sibling client component.
export function generateStaticParams(): Array<{ id: string }> {
  return [{ id: 'placeholder' }]
}

export default function ClaimDetailPage() {
  return <ClaimDetail />
}
