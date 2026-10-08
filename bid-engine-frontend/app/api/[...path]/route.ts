export const runtime = "nodejs";

function getUpstreamOrigin() {
  const raw =
    process.env.UPSTREAM_ORIGIN ||
    process.env.NEXT_PUBLIC_SYSTEM_SERVER ||
    "http://localhost:1022";
  try {
    const u = new URL(raw);
    return u.origin;
  } catch {
    return "http://localhost:1022";
  }
}

function rewriteSetCookie(cookie: string) {
  return cookie.replace(/;\s*Domain=[^;]+/i, "");
}

async function proxy(req: Request, path: string[]) {
  const reqUrl = new URL(req.url);
  const upstream = new URL(`${getUpstreamOrigin()}/api/${path.join("/")}`);
  upstream.search = reqUrl.search;

  const headers = new Headers(req.headers);
  headers.delete("host");
  headers.delete("connection");
  headers.delete("content-length");

  const method = req.method.toUpperCase();
  const body =
    method === "GET" || method === "HEAD" ? undefined : await req.arrayBuffer();

  const upstreamRes = await fetch(upstream, {
    method,
    headers,
    body,
    redirect: "manual",
  });

  const outHeaders = new Headers(upstreamRes.headers);
  outHeaders.delete("set-cookie");

  const getSetCookie = (upstreamRes.headers as any)?.getSetCookie;
  const setCookies: string[] =
    typeof getSetCookie === "function"
      ? getSetCookie.call(upstreamRes.headers)
      : upstreamRes.headers.get("set-cookie")
        ? [String(upstreamRes.headers.get("set-cookie"))]
        : [];

  for (const c of setCookies) {
    if (!c) continue;
    outHeaders.append("set-cookie", rewriteSetCookie(c));
  }

  // SSE（text/event-stream）必须流式透传，禁止整体 arrayBuffer() 缓冲
  const contentType = outHeaders.get("content-type") || "";
  if (contentType.includes("text/event-stream")) {
    // Node fetch may transparently decode upstream bodies; never forward stale
    // length/encoding metadata or allow intermediaries to transform the stream.
    outHeaders.delete("content-length");
    outHeaders.delete("content-encoding");
    outHeaders.delete("connection");
    outHeaders.set("cache-control", "no-cache, no-transform");
    return new Response(upstreamRes.body, {
      status: upstreamRes.status,
      statusText: upstreamRes.statusText,
      headers: outHeaders,
    });
  }

  const buf = await upstreamRes.arrayBuffer();
  return new Response(buf, {
    status: upstreamRes.status,
    statusText: upstreamRes.statusText,
    headers: outHeaders,
  });
}

export async function GET(req: Request, ctx: { params: { path: string[] } }) {
  return proxy(req, ctx.params.path || []);
}
export async function POST(req: Request, ctx: { params: { path: string[] } }) {
  return proxy(req, ctx.params.path || []);
}
export async function PUT(req: Request, ctx: { params: { path: string[] } }) {
  return proxy(req, ctx.params.path || []);
}
export async function PATCH(req: Request, ctx: { params: { path: string[] } }) {
  return proxy(req, ctx.params.path || []);
}
export async function DELETE(req: Request, ctx: { params: { path: string[] } }) {
  return proxy(req, ctx.params.path || []);
}
export async function OPTIONS(req: Request, ctx: { params: { path: string[] } }) {
  return proxy(req, ctx.params.path || []);
}
