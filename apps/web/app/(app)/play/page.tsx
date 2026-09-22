import Link from "next/link";
import { requireAccount } from "../../account-server";

export default async function PlayPage() {
  const account = await requireAccount("/play");
  return <main className="account-main"><h1>Halo, {account.profile.displayName}.</h1><p>Pilih cara bermain dan ajak teman ke meja.</p><div className="capability-grid">
    <Link className="capability-card capability-primary" href="/lobby"><h2>Casual Game</h2><p>Buat meja atau masuk dengan kode undangan.</p><strong>Mulai bermain →</strong></Link>
    <Link className="capability-card" href="/match"><h2>Team Match</h2><p>Dua tim, dua room, board yang sama.</p><strong>Atur match →</strong></Link>
  </div></main>;
}
