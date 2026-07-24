import { CategoryDetail } from '@/components/categories/detail'

// `output: export` requires a dynamic segment to declare params at build time.
// Categories are per-org runtime data with no build-time ids, so we emit a single
// throwaway placeholder; the client detail view resolves its id from the route and
// fetches the record in the browser. Unknown ids render not-found.
export function generateStaticParams(): { id: string }[] {
  return [{ id: 'placeholder' }]
}

export default function CategoryDetailPage() {
  return <CategoryDetail />
}
