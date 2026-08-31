"use client";

import { useRouter } from "next/navigation";
import { useEffect, useMemo, useRef, useState } from "react";
import IssueNotice from "./issue-notice";
import {
  oppositeSeat,
  playableHand,
  tableOrientation,
  visualPositionForSeat,
  type Call,
  type Card,
} from "./table-state";
import { AuctionTable, BiddingBox } from "./table/auction-controls";
import { BoardResult } from "./table/board-result";
import { ConsensusControls } from "./table/consensus-controls";
import { CurrentTrick } from "./table/current-trick";
import { callKey } from "./table/gameplay-presentation";
import { BridgeHand } from "./table/playing-card";
import {
  ActiveTableStatusBar,
  WaitingTableStatusBar,
} from "./table/table-status-bar";
import { TableSurface } from "./table/table-surface";
import { WaitingRoom } from "./table/waiting-room";
import { useTableSession } from "./use-table-session";

export default function BridgeTable({
  expectedTableId,
}: {
  expectedTableId: string;
}) {
  const router = useRouter();
  const session = useTableSession();
  const { openTable, sendCommand } = session;
  const attemptedTableIdRef = useRef<string | null>(null);
  const [copied, setCopied] = useState(false);
  const table = session.tableState.table;
  const game = table?.game;
  const hasPendingCommand = Object.keys(session.tableState.pending).length > 0;
  const commandDisabled =
    session.connectionState !== "connected" ||
    hasPendingCommand ||
    session.tableState.controllerState !== "current";
  const gameplayDisabled =
    commandDisabled || table?.actionRequest !== undefined;
  const orientation = useMemo(
    () => tableOrientation(table?.viewerSeat),
    [table?.viewerSeat],
  );
  const legalPlay = table === null ? null : playableHand(table);
  const viewerTurn = game?.turn === table?.viewerSeat;

  useEffect(() => {
    if (session.initializing) return;
    if (session.nickname === null) {
      router.replace("/");
      return;
    }
    if (
      table?.tableId !== expectedTableId &&
      attemptedTableIdRef.current !== expectedTableId
    ) {
      attemptedTableIdRef.current = expectedTableId;
      void openTable(expectedTableId);
    }
  }, [
    expectedTableId,
    openTable,
    router,
    session.initializing,
    session.nickname,
    table?.tableId,
  ]);

  useEffect(() => {
    function handleAuctionKeyboard(event: KeyboardEvent) {
      if (
        event.target instanceof HTMLInputElement ||
        event.target instanceof HTMLSelectElement ||
        event.target instanceof HTMLTextAreaElement ||
        game?.phase !== "AUCTION" ||
        !viewerTurn ||
        gameplayDisabled
      )
        return;
      const shortcutCalls: Record<string, Call> = {
        p: { kind: "PASS" },
        x: { kind: "DOUBLE" },
        r: { kind: "REDOUBLE" },
      };
      const call = shortcutCalls[event.key.toLowerCase()];
      if (
        call !== undefined &&
        game.legalCalls?.some(
          (legalCall) => callKey(legalCall) === callKey(call),
        )
      ) {
        event.preventDefault();
        sendCommand("game.make_call", { call });
      }
    }
    window.addEventListener("keydown", handleAuctionKeyboard);
    return () => window.removeEventListener("keydown", handleAuctionKeyboard);
  }, [game, gameplayDisabled, sendCommand, viewerTurn]);

  const shareUrl = useMemo(() => {
    if (session.inviteCode === null || typeof window === "undefined") return "";
    const url = new URL("/lobby", window.location.origin);
    url.searchParams.set("invite", session.inviteCode);
    return url.toString();
  }, [session.inviteCode]);

  async function copyInvite() {
    if (shareUrl === "") return;
    await navigator.clipboard.writeText(shareUrl);
    setCopied(true);
    window.setTimeout(() => setCopied(false), 2500);
  }

  async function returnToLobby() {
    await session.leaveTable();
    router.replace("/lobby");
  }

  if (
    session.initializing ||
    session.nickname === null ||
    table?.tableId !== expectedTableId
  ) {
    return (
      <main className="table-route-state">
        {session.tableState.issue === null ? (
          <p role="status">Menyiapkan meja…</p>
        ) : (
          <IssueNotice
            issue={session.tableState.issue}
            onDismiss={session.dismissIssue}
            onAction={(action) => {
              if (action === "backToLobby" || action === "editInvite")
                router.replace("/lobby");
              else if (action === "signInAgain") router.replace("/");
              else if (action === "retry") {
                attemptedTableIdRef.current = null;
                void openTable(expectedTableId);
              }
            }}
          />
        )}
      </main>
    );
  }

  if (table.state === "WAITING") {
    return (
      <main className="table-client waiting-client">
        <WaitingTableStatusBar connectionState={session.connectionState} />
        {session.tableState.issue === null ? null : (
          <IssueNotice
            compact
            issue={session.tableState.issue}
            onDismiss={session.dismissIssue}
            onAction={(action) => {
              if (action === "retry") session.reconnect();
              else if (action === "resync") session.resync();
            }}
          />
        )}
        <WaitingRoom
          table={table}
          orientation={orientation}
          session={session}
          commandDisabled={commandDisabled}
          shareUrl={shareUrl}
          copied={copied}
          onCopy={() => void copyInvite()}
          onLeaveTable={() => void returnToLobby()}
        />
      </main>
    );
  }

  const dummySeat =
    game?.auction.contract === undefined
      ? undefined
      : oppositeSeat(game.auction.contract.declarer);
  const dummyPosition =
    dummySeat === undefined
      ? undefined
      : visualPositionForSeat(orientation, dummySeat);
  const viewerIsDummy =
    dummySeat !== undefined && table.viewerSeat === dummySeat;
  return (
    <main className="table-client active-table-client">
      <ActiveTableStatusBar
        table={table}
        connectionState={session.connectionState}
        inviteCode={session.inviteCode}
        copied={copied}
        onCopy={() => void copyInvite()}
      />

      <div className="table-feedback" aria-live="polite">
        {session.tableState.issue === null ? null : (
          <IssueNotice
            compact
            issue={session.tableState.issue}
            onDismiss={session.dismissIssue}
            onAction={(action) => {
              if (action === "resync") session.resync();
              else if (action === "takeover")
                session.sendCommand("table.takeover");
              else if (action === "retry") session.reconnect();
              else if (action === "backToLobby") router.push("/lobby");
              else if (action === "signInAgain")
                void session.logout().then(() => router.replace("/"));
            }}
          />
        )}
        {session.tableState.notice === null ? null : (
          <div className="success-notice" role="status">
            <span>{session.tableState.notice}</span>
            <button
              type="button"
              onClick={session.dismissNotice}
              aria-label="Tutup konfirmasi"
            >
              ×
            </button>
          </div>
        )}
      </div>

      <TableSurface
        table={table}
        orientation={orientation}
        session={session}
        commandDisabled={commandDisabled}
      >
        <ConsensusControls
          table={table}
          disabled={commandDisabled}
          onCommand={session.sendCommand}
        />
        {game?.phase === "AUCTION" ? (
          <div className="auction-workspace">
            <AuctionTable game={game} />
            <BiddingBox
              legalCalls={game.legalCalls ?? []}
              disabled={gameplayDisabled || !viewerTurn}
              onCall={(call) => session.sendCommand("game.make_call", { call })}
            />
          </div>
        ) : null}
        {game !== undefined &&
        (game.phase === "OPENING_LEAD" || game.phase === "PLAY") ? (
          <>
            {game.dummyHand === undefined ||
            dummyPosition === undefined ||
            viewerIsDummy ? null : (
              <BridgeHand
                className={`dummy-hand dummy-${dummyPosition}`}
                title={``}
                variant="dummy"
                cards={game.dummyHand}
                playableCards={
                  legalPlay?.source === "dummy" ? legalPlay.hand : []
                }
                disabled={gameplayDisabled}
                onPlay={(card) =>
                  session.sendCommand("game.play_card", { card })
                }
              />
            )}
            <CurrentTrick game={game} orientation={orientation} />
          </>
        ) : null}
        {game?.result === undefined &&
        table.state !== "FINISHED" ? null : table.state === "FINISHED" ? (
          <section className="board-result finished-result">
            <p>Meja selesai</p>
            <h2>Terima kasih sudah bermain.</h2>
            <button
              className="primary-button"
              type="button"
              disabled={session.busy}
              onClick={() => void returnToLobby()}
            >
              Kembali ke lobby
            </button>
          </section>
        ) : (
          <BoardResult
            table={table}
            commandDisabled={gameplayDisabled}
            onCommand={session.sendCommand}
          />
        )}
      </TableSurface>

      {game === undefined ? null : (
        <BridgeHand
          className="own-hand"
          title={``}
          cards={game.ownHand}
          playableCards={legalPlay?.source === "own" ? legalPlay.hand : []}
          disabled={gameplayDisabled || viewerIsDummy}
          {...(game.phase === "OPENING_LEAD" || game.phase === "PLAY"
            ? {
                onPlay: (card: Card) =>
                  session.sendCommand("game.play_card", { card }),
              }
            : {})}
        />
      )}
    </main>
  );
}
