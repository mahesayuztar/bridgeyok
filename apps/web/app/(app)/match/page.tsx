import type { Metadata } from "next";
import { requireAccount } from "../../account-server";
import { MatchLobby } from "../../team-match";

export const metadata: Metadata = { title: "Team Match · BridgeYok" };

export default async function MatchPage() {
  const account = await requireAccount("/match");
  return <main className="account-main match-main"><p className="eyebrow">Play · Team Match</p><h1>Dua room. Satu match.</h1><p>Delapan pemain, board identik, hasil dibandingkan dalam IMP.</p><MatchLobby profile={account.profile} /></main>;
}
