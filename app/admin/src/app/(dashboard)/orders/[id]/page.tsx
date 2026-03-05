import { OrderDetail } from './order-detail'

export default function OrderDetailPage({ params }: { params: Promise<{ id: string }> }) {
  return <OrderDetail params={params} />
}
