import type { components } from "@bridgeyok/contracts/openapi";
export const avatars = ["spade", "heart", "diamond", "club", "owl", "fox"] as const;
export type Avatar = typeof avatars[number];
export type Profile = components["schemas"]["AccountProfile"];
export type AccountLogin = components["schemas"]["AccountLogin"];
export type Invitation = components["schemas"]["PlayerInvitation"];

export function safeReturnPath(value: string | null | undefined) {
  if (!value || !/^\/(play|lobby|friends|history|settings|match(?:\/[^/?#]+)?|table\/[^/?#]+)(\?|$)/.test(value) || value.includes("\\") || /[\r\n]/.test(value)) return "/play";
  return value;
}

export class AccountRequestError extends Error {
  status: number;

  constructor(message: string, status: number) {
    super(message);
    this.status = status;
  }
}

export async function accountRequest<T>(path = "", init: RequestInit = {}): Promise<T> {
  const response = await fetch(`/api/account${path}`, { ...init, headers: { "Content-Type": "application/json", ...init.headers }, signal: init.signal ?? AbortSignal.timeout(10000), cache: "no-store" });
  if (!response.ok) {
    const problem = await response.json().catch(() => ({})) as { code?: string };
    const messages: Record<string, string> = {
      INVALID_CREDENTIAL: "Username atau kata sandi tidak cocok. Silakan login kembali.",
      USERNAME_TAKEN: "Username sudah digunakan.",
      INVALID_ACCOUNT_INPUT: "Periksa isian: username 3–24 huruf/angka/_, nama 2–24 karakter, kata sandi 10–128 karakter.",
      USER_OFFLINE: "Pemain sedang offline atau meja tidak tersedia.",
      MATCH_NOT_FOUND: "Match tidak ditemukan atau kamu bukan pesertanya.",
      MATCH_FORBIDDEN: "Hanya pemilik yang dapat memulai match.",
      INVALID_MATCH_INPUT: "Pilih delapan pemain berbeda dan 1–32 board.",
      MATCH_CAPACITY: "Salah satu pemain sudah memiliki empat match yang belum selesai.",
      STATE_CHANGED: "Status telah berubah. Periksa pembaruan lalu coba lagi.",
      INVALID_HISTORY_LABEL: "Label belajar harus berisi 1–80 karakter.",
      HISTORY_BOARD_NOT_FOUND: "Board history tidak ditemukan atau tidak bisa kamu ubah.",
      RATE_LIMITED: "Terlalu banyak percobaan. Coba lagi sebentar."
    };
    throw new AccountRequestError(messages[problem.code ?? ""] ?? "Layanan belum terhubung. Coba lagi.", response.status);
  }
  return response.status === 204 ? undefined as T : await response.json() as T;
}
