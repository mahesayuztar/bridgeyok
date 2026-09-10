"use client";

import { useEffect, useState } from "react";
import { accountRequest, type Profile } from "./account-types";
import { ChatPanel } from "./chat-panel";
import { ProfileAvatar } from "./profile-avatar";

export function FollowAction({ profile, viewerId, onChanged }: { profile: Profile; viewerId: string; onChanged: () => void }) {
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");
  async function change() {
    setPending(true); setError("");
    try { await accountRequest(`/users/${profile.id}/follow`, { method: profile.following ? "DELETE" : "PUT" }); onChanged(); }
    catch (error) { setError(error instanceof Error ? error.message : "Koneksi terputus."); }
    finally { setPending(false); }
  }
  if (profile.id === viewerId) return <span className="social-state">Kamu</span>;
  return <div className="follow-action"><button type="button" disabled={pending} onClick={() => void change()} aria-label={profile.following ? `Unfollow ${profile.displayName}` : `Follow ${profile.displayName}`}>{pending ? "Menyimpan…" : profile.friends ? "Friends · Unfollow" : profile.following ? "Following · Unfollow" : "Follow"}</button>{error ? <p role="alert" className="form-error">{error}</p> : null}</div>;
}

export function SocialUsers({ viewerId, table, initialQuery = "" }: { initialQuery?: string; viewerId: string; table?: { id: string; inviteCode: string; disabled: boolean } }) {
  const [chatProfile, setChatProfile] = useState<Profile | null>(null);
  const [query, setQuery] = useState(initialQuery);
  const [friendsOnly, setFriendsOnly] = useState(!table && !initialQuery);
  const [state, setState] = useState<{ profiles: Profile[]; loading: boolean; error: string }>({ profiles: [], loading: true, error: "" });
  const [revision, setRevision] = useState(0);
  const [pendingId, setPendingId] = useState<string | null>(null);
  const [notice, setNotice] = useState("");
  useEffect(() => {
    const controller = new AbortController();
    let running = false;
    async function load() {
      if (running) return;
      running = true;
      try {
        const profiles = await accountRequest<Profile[]>(`/users?q=${encodeURIComponent(query.trim())}&friends=${friendsOnly}`, { signal: controller.signal });
        if (!controller.signal.aborted) setState({ profiles, loading: false, error: "" });
      } catch (error) { if (!controller.signal.aborted) setState({ profiles: [], loading: false, error: error instanceof Error ? error.message : "Koneksi terputus." }); }
      finally { running = false; }
    }
    const timeout = setTimeout(() => void load(), 250);
    const interval = setInterval(() => void load(), 15000);
    return () => { controller.abort(); clearTimeout(timeout); clearInterval(interval); };
  }, [query, friendsOnly, revision]);
  function reload() { setState(previous => ({ ...previous, loading: true })); setRevision(value => value + 1); }
  async function invite(profile: Profile) {
    if (!table) return;
    setPendingId(profile.id); setNotice("");
    try { await accountRequest(`/tables/${table.id}/invites`, { method: "POST", body: JSON.stringify({ userId: profile.id, inviteCode: table.inviteCode }) }); setNotice(`Undangan terkirim ke ${profile.displayName}.`); }
    catch (error) { setNotice(error instanceof Error ? error.message : "Undangan gagal. Coba lagi."); reload(); }
    finally { setPendingId(null); }
  }
  return <section className="social-users" aria-label={table ? "Undang pemain" : "Friends dan pencarian"}>
    <label className="search-label">Cari pemain<input type="search" value={query} maxLength={96} placeholder="Username atau nama" onChange={event => { setQuery(event.target.value); setState(previous => ({ ...previous, loading: true })); }} /></label>
    <label className="friends-filter"><input type="checkbox" checked={friendsOnly} onChange={event => { setFriendsOnly(event.target.checked); setState(previous => ({ ...previous, loading: true })); }} />Friends saja</label>
    {state.loading ? <p role="status">Memuat pemain…</p> : state.error ? <div role="alert"><p className="form-error">{state.error}</p><button type="button" onClick={reload}>Coba lagi</button></div> : state.profiles.length === 0 ? <p role="status">{query ? "Tidak ada pemain yang cocok." : friendsOnly ? "Belum ada Friends. Cari pemain dan saling follow untuk berteman." : "Belum ada pemain."}</p> : <ul className="social-list">{state.profiles.map(profile => <li key={profile.id}>
      <ProfileAvatar avatar={profile.avatar} online={profile.online} /><div className="social-user-name"><strong>{profile.displayName}</strong><span>@{profile.username} · {profile.online ? "Online" : "Offline"}</span></div>
      {profile.friends ? <button type="button" onClick={() => setChatProfile(profile)}>Chat</button> : null}
      <FollowAction profile={profile} viewerId={viewerId} onChanged={reload} />
      {table ? <div className="invite-action"><button type="button" disabled={!profile.online || profile.id === viewerId || pendingId !== null || table.disabled} onClick={() => void invite(profile)}>{pendingId === profile.id ? "Mengirim…" : "Invite"}</button>{!profile.online ? <span>Pemain offline</span> : profile.id === viewerId ? <span>Ini akunmu</span> : table.disabled ? <span>Meja tidak tersedia</span> : null}</div> : null}
    </li>)}</ul>}
    {notice ? <p role="status">{notice}</p> : null}
 {chatProfile ? <ChatPanel key={chatProfile.id} target={{ scope: "private", id: chatProfile.id }} profile={state.profiles.find(profile => profile.id === chatProfile.id) ?? chatProfile} onClose={() => setChatProfile(null)} /> : null}
  </section>;
}
