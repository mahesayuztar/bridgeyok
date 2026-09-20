"use client";

import { usePathname, useRouter } from "next/navigation";
import { useEffect, useRef, useState, type ReactNode } from "react";
import { AccountPresence } from "./account-presence";
import type { Profile } from "./account-types";
import { AppNavigation, AppNotices } from "./app-navigation";
import BridgeTable from "./bridge-table";
import IssueNotice from "./issue-notice";
import { TableSocialProvider } from "./table/table-social";
import { useTableSession } from "./use-table-session";
import { WorkspacePresentationProvider } from "./workspace-state";

function tableIdFromPath(pathname: string) {
  const match = /^\/table\/([^/]+)$/.exec(pathname);
  return match?.[1] ? decodeURIComponent(match[1]) : null;
}

export function AppShell({ children, profile }: { children: ReactNode; profile: Profile }) {
  const pathname = usePathname();
  const router = useRouter();
  const session = useTableSession();
  const requestedTableId = tableIdFromPath(pathname);
  const activeTableId = session.tableState.activeTableId;
  const persistentTableId = activeTableId ?? requestedTableId;
  const tableConflict = requestedTableId !== null && activeTableId !== null && requestedTableId !== activeTableId;
  const tableVisible = requestedTableId !== null && !tableConflict;
  const previousPathRef = useRef<string | null>(null);
  const currentPathRef = useRef(pathname);
  const scrollPositionsRef = useRef(new Map<string, number>());
  const [switching, setSwitching] = useState(false);

  useEffect(() => {
    const scrollPositions = scrollPositionsRef.current;
    const previousPath = currentPathRef.current;
    if (previousPath !== pathname) {
      previousPathRef.current = previousPath;
      currentPathRef.current = pathname;
    }
    const frame = requestAnimationFrame(() => window.scrollTo({ top: scrollPositions.get(pathname) ?? 0 }));
    return () => {
      cancelAnimationFrame(frame);
      scrollPositions.set(pathname, window.scrollY);
    };
  }, [pathname]);

  function closeWorkspace() {
    const tablePath = activeTableId === null ? null : `/table/${activeTableId}`;
    if (tablePath !== null && previousPathRef.current === tablePath) {
      router.back();
      return;
    }
    router.replace(tablePath ?? "/play");
  }

  async function switchTable() {
    if (requestedTableId === null) return;
    setSwitching(true);
    const switched = await session.switchTable(requestedTableId);
    setSwitching(false);
    if (!switched && session.tableState.activeTableId === null) router.replace("/lobby");
  }

  return (
    <WorkspacePresentationProvider>
      <AppNotices>
        <div className="account-shell">
          <AppNavigation profile={profile} />
          <div className="account-content">
            <AccountPresence />
            {persistentTableId === null ? null : (
              <div
                className="persistent-table-workspace"
                data-visible={tableVisible}
                hidden={!tableVisible}
                inert={!tableVisible}
              >
                <TableSocialProvider key={persistentTableId} tableId={persistentTableId} viewerId={profile.id}>
                  <BridgeTable expectedTableId={persistentTableId} visible={tableVisible} />
                </TableSocialProvider>
              </div>
            )}
            {tableVisible ? null : (
              <div className="secondary-workspace">
                {tableConflict ? (
                  <main className="workspace-switch" aria-labelledby="workspace-switch-title">
                    <h1 id="workspace-switch-title">Pindah meja?</h1>
                    <p>Kamu masih terhubung ke meja yang sedang aktif. Pindah hanya setelah meja lama berhasil ditinggalkan.</p>
                    <div className="workspace-switch-actions">
                      <button type="button" className="primary-button" disabled={switching || session.busy} onClick={() => void switchTable()}>
                        {switching ? "Memindahkan…" : "Tinggalkan dan pindah"}
                      </button>
                      <button type="button" disabled={switching || session.busy} onClick={() => router.replace(`/table/${activeTableId}`)}>
                        Tetap di meja ini
                      </button>
                    </div>
                    {session.tableState.issue === null ? null : (
                      <IssueNotice issue={session.tableState.issue} onDismiss={session.dismissIssue} onAction={() => void switchTable()} />
                    )}
                  </main>
                ) : (
                  <>
                    {pathname === "/play" ? null : (
                      <button type="button" className="workspace-close" onClick={closeWorkspace} aria-label={activeTableId === null ? "Tutup workspace dan kembali ke Play" : "Tutup workspace dan kembali ke meja"}>
                        <span aria-hidden="true">×</span>
                      </button>
                    )}
                    {children}
                  </>
                )}
              </div>
            )}
          </div>
        </div>
      </AppNotices>
    </WorkspacePresentationProvider>
  );
}
