import { requireAccount } from "../../account-server";
import { SocialUsers } from "../../social-users";

export default async function FriendsPage({ searchParams }: { searchParams: Promise<{ user?: string }> }) {
  const query = (await searchParams).user ?? "";
  const account = await requireAccount("/friends");
  return <main className="account-main"><p className="eyebrow">Main bersama</p><h1>Friends</h1><p>Saling follow untuk menjadi Friends.</p><SocialUsers initialQuery={query} viewerId={account.profile.id} /></main>;
}
