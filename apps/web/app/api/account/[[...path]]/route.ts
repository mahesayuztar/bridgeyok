import { cookies } from "next/headers";
import { NextResponse } from "next/server";
import { ACCOUNT_API_URL, ACCOUNT_COOKIE } from "../../../account-server";
import type { AccountLogin } from "../../../account-types";

async function handle(request: Request, context: { params: Promise<{ path?: string[] }> }) {
  const path = (await context.params).path?.join("/") ?? "";
  const allowed = /^(signup|login|logout|heartbeat|profile|users|invitations|chat|users\/[a-f0-9-]+\/follow|tables\/[a-f0-9-]+\/(participants|invites))?$/;
  if (!allowed.test(path)) return new Response(null, { status: 404 });
  if (request.method !== "GET" && request.headers.get("origin") !== new URL(request.url).origin) return new Response(null, { status: 403 });
  const cookieStore = await cookies();
  const token = cookieStore.get(ACCOUNT_COOKIE)?.value;
  const isLogin = path === "signup" || path === "login";
  if (!isLogin && !token) return NextResponse.json({ code: "INVALID_CREDENTIAL" }, { status: 401 });
  try {
    const body = request.method === "GET" ? undefined : await request.text();
    if (body && body.length > 8192) return new Response(null, { status: 413 });
    const response = await fetch(`${ACCOUNT_API_URL}/v1/account${path ? `/${path}` : ""}${new URL(request.url).search}`, {
      method: request.method,
      headers: { "Content-Type": "application/json", ...(token ? { Authorization: `Bearer ${token}` } : {}) },
      ...(body === undefined ? {} : { body }), cache: "no-store", signal: AbortSignal.timeout(8000)
    });
    if (!response.ok) return new Response(await response.text(), { status: response.status, headers: { "Content-Type": "application/json", "Cache-Control": "no-store" } });
    if (path === "logout") cookieStore.delete(ACCOUNT_COOKIE);
    if (response.status === 204) return new Response(null, { status: 204 });
    if (isLogin || path === "") {
      const account = await response.json() as AccountLogin;
      if (isLogin) cookieStore.set(ACCOUNT_COOKIE, account.token, { httpOnly: true, secure: new URL(request.url).protocol === "https:", sameSite: "lax", path: "/", expires: new Date(account.expiresAt) });
      return NextResponse.json({ profile: account.profile, credentials: { sessionId: account.sessionId, nickname: account.profile.displayName, accessToken: account.token, accessExpiresAt: account.expiresAt, deviceCredential: "registered" } }, { headers: { "Cache-Control": "private, no-store" } });
    }
    return new Response(await response.text(), { headers: { "Content-Type": "application/json", "Cache-Control": "private, no-store" } });
  } catch {
    return NextResponse.json({ code: "SERVICE_UNAVAILABLE" }, { status: 503 });
  }
}

export const GET = handle;
export const POST = handle;
export const PUT = handle;
export const DELETE = handle;
