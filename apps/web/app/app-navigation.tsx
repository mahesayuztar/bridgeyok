"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useState } from "react";
import { ProfileAvatar } from "./profile-avatar";
import type { Profile } from "./account-types";

const navigation = [
  { label: "Friends", icon: "♧", href: "/friends" },
  { label: "History", icon: "◷", href: null },
  { label: "Play", icon: "♠", href: "/play" },
  { label: "Deals", icon: "▤", href: null },
  { label: "Settings", icon: "⚙", href: "/settings" }
];

export function AppNavigation({ profile }: { profile: Profile }) {
  const path = usePathname();
  const [toast, setToast] = useState(0);
  return <>
    <aside className="account-navigation"><Link href="/play" className="wordmark">BridgeYok</Link><nav aria-label="Navigasi utama">{navigation.map(item => item.href ?
      <Link key={item.label} href={item.href} className={item.label === "Play" ? "nav-play" : ""} aria-current={path === item.href || (item.href === "/play" && path === "/lobby") ? "page" : undefined}><span aria-hidden="true">{item.icon}</span>{item.label}</Link> :
      <button key={item.label} type="button" onClick={() => setToast(value => value + 1)}><span aria-hidden="true">{item.icon}</span>{item.label}</button>)}</nav><Link href="/settings" className="nav-profile"><ProfileAvatar avatar={profile.avatar} compact /><span>{profile.displayName}</span></Link></aside>
    {toast > 0 ? <div className="account-toast" role="status" key={toast}>Coming soon<button type="button" aria-label="Tutup pesan" onClick={() => setToast(0)}>×</button></div> : null}
  </>;
}

export function ComingSoon({ children, className }: { children: React.ReactNode; className?: string }) {
  const [toast, setToast] = useState(0);
  return <><button type="button" className={className} onClick={() => setToast(value => value + 1)}>{children}</button>{toast > 0 ? <div className="account-toast" role="status" key={toast}>Coming soon<button type="button" aria-label="Tutup pesan" onClick={() => setToast(0)}>×</button></div> : null}</>;
}
