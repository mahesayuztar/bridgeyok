"use client";

import Link from "next/link";
import { useActionState } from "react";
import { accountRequest, safeReturnPath } from "./account-types";

export default function AuthForm({ signup, next }: { signup: boolean; next: string }) {
  const [error, submit, pending] = useActionState(async (_previous: string, form: FormData) => {
    try {
      await accountRequest(signup ? "/signup" : "/login", { method: "POST", body: JSON.stringify({ username: form.get("username"), password: form.get("password"), ...(signup ? { displayName: form.get("displayName"), avatar: "spade" } : {}) }) });
      try {
      localStorage.removeItem("bridgeyok.identity.v1");
      localStorage.removeItem("bridgeyok.table.v1");
      sessionStorage.removeItem("bridgeyok.access.v1");
      } catch {}
      window.location.assign(safeReturnPath(next));
      return "";
    } catch (error) { return error instanceof Error ? error.message : "Koneksi terputus. Coba lagi."; }
  }, "");
  return <form action={submit} className="account-form" aria-busy={pending}>
    <label>Username<input name="username" autoComplete="username" pattern="[A-Za-z0-9_]{3,24}" minLength={3} maxLength={24} required disabled={pending} autoCapitalize="none" spellCheck={false} /></label>
    {signup ? <label>Nama di meja<input name="displayName" autoComplete="nickname" minLength={2} maxLength={24} required disabled={pending} /></label> : null}
    <label>Kata sandi<input name="password" type="password" autoComplete={signup ? "new-password" : "current-password"} minLength={signup ? 10 : undefined} maxLength={128} required disabled={pending} /></label>
    {signup ? <p className="form-hint">Username 3–24 huruf, angka atau _. Kata sandi minimal 10 karakter.</p> : null}
    {error ? <p className="form-error" role="alert">{error}</p> : null}
    <button className="primary-button" disabled={pending}>{pending ? "Menghubungkan…" : signup ? "Sign Up" : "Login"}</button>
    <Link href={`/${signup ? "login" : "signup"}?next=${encodeURIComponent(next)}`}>{signup ? "Sudah punya akun? Login" : "Belum punya akun? Sign Up"}</Link>
  </form>;
}
