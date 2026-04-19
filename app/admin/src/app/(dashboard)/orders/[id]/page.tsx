import { OrderDetail } from './order-detail'

// Required by Next.js `output: 'export'` for dynamic route segments.
// Empty list — actual order IDs resolve at runtime via the SPA fallback.
export async function generateStaticParams(): Promise<Array<{ id: string }>> {
  return []
}

export default function OrderDetailPage({ params }: { params: Promise<{ id: string }> }) {
  return <OrderDetail params={params} />
}
