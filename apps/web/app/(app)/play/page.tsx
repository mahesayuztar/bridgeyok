import Link from "next/link";
import { requireAccount } from "../../account-server";
import { ComingSoon } from "../../app-navigation";

export default async function PlayPage() {
  const account = await requireAccount("/play");
  return <main className="account-main"><p className="eyebrow">Play</p><h1>Halo, {account.profile.displayName}.</h1><p>Pilih permainanmu. Ajak teman ke meja.</p><div className="capability-grid">
    <Link className="capability-card capability-primary" href="/lobby"><span aria-hidden="true">♠</span><h2>Casual Game</h2><p>Buat meja atau masuk dengan kode undangan.</p><strong>Mulai bermain →</strong></Link>
    <Link className="capability-card" href="/match"><span aria-hidden="true">♣</span><h2>Team Match</h2><p>Dua tim, dua room, board yang sama.</p><strong>Atur match →</strong></Link>
    {[ { title: "VS Robot", icon: "♢" }, { title: "Teacher Table", icon: "♡" }].map(item => <ComingSoon key={item.title} className="capability-card"><span aria-hidden="true">{item.icon}</span><h2>{item.title}</h2><p>Coming soon</p></ComingSoon>)}
  </div></main>;
}
