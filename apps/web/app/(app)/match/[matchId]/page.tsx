import type { Metadata } from "next";
import Link from "next/link";
import { requireAccount } from "../../../account-server";
import { MatchProgress } from "../../../team-match";

export const metadata: Metadata = { title: "Match · BridgeYok" };

export default async function MatchPage({ params }: { params: Promise<{ matchId: string }> }) {
  const { matchId } = await params;
  await requireAccount(`/match/${matchId}`);
  return <main className="account-main match-main"><Link href="/match">← Team Match</Link><h1>Match</h1><MatchProgress key={matchId} matchId={matchId} /></main>;
}
