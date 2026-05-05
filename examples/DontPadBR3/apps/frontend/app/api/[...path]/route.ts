import { NextRequest, NextResponse } from "next/server";

export const dynamic = "force-dynamic";

type RouteContext = { params: Promise<{ path: string[] }> };

function backendBaseURL() {
  const configured = process.env.DONTPAD_BACKEND_URL?.trim();
  return (configured && configured.length > 0
    ? configured
    : "http://127.0.0.1:8080"
  ).replace(/\/+$/, "");
}

async function proxy(request: NextRequest, context: RouteContext) {
  const { path } = await context.params;
  const backendURL = new URL(backendBaseURL());
  const upstream = new URL(`${backendURL.toString().replace(/\/+$/, "")}/api/${path.join("/")}`);
  upstream.search = request.nextUrl.search;

  const headers = new Headers(request.headers);
  headers.delete("host");
  headers.delete("content-length");
  headers.set("x-forwarded-host", backendURL.host);
  headers.set("x-forwarded-proto", backendURL.protocol.replace(":", ""));

  const method = request.method.toUpperCase();
  const hasBody = !["GET", "HEAD"].includes(method);

  const init: RequestInit & { duplex?: "half" } = {
    method,
    headers,
    cache: "no-store",
    redirect: "manual",
  };
  if (hasBody && request.body !== null) {
    init.body = request.body;
    init.duplex = "half";
  }

  const response = await fetch(upstream.toString(), init);

  const responseHeaders = new Headers(response.headers);
  responseHeaders.delete("content-encoding");
  responseHeaders.delete("content-length");

  if (method === "HEAD") {
    return new NextResponse(null, {
      status: response.status,
      headers: responseHeaders,
    });
  }

  return new NextResponse(response.body, {
    status: response.status,
    headers: responseHeaders,
  });
}

export async function GET(request: NextRequest, context: RouteContext) {
  return proxy(request, context);
}

export async function POST(request: NextRequest, context: RouteContext) {
  return proxy(request, context);
}

export async function PUT(request: NextRequest, context: RouteContext) {
  return proxy(request, context);
}

export async function PATCH(request: NextRequest, context: RouteContext) {
  return proxy(request, context);
}

export async function DELETE(request: NextRequest, context: RouteContext) {
  return proxy(request, context);
}

export async function OPTIONS(request: NextRequest, context: RouteContext) {
  return proxy(request, context);
}

export async function HEAD(request: NextRequest, context: RouteContext) {
  return proxy(request, context);
}
