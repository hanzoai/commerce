import { CustomerGroupDetail } from '@/components/customer-groups/customer-group-detail'

// `output: export` requires a dynamic segment to declare its params at build time.
// Group IDs are per-org runtime data, so none are prerendered: the list navigates
// here client-side and the record is fetched in the browser. This server wrapper
// exists only to satisfy the static-export contract; all behaviour lives in the
// sibling client component.
export function generateStaticParams(): { id: string }[] {
  return [{ id: 'placeholder' }]
}

export default function CustomerGroupDetailPage() {
  return <CustomerGroupDetail />
}
