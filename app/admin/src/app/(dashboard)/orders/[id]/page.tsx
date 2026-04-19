import { OrderDetail } from './order-detail'

// Required by Next.js `output: 'export'` for dynamic route segments.
// Empty array — actual order IDs resolve client-side at runtime.
export function generateStaticParams() {
  return []
}

export const dynamicParams = true

export default function OrderDetailPage({ params }: { params: Promise<{ id: string }> }) {
  return <OrderDetail params={params} />
}
