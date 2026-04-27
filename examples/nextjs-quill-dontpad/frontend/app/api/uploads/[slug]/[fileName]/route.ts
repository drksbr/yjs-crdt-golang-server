import { NextResponse } from "next/server"

import { readUploadForPad } from "@/lib/server/upload-store"

export async function GET(
  _request: Request,
  { params }: { params: Promise<{ slug: string; fileName: string }> },
) {
  const { slug, fileName } = await params

  try {
    const file = await readUploadForPad(slug, fileName)
    return new NextResponse(file.file, {
      headers: {
        "content-type": "application/octet-stream",
        "content-length": String(file.size),
        "content-disposition": `attachment; filename="${file.downloadName}"`,
        "cache-control": "private, max-age=31536000, immutable",
      },
    })
  } catch {
    return NextResponse.json({ error: "arquivo nao encontrado" }, { status: 404 })
  }
}
