import { requireAccount } from "../../account-server";
import { ProfileSettings } from "../../profile-settings";

export default async function SettingsPage() {
  const account = await requireAccount("/settings");
  return <main className="account-main"><h1>Settings</h1><section className="settings-section" aria-labelledby="profile-settings-title"><h2 id="profile-settings-title">Profile</h2><ProfileSettings profile={account.profile} /></section></main>;
}
