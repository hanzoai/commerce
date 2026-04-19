import { ProductDetail } from './product-detail'

// Required by Next.js `output: 'export'` for dynamic route segments.
// Empty list — actual product IDs resolve at runtime via the SPA fallback.
export async function generateStaticParams(): Promise<Array<{ id: string }>> {
  return []
}

export default function ProductDetailPage({ params }: { params: Promise<{ id: string }> }) {
  return <ProductDetail params={params} />
}
