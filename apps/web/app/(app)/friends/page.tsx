import { requireAccount } from "../../account-server";
import { SocialUsers } from "../../social-users";

export default async function FriendsPage({ searchParams }: { searchParams: Promise<{ user?: string }> }) {
  const query = (await searchParams).user ?? "";
  const account = await requireAccount("/friends");
  return <main className="account-main"><h1>Friends</h1><SocialUsers initialQuery={query} viewerId={account.profile.id} /></main>;
}
