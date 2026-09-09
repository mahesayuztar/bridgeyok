import type { ReactNode } from "react";
import { currentAccount } from "../account-server";
import { AppNavigation, AppNotices } from "../app-navigation";
import { AccountPresence } from "../account-presence";

export default async function AppLayout({ children }: { children: ReactNode }) {
  const account = await currentAccount();
  if (!account) return children;
  return <AppNotices><div className="account-shell"><AppNavigation profile={account.profile} /><div className="account-content"><AccountPresence />{children}</div></div></AppNotices>;
}
