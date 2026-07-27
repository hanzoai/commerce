/**
 * Every merchant list route — ONE file.
 *
 * The catalog in `~/lib/resources` says what a resource is; this route renders it
 * with the shared `CommerceResource`. `generateStaticParams` emits one static page
 * per catalog row, so adding a surface is adding a row, never a directory.
 */
import { RESOURCES } from '@/lib/resources'
import { ResourceList } from './resource-list'

export function generateStaticParams() {
  return RESOURCES.map((r) => ({ resource: r.slug }))
}

export default async function ResourcePage({ params }: { params: Promise<{ resource: string }> }) {
  const { resource } = await params
  return <ResourceList slug={resource} />
}
