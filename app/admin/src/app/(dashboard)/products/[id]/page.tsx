import { ProductDetail } from './product-detail'

// Required by Next.js `output: 'export'` for dynamic route segments.
export function generateStaticParams() {
  return []
}


export default function ProductDetailPage({ params }: { params: Promise<{ id: string }> }) {
  return <ProductDetail params={params} />
}
