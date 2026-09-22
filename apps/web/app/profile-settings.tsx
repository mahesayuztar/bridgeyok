"use client";

import { useActionState, useState } from "react";
import { useRouter } from "next/navigation";
import { accountRequest, avatars, type Avatar, type Profile } from "./account-types";
import { ProfileAvatar } from "./profile-avatar";
import { useTableSession } from "./use-table-session";
import { useTurnSoundPreference } from "./use-turn-audio";
import { useWorkspacePresentation } from "./workspace-state";

export function ProfileSettings({ profile }: { profile: Profile }) {
  const router = useRouter();
  const session = useTableSession();
  const sound = useTurnSoundPreference();
  const [logoutError, setLogoutError] = useState(false);
  const workspace = useWorkspacePresentation();
  const savedDraft = workspace?.settingsDraft?.profileId === profile.id ? workspace.settingsDraft : null;
  const [localName, setLocalName] = useState(savedDraft?.name ?? profile.displayName);
  const [localAvatar, setLocalAvatar] = useState<Avatar>(savedDraft?.avatar ?? profile.avatar);
  const name = savedDraft?.name ?? localName;
  const avatar = savedDraft?.avatar ?? localAvatar;
  const setName = (value: string) => {
    setLocalName(value);
    workspace?.setSettingsDraft({ profileId: profile.id, name: value, avatar });
  };
  const setAvatar = (value: Avatar) => {
    setLocalAvatar(value);
    workspace?.setSettingsDraft({ profileId: profile.id, name, avatar: value });
  };
  const [state, save, pending] = useActionState(async (_previous: { error: boolean; message: string }, form: FormData) => {
    try {
      await accountRequest<Profile>("/profile", { method: "PUT", body: JSON.stringify({ displayName: form.get("displayName"), avatar: form.get("avatar") }) });
      workspace?.setSettingsDraft(null);
      workspace?.setSettingsStatus({ error: false, message: "Profile tersimpan." });
      router.refresh();
      return { error: false, message: "Profile tersimpan." };
    } catch (error) {
      const result = { error: true, message: error instanceof Error ? error.message : "Koneksi terputus." };
      workspace?.setSettingsStatus(result);
      return result;
    }
  }, workspace?.settingsStatus ?? { error: false, message: "" });
  async function logout() {
    setLogoutError(false);
    if (await session.logout()) window.location.replace("/login");
    else setLogoutError(true);
  }
  return <><form className="account-form profile-form" action={save} aria-busy={pending}>
    <div className="profile-preview"><ProfileAvatar avatar={avatar} online={profile.online} /><div><strong>{name || profile.username}</strong><p>@{profile.username}</p></div></div>
    <label>Nama di meja<input name="displayName" value={name} onChange={event => setName(event.target.value)} minLength={2} maxLength={24} required disabled={pending} /></label>
    <fieldset disabled={pending}><legend>Avatar BridgeYok</legend><div className="avatar-options">{avatars.map(option => <label key={option}><input type="radio" name="avatar" value={option} checked={avatar === option} onChange={() => setAvatar(option)} /><ProfileAvatar avatar={option} /><span>{option}</span></label>)}</div></fieldset>
    <button className="primary-button" disabled={pending}>{pending ? "Menyimpan…" : "Simpan profile"}</button>
    {state.message ? <p className={state.error ? "form-error" : "form-success"} role={state.error ? "alert" : "status"}>{state.message}</p> : null}
  </form>
  <section className="settings-section" aria-labelledby="audio-settings-title">
    <h2 id="audio-settings-title">Audio</h2>
    <label className="settings-toggle"><input type="checkbox" checked={!sound.muted} onChange={(event) => sound.setMuted(!event.target.checked)} />Bunyikan penanda giliran</label>
  </section>
  <button className="logout-button" type="button" disabled={session.busy} onClick={() => void logout()}>{session.busy ? "Menutup sesi…" : "Logout"}</button>
  {logoutError ? <p className="form-error" role="alert">Logout gagal. Sesi dan meja tetap aktif; coba lagi.</p> : null}</>;
}
