import LobbyClient from "./lobby-client";

export default async function LobbyPage({ searchParams }: { searchParams: Promise<{ invite?: string | string[] }> }) {
  const inviteParameter = (await searchParams).invite;
  const initialInviteCode = typeof inviteParameter === "string" ? inviteParameter : "";

  return <LobbyClient initialInviteCode={initialInviteCode} />;
}
