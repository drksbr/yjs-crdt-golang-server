import { redirect } from "next/navigation"

import { CollaborativePad } from "@/components/collaborative-pad"
import { normalizePadSlug } from "@/lib/pad-slug"

export default async function PadPage({
  params,
}: {
  params: Promise<{ slug: string }>
}) {
  const { slug } = await params
  const normalizedSlug = normalizePadSlug(slug)

  if (normalizedSlug !== slug) {
    redirect(`/${normalizedSlug}`)
  }

  return <CollaborativePad slug={normalizedSlug} />
}
