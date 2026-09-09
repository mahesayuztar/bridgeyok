import Link from "next/link";
import { redirect } from "next/navigation";
import AuthForm from "../auth-form";
import { currentAccount } from "../account-server";
import { safeReturnPath } from "../account-types";

export default async function AuthPage({ searchParams }: { searchParams: Promise<{ next?: string }> }) {
  if (await currentAccount()) redirect("/play");
  const next = safeReturnPath((await searchParams).next);
  return <main className="auth-page"><Link className="wordmark" href="/">BridgeYok</Link><section><p className="eyebrow">Satu akun, meja bersama</p><h1>Mulai bermain.</h1><AuthForm signup={true} next={next} /></section></main>;
}
