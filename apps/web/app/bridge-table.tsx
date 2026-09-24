"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useMemo, useRef, useState, useSyncExternalStore } from "react";
import { usePositionAnalysis } from "./use-position-analysis";
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
import { CompletedDeal } from "./table/completed-deal";
import { CurrentTrick } from "./table/current-trick";
import { callKey } from "./table/gameplay-presentation";
import { BridgeHand } from "./table/playing-card";
import {
  ActiveTableStatusBar,
  WaitingTableStatusBar,
} from "./table/table-status-bar";
import { TableChat } from "./table/table-chat";
import { TableSurface } from "./table/table-surface";
import { useGameplayMotion } from "./table/use-gameplay-motion";
import { WaitingRoom } from "./table/waiting-room";
import { useTableSession, type TableSession } from "./use-table-session";
import { useTurnAudio } from "./use-turn-audio";

function subscribeDocumentVisibility(listener: () => void) {
  document.addEventListener("visibilitychange", listener);
  return () => document.removeEventListener("visibilitychange", listener);
}

function documentVisible() {
  return document.visibilityState === "visible";
}

export default function BridgeTable({
  expectedTableId,
  visible = true,
}: {
  expectedTableId: string;
  visible?: boolean;
}) {
  const router = useRouter();
  const session = useTableSession();
  const documentIsVisible = useSyncExternalStore(
    subscribeDocumentVisibility,
    documentVisible,
    () => true,
  );
  const presentationVisible = visible && documentIsVisible;
  const { canSendCommand, openTable, sendCommand } = session;
  const canSendVisibleCommand: TableSession["canSendCommand"] = (name, payload) =>
    presentationVisible && canSendCommand(name, payload);
  const sendVisibleCommand: TableSession["sendCommand"] = (name, payload) => {
    if (canSendVisibleCommand(name, payload)) sendCommand(name, payload);
  };
  const attemptedTableIdRef = useRef<string | null>(null);
  const table = session.projectedTable;
  const game = table?.game;
  const orientation = useMemo(
    () => tableOrientation(table?.viewerSeat),
    [table?.viewerSeat],
  );
  const legalPlay = table === null ? null : playableHand(table);
  const visibleDummyTurn = game?.auction.contract !== undefined && game.turn === oppositeSeat(game.auction.contract.declarer) && game.dummyHand !== undefined;
  const analysisHand = visibleDummyTurn && table && game?.turn && game.dummyHand
    ? playableHand({ ...table, viewerSeat: game.turn, game: { ...game, ownHand: game.dummyHand } })
    : legalPlay;
  const [doubleDummy, setDoubleDummy] = useState(false);
  const analysis = usePositionAnalysis(session.loadPositionAnalysis, table?.boardId,
    String(table?.revision), doubleDummy && analysisHand !== null && (game?.phase === "PLAY" || game?.phase === "OPENING_LEAD") && Object.keys(session.tableState.pending).length === 0);
  const viewerTurn = game?.turn === table?.viewerSeat;
  const motion = useGameplayMotion(game, presentationVisible);
  const turnAudio = useTurnAudio(session.tableState.table, presentationVisible);

  useEffect(() => {
    if (!visible) return;
    if (session.initializing) return;
    if (session.nickname === null && session.tableState.issue === null) {
      router.replace("/");
      return;
    }
    if (session.recoveryState === "TABLE_EXPIRED") {
      router.replace("/lobby");
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
    session.recoveryState,
    session.tableState.issue,
    table?.tableId,
    visible,
  ]);

  useEffect(() => {
    function handleAuctionKeyboard(event: KeyboardEvent) {
      if (
        !presentationVisible ||
        event.repeat ||
        (event.target instanceof HTMLElement && event.target.isContentEditable) ||
        document.querySelector("dialog[open]") !== null ||
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
  }, [canSendCommand, game, presentationVisible, sendCommand, viewerTurn]);

  async function returnToLobby() {
    if (table?.matchId) { router.push(`/match/${table.matchId}`); return; }
    if (await session.leaveTable()) {
      router.replace("/lobby");
    }
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

  if (table.state === "WAITING" && table.matchId) {
    return <main className="table-route-state"><h1>Team Match</h1><p>Persiapan dan persetujuan delapan pemain ada di halaman match.</p><Link className="primary-button" href={`/match/${table.matchId}`}>Buka persiapan match</Link></main>;
  }

  if (table.state === "WAITING") {
    return (
      <main className="table-client waiting-client">
        <WaitingTableStatusBar
          inviteCode={session.inviteCode}
          loadBoardReplay={session.loadBoardReplay} loadPositionAnalysis={session.loadPositionAnalysis}
          table={table}
          connectionState={session.connectionState}
          onLeaveTable={() => void returnToLobby()}
        />
        <div className="play-board-layout waiting-play-layout">
          <div className="gameplay-region waiting-gameplay-region">
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
              presence={session.tableState.presence}
              inviteCode={session.inviteCode}
              canSendCommand={canSendVisibleCommand}
              onLeaveTable={() => void returnToLobby()}
              onCommand={sendVisibleCommand}
            />
          </div>
          {visible ? <TableChat tableId={table.tableId} /> : null}
        </div>
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
  const boardComplete = game?.phase === "BOARD_SCORED";
  return (
    <main
      className="table-client active-table-client play-board-client"
      data-board-complete={boardComplete}
    >
      <ActiveTableStatusBar
        loadBoardReplay={session.loadBoardReplay} loadPositionAnalysis={session.loadPositionAnalysis}
        table={table}
        connectionState={session.connectionState}
        inviteCode={session.inviteCode}
        canSendCommand={canSendVisibleCommand}
        onCommand={sendVisibleCommand}
        analysisControl={table.matchId ? null : <button type="button" aria-pressed={doubleDummy} onClick={() => setDoubleDummy((value) => !value)} title="Predicted total tricks untuk pasangan yang sedang turn">DD {doubleDummy ? "ON" : "OFF"}</button>}
        soundMuted={turnAudio.muted}
        onSoundMutedChange={turnAudio.setMuted}
        onLeaveTable={() => void returnToLobby()}
      />

      <div className="play-board-layout">
        <div className="gameplay-region">
      <div className="table-feedback" aria-live="polite">
        {analysis.failed ? <span role="status">DDS tidak tersedia untuk posisi ini.</span> : null}
        {session.tableState.issue === null ? null : (
          <IssueNotice
            compact
            issue={session.tableState.issue}
            onDismiss={session.dismissIssue}
            onAction={(action) => {
              if (action === "resync") session.resync();
              else if (action === "retry") session.reconnect();
              else if (action === "backToLobby") void returnToLobby();
              else if (action === "signInAgain")
                void session.logout().then((loggedOut) => { if (loggedOut) window.location.replace("/login"); });
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
        canSendCommand={canSendVisibleCommand}
        onCommand={sendVisibleCommand}
        onBoardClick={motion.skipCurrent}
      >
        {game === undefined || !boardComplete ? null : (
          <CompletedDeal game={game} orientation={orientation} />
        )}
        {game?.phase === "AUCTION" ? (
          <div className="auction-workspace">
            <AuctionTable game={game} showSummary={false} />
            <BiddingBox
              legalCalls={game.legalCalls ?? []}
              disabled={!viewerTurn}
              canCall={(call) => canSendVisibleCommand("game.make_call", { call })}
              onCall={(call) => sendVisibleCommand("game.make_call", { call })}
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
                position={dummyPosition}
                cards={game.dummyHand}
                analysisCards={visibleDummyTurn ? analysisHand?.hand : undefined}
                predictions={visibleDummyTurn ? analysis.result?.cards : undefined}
                analysisPending={visibleDummyTurn && analysis.pending}
                contractStrain={game.auction.contract?.strain}
                playableCards={
                  legalPlay?.source === "dummy"
                    ? legalPlay.hand.filter((card) =>
                        canSendVisibleCommand("game.play_card", { card }),
                      )
                    : []
                }
                onPlay={(card) =>
                  sendVisibleCommand("game.play_card", { card })
                }
              />
            )}
            <CurrentTrick
              trick={motion.frame.trick}
              orientation={orientation}
              stage={motion.frame.stage}
              {...(dummyPosition === undefined ? {} : { dummyPosition })}
              {...(motion.frame.movingSeat === undefined
                ? {}
                : { movingSeat: motion.frame.movingSeat })}
            />
          </>
        ) : null}
        {game?.result !== undefined && table.state !== "FINISHED" ? (
          <BoardResult
            table={table}
            autoAdvance={presentationVisible}
            canSendCommand={canSendVisibleCommand}
            onCommand={sendVisibleCommand}
          />
        ) : null}
      </TableSurface>

      {table.state === "FINISHED" ? (
        <section className="board-result finished-result">
          <p>{table.matchId ? "Room selesai" : "Meja selesai"}</p>
          <h2>Terima kasih sudah bermain.</h2>
          <button
            className="primary-button"
            type="button"
            disabled={session.busy}
            onClick={() => void returnToLobby()}
          >
            {table.matchId ? "Lihat progres match" : "Kembali ke lobby"}
          </button>
        </section>
      ) : game === undefined || boardComplete ? null : (
        <BridgeHand
          className={`own-hand ${game.phase === "AUCTION" ? "own-hand-auction" : "own-hand-play"}`}
          title="Kartu Anda"
          cards={game.ownHand}
          analysisCards={game.turn === table.viewerSeat ? analysisHand?.hand : undefined}
          predictions={game.turn === table.viewerSeat ? analysis.result?.cards : undefined}
          analysisPending={game.turn === table.viewerSeat && analysis.pending}
          contractStrain={game.auction.contract?.strain}
          playableCards={
            legalPlay?.source === "own"
              ? legalPlay.hand.filter((card) =>
                  canSendVisibleCommand("game.play_card", { card }),
                )
              : []
          }
          disabled={viewerIsDummy}
          {...(game.phase === "OPENING_LEAD" || game.phase === "PLAY"
            ? {
                onPlay: (card: Card) =>
                  sendVisibleCommand("game.play_card", { card }),
              }
            : {})}
        />
      )}
        </div>
        {visible ? <TableChat tableId={table.tableId} /> : null}
      </div>
    </main>
  );
}
