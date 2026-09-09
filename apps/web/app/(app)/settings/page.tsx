import { requireAccount } from "../../account-server";
import { ProfileSettings } from "../../profile-settings";

export default async function SettingsPage() {
  const account = await requireAccount("/settings");
  return <main className="account-main"><p className="eyebrow">Settings</p><h1>Profile</h1><ProfileSettings profile={account.profile} /></main>;
}
