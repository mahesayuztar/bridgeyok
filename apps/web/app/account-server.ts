import { cookies } from "next/headers";
import { cache } from "react";
import { redirect } from "next/navigation";
import type { AccountLogin } from "./account-types";

export const ACCOUNT_COOKIE = "bridgeyok_account";
export const ACCOUNT_API_URL = process.env.API_BASE_URL ?? process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://localhost:8080";

export const currentAccount = cache(async () => {
  const token = (await cookies()).get(ACCOUNT_COOKIE)?.value;
  if (!token) return null;
  const response = await fetch(`${ACCOUNT_API_URL}/v1/account`, { headers: { Authorization: `Bearer ${token}` }, cache: "no-store", signal: AbortSignal.timeout(8000) });
  if (response.status === 401) return null;
  if (!response.ok) throw new Error("Layanan akun belum terhubung. Silakan coba lagi.");
  return await response.json() as AccountLogin;
});

export async function requireAccount(returnPath = "/play") {
  const account = await currentAccount();
  if (!account) redirect(`/login?next=${encodeURIComponent(returnPath)}`);
  return account;
}
