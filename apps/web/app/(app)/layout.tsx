import type { ReactNode } from "react";
import { currentAccount } from "../account-server";
import { AppShell } from "../app-shell";

export default async function AppLayout({ children }: { children: ReactNode }) {
  const account = await currentAccount();
  if (!account) return children;
  return <AppShell profile={account.profile}>{children}</AppShell>;
}
