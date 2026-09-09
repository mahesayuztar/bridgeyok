import { requireAccount } from "../../account-server";
import { SocialUsers } from "../../social-users";

export default async function FriendsPage() {
  const account = await requireAccount("/friends");
  return <main className="account-main"><p className="eyebrow">Main bersama</p><h1>Friends</h1><p>Saling follow untuk menjadi Friends.</p><SocialUsers viewerId={account.profile.id} /></main>;
}
