"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import type { components } from "@bridgeyok/contracts/openapi";
import { accountRequest } from "./account-types";
import { boardResultLabel } from "./table-state";
import { compactContractLabel } from "./table/gameplay-presentation";
import { useTableSession } from "./use-table-session";

type MatchView = components["schemas"]["MatchView"];

const matchStatusLabels = {
  WAITING: "Menunggu pemain siap",
  ACTIVE: "Sedang dimainkan",
  COMPLETE: "Selesai",
  CANCELLED: "Dibatalkan",
};

export function HistoryWorkspace() {
  const session = useTableSession();
  const table = session.projectedTable;
  const [matches, setMatches] = useState<MatchView[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [revision, setRevision] = useState(0);

  useEffect(() => {
    const controller = new AbortController();
    async function load() {
      try {
        const result = await accountRequest<MatchView[]>("/matches", { signal: controller.signal });
        if (!controller.signal.aborted) {
          setMatches(result);
          setError("");
          setLoading(false);
        }
      } catch (failure) {
        if (!controller.signal.aborted) {
          setError(failure instanceof Error ? failure.message : "Koneksi terputus.");
          setLoading(false);
        }
      }
    }
    void load();
    return () => controller.abort();
  }, [revision]);

  return (
    <div className="history-workspace">
      {table === null ? null : (
        <section className="history-section" aria-labelledby="active-table-history-title">
          <div className="history-section-heading">
            <div>
              <h2 id="active-table-history-title">Meja aktif</h2>
              <p>Board dari sesi meja yang sedang kamu ikuti.</p>
            </div>
            <Link href={`/table/${table.tableId}`}>Buka meja</Link>
          </div>
          {table.scoreSheet.length === 0 ? <p>Belum ada hasil board di meja ini.</p> : (
            <ol className="history-board-list">
              {table.scoreSheet.map((entry) => (
                <li key={entry.boardId}>
                  <strong>Board {entry.boardNumber}</strong>
                  <span>{entry.result.passedOut ? "Pass out" : `${compactContractLabel(entry.result.contract)} · ${boardResultLabel(entry.result)}`}</span>
                  <span>NS {entry.result.scoreNS > 0 ? "+" : ""}{entry.result.scoreNS}</span>
                </li>
              ))}
            </ol>
          )}
        </section>
      )}
      <section className="history-section" aria-labelledby="match-history-title">
        <div className="history-section-heading">
          <div>
            <h2 id="match-history-title">Team Match</h2>
            <p>Match yang dapat kamu akses. Daftar ini dibatasi hingga 20 entri.</p>
          </div>
        </div>
        {loading ? <p role="status">Memuat match…</p> : null}
        {error ? <div role="alert"><p className="form-error">{error}</p><button type="button" onClick={() => { setLoading(true); setRevision((value) => value + 1); }}>Coba lagi</button></div> : null}
        {!loading && !error && matches.length === 0 ? <p>Belum ada Team Match.</p> : null}
        <ul className="history-match-list">
          {matches.map((match) => (
            <li key={match.id}>
              <Link href={`/match/${match.id}`}>
                <strong>Tim {match.team} · {match.room} · {match.seat}</strong>
                <span>{matchStatusLabels[match.status]} · {match.boardCount} board</span>
                {match.status === "COMPLETE" ? <span>Tim A {match.teamAIMP} IMP</span> : null}
              </Link>
            </li>
          ))}
        </ul>
      </section>
    </div>
  );
}
