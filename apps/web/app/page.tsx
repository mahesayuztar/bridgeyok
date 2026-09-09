import Link from "next/link";
import { redirect } from "next/navigation";
import { currentAccount } from "./account-server";
import { WelcomeCards } from "./welcome-cards";

export default async function HomePage({ searchParams }: { searchParams: Promise<{ invite?: string }> }) {
  const invite = (await searchParams).invite;
  const next = invite ? `/lobby?invite=${encodeURIComponent(invite)}` : "/play";
  if (await currentAccount()) redirect(next);
  return <div className="welcome-page">
    <header><Link className="wordmark" href="/">BridgeYok</Link><nav aria-label="Akun"><Link href={`/login?next=${encodeURIComponent(next)}`}>Login</Link><Link href={`/signup?next=${encodeURIComponent(next)}`}>Sign Up</Link></nav></header>
    <main className="welcome-hero">
      <div><p className="eyebrow">Bridge, bareng.</p><h1>Empat kursi.<br />Satu meja.</h1><p className="welcome-summary">Buat meja bridge dan bermain bersama teman, dari mana saja.</p><Link className="primary-button link-button" href={`/signup?next=${encodeURIComponent(next)}`}>Get Started <span aria-hidden="true">→</span></Link></div>
      <WelcomeCards />
    </main>
    <footer>BridgeYok · Main bridge bareng</footer>
  </div>;
}
