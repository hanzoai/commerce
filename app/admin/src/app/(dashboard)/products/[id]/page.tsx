import { ProductDetail } from './product-detail'

export default function ProductDetailPage({ params }: { params: Promise<{ id: string }> }) {
  return <ProductDetail params={params} />
}
