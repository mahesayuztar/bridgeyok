"use client";

import { useActionState, useState } from "react";
import { useRouter } from "next/navigation";
import { accountRequest, avatars, type Avatar, type Profile } from "./account-types";
import { ProfileAvatar } from "./profile-avatar";
import { useTableSession } from "./use-table-session";

export function ProfileSettings({ profile }: { profile: Profile }) {
  const router = useRouter();
  const session = useTableSession();
  const [name, setName] = useState(profile.displayName);
  const [avatar, setAvatar] = useState<Avatar>(profile.avatar);
  const [state, save, pending] = useActionState(async (_previous: { error: boolean; message: string }, form: FormData) => {
    try {
      await accountRequest<Profile>("/profile", { method: "PUT", body: JSON.stringify({ displayName: form.get("displayName"), avatar: form.get("avatar") }) });
      router.refresh();
      return { error: false, message: "Profile tersimpan." };
    } catch (error) { return { error: true, message: error instanceof Error ? error.message : "Koneksi terputus." }; }
  }, { error: false, message: "" });
  async function logout() {
    await session.logout();
    window.location.replace("/login");
  }
  return <><form className="account-form profile-form" action={save} aria-busy={pending}>
    <div className="profile-preview"><ProfileAvatar avatar={avatar} online={profile.online} /><div><strong>{name || profile.username}</strong><p>@{profile.username}</p></div></div>
    <label>Nama di meja<input name="displayName" value={name} onChange={event => setName(event.target.value)} minLength={2} maxLength={24} required disabled={pending} /></label>
    <fieldset disabled={pending}><legend>Avatar BridgeYok</legend><div className="avatar-options">{avatars.map(option => <label key={option}><input type="radio" name="avatar" value={option} checked={avatar === option} onChange={() => setAvatar(option)} /><ProfileAvatar avatar={option} /><span>{option}</span></label>)}</div></fieldset>
    <button className="primary-button" disabled={pending}>{pending ? "Menyimpan…" : "Simpan profile"}</button>
    {state.message ? <p className={state.error ? "form-error" : "form-success"} role={state.error ? "alert" : "status"}>{state.message}</p> : null}
  </form><button className="logout-button" type="button" onClick={() => void logout()}>Logout</button></>;
}
