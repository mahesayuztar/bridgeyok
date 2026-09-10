"use client";
import { chatStore } from "./chat-store";

import { useActionState, useState } from "react";
import { useRouter } from "next/navigation";
import { accountRequest, avatars, type Avatar, type Profile } from "./account-types";
import { ProfileAvatar } from "./profile-avatar";

export function ProfileSettings({ profile }: { profile: Profile }) {
  const router = useRouter();
  const [name, setName] = useState(profile.displayName);
  const [avatar, setAvatar] = useState<Avatar>(profile.avatar);
  const [state, save, pending] = useActionState(async (_previous: { error: boolean; message: string }, form: FormData) => {
    try {
      await accountRequest<Profile>("/profile", { method: "PUT", body: JSON.stringify({ displayName: form.get("displayName"), avatar: form.get("avatar") }) });
      router.refresh();
      return { error: false, message: "Profile tersimpan." };
    } catch (error) { return { error: true, message: error instanceof Error ? error.message : "Koneksi terputus." }; }
  }, { error: false, message: "" });
  const [logoutError, setLogoutError] = useState("");
  async function logout() {
    try {
      await accountRequest("/logout", { method: "POST" });
      chatStore.identify("");
      try {
      localStorage.removeItem("bridgeyok.identity.v1");
      sessionStorage.removeItem("bridgeyok.access.v1");
      } catch {}
      router.replace("/login");
      router.refresh();
    } catch { setLogoutError("Belum dapat logout. Periksa koneksi dan coba lagi."); }
  }
  return <><form className="account-form profile-form" action={save} aria-busy={pending}>
    <div className="profile-preview"><ProfileAvatar avatar={avatar} online={profile.online} /><div><strong>{name || profile.username}</strong><p>@{profile.username}</p></div></div>
    <label>Nama di meja<input name="displayName" value={name} onChange={event => setName(event.target.value)} minLength={2} maxLength={24} required disabled={pending} /></label>
    <fieldset disabled={pending}><legend>Avatar BridgeYok</legend><div className="avatar-options">{avatars.map(option => <label key={option}><input type="radio" name="avatar" value={option} checked={avatar === option} onChange={() => setAvatar(option)} /><ProfileAvatar avatar={option} /><span>{option}</span></label>)}</div></fieldset>
    <button className="primary-button" disabled={pending}>{pending ? "Menyimpan…" : "Simpan profile"}</button>
    {state.message ? <p className={state.error ? "form-error" : "form-success"} role={state.error ? "alert" : "status"}>{state.message}</p> : null}
  </form><button className="logout-button" type="button" onClick={() => void logout()}>Logout</button>{logoutError ? <p role="alert">{logoutError}</p> : null}</>;
}
