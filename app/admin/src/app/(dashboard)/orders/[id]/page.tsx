import { OrderDetail } from './order-detail'

// `output: export` requires a dynamic segment to declare its params at build
// time. Order IDs are only known at runtime, so we prerender none: the list
// navigates here client-side and the detail is fetched in the browser. (A
// server component is required to export this — the detail UI is the sibling
// client component.)
export function generateStaticParams(): Array<{ id: string }> {
  return [{ id: 'placeholder' }]
}

export default function OrderDetailPage() {
  return <OrderDetail />
}
