const API_ORIGIN = process.env.API_ORIGIN ?? "http://localhost:8080";

async function proxy(request: Request, context: { params: Promise<{ path: string[] }> }) {
  const { path } = await context.params;
  const incoming = new URL(request.url);
  const target = new URL(`/${path.join("/")}${incoming.search}`, API_ORIGIN);
  const headers = new Headers(request.headers);
  headers.delete("host"); headers.delete("content-length");
  const response = await fetch(target, { method: request.method, headers, body: request.method === "GET" || request.method === "HEAD" ? undefined : request.body, duplex: "half", redirect: "manual" } as RequestInit & { duplex: "half" });
  const outgoing = new Headers(response.headers);
  outgoing.delete("content-length"); outgoing.delete("content-encoding");
  return new Response(response.body, { status: response.status, headers: outgoing });
}

export const GET = proxy;
export const POST = proxy;
export const PUT = proxy;
export const PATCH = proxy;
export const DELETE = proxy;
