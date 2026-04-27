import { NextResponse } from "next/server"

import { saveUploadForPad } from "@/lib/server/upload-store"

export async function POST(
  request: Request,
  { params }: { params: Promise<{ slug: string }> },
) {
  const { slug } = await params
  const form = await request.formData()
  const file = form.get("file")

  if (!(file instanceof File)) {
    return NextResponse.json(
      { error: "campo file obrigatorio" },
      { status: 400 },
    )
  }

  try {
    const upload = await saveUploadForPad(slug, file)
    return NextResponse.json(upload, { status: 201 })
  } catch (error) {
    const message =
      error instanceof Error ? error.message : "falha ao salvar upload"
    const status =
      message.includes("limite") || message.includes("vazio") ? 400 : 500
    return NextResponse.json({ error: message }, { status })
  }
}
