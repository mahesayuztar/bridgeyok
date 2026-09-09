import { requireAccount } from "../account-server";
import LobbyClient from "./lobby-client";

export default async function LobbyPage({ searchParams }: { searchParams: Promise<{ invite?: string | string[] }> }) {
  const inviteParameter = (await searchParams).invite;
  const initialInviteCode = typeof inviteParameter === "string" ? inviteParameter : "";

  await requireAccount(initialInviteCode ? `/lobby?invite=${encodeURIComponent(initialInviteCode)}` : "/lobby");
  return <LobbyClient initialInviteCode={initialInviteCode} />;
}
