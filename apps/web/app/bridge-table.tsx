"use client";

import { useRouter } from "next/navigation";
import { useEffect, useMemo, useRef } from "react";
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
import { CurrentTrick } from "./table/current-trick";
import { callKey } from "./table/gameplay-presentation";
import { BridgeHand } from "./table/playing-card";
import {
  ActiveTableStatusBar,
  WaitingTableStatusBar,
} from "./table/table-status-bar";
import { TableSurface } from "./table/table-surface";
import { useGameplayMotion } from "./table/use-gameplay-motion";
import { WaitingRoom } from "./table/waiting-room";
import { useTableSession } from "./use-table-session";
import { useTurnAudio } from "./use-turn-audio";

export default function BridgeTable({
  expectedTableId,
}: {
  expectedTableId: string;
}) {
  const router = useRouter();
  const session = useTableSession();
  const { canSendCommand, openTable, sendCommand } = session;
  const attemptedTableIdRef = useRef<string | null>(null);
  const table = session.projectedTable;
  const game = table?.game;
  const orientation = useMemo(
    () => tableOrientation(table?.viewerSeat),
    [table?.viewerSeat],
  );
  const legalPlay = table === null ? null : playableHand(table);
  const viewerTurn = game?.turn === table?.viewerSeat;
  const motion = useGameplayMotion(game);
  const turnAudio = useTurnAudio(session.tableState.table);

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
        !viewerTurn
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
        ) && canSendCommand("game.make_call", { call })
      ) {
        event.preventDefault();
        sendCommand("game.make_call", { call });
      }
    }
    window.addEventListener("keydown", handleAuctionKeyboard);
    return () => window.removeEventListener("keydown", handleAuctionKeyboard);
  }, [canSendCommand, game, sendCommand, viewerTurn]);

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
              else if (action === "takeover")
                session.sendCommand("table.takeover");
            }}
          />
        )}
        <WaitingRoom
          table={table}
          orientation={orientation}
          presence={session.tableState.presence}
          inviteCode={session.inviteCode}
          canSendCommand={session.canSendCommand}
          onLeaveTable={() => void returnToLobby()}
          onCommand={session.sendCommand}
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
        canSendCommand={session.canSendCommand}
        onCommand={session.sendCommand}
        soundMuted={turnAudio.muted}
        onSoundMutedChange={turnAudio.setMuted}
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
        presence={session.tableState.presence}
        canSendCommand={session.canSendCommand}
        onCommand={session.sendCommand}
        onBoardClick={motion.skipCurrent}
      >
        {game?.phase === "AUCTION" ? (
          <div className="auction-workspace">
            <AuctionTable game={game} />
            <BiddingBox
              legalCalls={game.legalCalls ?? []}
              disabled={!viewerTurn}
              canCall={(call) => session.canSendCommand("game.make_call", { call })}
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
                title="Kartu dummy"
                variant="dummy"
                cards={game.dummyHand}
                playableCards={
                  legalPlay?.source === "dummy"
                    ? legalPlay.hand.filter((card) =>
                        canSendCommand("game.play_card", { card }),
                      )
                    : []
                }
                onPlay={(card) =>
                  session.sendCommand("game.play_card", { card })
                }
              />
            )}
            <CurrentTrick
              trick={motion.frame.trick}
              orientation={orientation}
              stage={motion.frame.stage}
              {...(motion.frame.movingSeat === undefined
                ? {}
                : { movingSeat: motion.frame.movingSeat })}
            />
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
            canSendCommand={session.canSendCommand}
            onCommand={session.sendCommand}
          />
        )}
      </TableSurface>

      {game === undefined ? null : (
        <BridgeHand
          className="own-hand"
          title="Kartu Anda"
          cards={game.ownHand}
          playableCards={
            legalPlay?.source === "own"
              ? legalPlay.hand.filter((card) =>
                  canSendCommand("game.play_card", { card }),
                )
              : []
          }
          disabled={viewerIsDummy}
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
