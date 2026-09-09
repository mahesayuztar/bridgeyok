"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { createContext, useContext, useState, type ReactNode } from "react";
import { ProfileAvatar } from "./profile-avatar";
import type { Profile } from "./account-types";

const navigation = [
  { label: "Friends", icon: "♧", href: "/friends" },
  { label: "History", icon: "◷", href: null },
  { label: "Play", icon: "♠", href: "/play" },
  { label: "Deals", icon: "▤", href: null },
  { label: "Settings", icon: "⚙", href: "/settings" }
];

const ComingSoonContext = createContext<() => void>(() => {});

export function AppNotices({ children }: { children: ReactNode }) {
  const [toast, setToast] = useState(0);
  return (
    <ComingSoonContext value={() => setToast(value => value + 1)}>
      {children}
      {toast > 0 ? (
        <div className="account-toast" role="status" key={toast}>
          Coming soon
          <button type="button" aria-label="Tutup pesan" onClick={() => setToast(0)}>×</button>
        </div>
      ) : null}
    </ComingSoonContext>
  );
}

export function AppNavigation({ profile }: { profile: Profile }) {
  const path = usePathname();
  const comingSoon = useContext(ComingSoonContext);
  return (
    <aside className="account-navigation">
      <Link href="/play" className="wordmark">BridgeYok</Link>
      <nav aria-label="Navigasi utama">
        {navigation.map(item => item.href ? (
          <Link key={item.label} href={item.href} className={item.label === "Play" ? "nav-play" : ""}
            aria-current={path === item.href || (item.href === "/play" && path === "/lobby") ? "page" : undefined}>
            <span aria-hidden="true">{item.icon}</span>{item.label}
          </Link>
        ) : (
          <button key={item.label} type="button" onClick={comingSoon}>
            <span aria-hidden="true">{item.icon}</span>{item.label}
          </button>
        ))}
      </nav>
      <Link href="/settings" className="nav-profile">
        <ProfileAvatar avatar={profile.avatar} compact /><span>{profile.displayName}</span>
      </Link>
    </aside>
  );
}

export function ComingSoon({ children, className }: { children: ReactNode; className?: string }) {
  const comingSoon = useContext(ComingSoonContext);
  return <button type="button" className={className} onClick={comingSoon}>{children}</button>;
}
