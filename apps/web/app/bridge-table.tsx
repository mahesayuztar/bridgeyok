"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  useEffect,
  useId,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { createPortal } from "react-dom";
import IssueNotice from "./issue-notice";
import {
  auctionRows,
  boardResultLabel,
  oppositeSeat,
  playableHand,
  tableOrientation,
  visualPositionForSeat,
  type Call,
  type Card,
  type LiveTableProjection,
  type ParticipantPresence,
  type Seat,
  type TableOrientation,
  type VisualPosition,
} from "./table-state";
import {
  callKey,
  callLabel,
  contractLabel,
  suitLabels,
} from "./table/gameplay-presentation";
import { BridgeHand, PlayingCard } from "./table/playing-card";
import { useTableSession, type TableSession } from "./use-table-session";

const SEATS: Seat[] = ["N", "E", "S", "W"];
const AUCTION_SEATS: Seat[] = ["W", "N", "E", "S"];
const STRAINS: Array<"C" | "D" | "H" | "S" | "NT"> = ["S", "H", "D", "C", "NT"];
const VULNERABILITY_LABELS = {
  NONE: "Tidak ada",
  NS: "NS",
  EW: "EW",
  BOTH: "Keduanya",
};
const CONNECTION_LABELS = {
  idle: "Belum terhubung",
  connecting: "Menghubungkan",
  syncing: "Menyelaraskan",
  connected: "Terhubung",
  degraded: "Koneksi terganggu",
  offline: "Offline",
};

function participantName(
  table: LiveTableProjection,
  participantId: string | undefined,
) {
  if (participantId === undefined) {
    return null;
  }
  return (
    table.participants.find((participant) => participant.id === participantId)
      ?.nickname ?? "Pemain"
  );
}

function PlayerPosition({
  table,
  presence,
  seat,
  position,
  disabled,
  turn,
  onCommand,
  onRemove,
  onLeaveTable,
}: {
  table: LiveTableProjection;
  presence: Record<string, ParticipantPresence>;
  seat: Seat;
  position: VisualPosition;
  disabled: boolean;
  turn: boolean;
  onCommand: TableSession["sendCommand"];
  onRemove?: (participantId: string) => void;
  onLeaveTable?: () => void;
}) {
  const dialogTitleId = useId();
  const closeButtonRef = useRef<HTMLButtonElement>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const [portalOpen, setPortalOpen] = useState(false);
  const assignment = table.seats[seat];
  const isViewer = table.viewerParticipantId === assignment?.participantId;
  const name = participantName(table, assignment?.participantId);
  const participant = table.participants.find(
    (candidate) => candidate.id === assignment?.participantId,
  );
  const isBot = participant?.isBot === true || assignment?.isBot === true;
  const participantPresence =
    assignment === undefined || isBot
      ? undefined
      : presence[assignment.participantId];
  const canTakeSeat =
    table.state === "WAITING" ||
    (table.viewerSeat === undefined &&
      (table.state === "ACTIVE" || table.state === "BETWEEN_BOARDS"));
  const canAddBot =
    assignment === undefined &&
    table.viewerRole === "OWNER" &&
    table.state !== "FINISHED";
  const canRemove =
    assignment !== undefined &&
    !isBot &&
    onRemove !== undefined &&
    (!isViewer || table.participants.length > 1);
  const canReplaceWithBot =
    assignment !== undefined &&
    !isBot &&
    !isViewer &&
    participant?.role === "PARTICIPANT" &&
    onRemove !== undefined;
  const canRemoveBot =
    assignment !== undefined && isBot && table.viewerRole === "OWNER";
  const canManageOwnSeat = isViewer && table.state === "WAITING";
  const hasPortalActions =
    (assignment === undefined && (canTakeSeat || canAddBot)) ||
    canManageOwnSeat ||
    canRemove ||
    canReplaceWithBot ||
    canRemoveBot ||
    (isViewer && onLeaveTable !== undefined);

  useEffect(() => {
    if (!portalOpen) return;
    const trigger = triggerRef.current;
    closeButtonRef.current?.focus();
    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") setPortalOpen(false);
    }
    window.addEventListener("keydown", handleKeyDown);
    return () => {
      window.removeEventListener("keydown", handleKeyDown);
      trigger?.focus();
    };
  }, [portalOpen]);

  const crown =
    participant?.role === "OWNER" ? (
      <svg
        className="owner-crown"
        viewBox="0 0 24 24"
        role="img"
        aria-label="Pemilik meja"
      >
        <path d="M3 7.5 7.5 12 12 5l4.5 7L21 7.5l-2 10H5l-2-10Zm3 12h12" />
      </svg>
    ) : null;
  const portal =
    portalOpen && typeof document !== "undefined"
      ? createPortal(
          <div
            className="participant-portal-layer"
            role="presentation"
            onMouseDown={() => setPortalOpen(false)}
          >
            <section
              className="participant-portal"
              role="dialog"
              aria-modal="true"
              aria-labelledby={dialogTitleId}
              onMouseDown={(event) => event.stopPropagation()}
            >
              <header>
                <span className="participant-portal-seat">{seat}</span>
                <div>
                  <h2 id={dialogTitleId}>{name ?? "Kursi kosong"}</h2>
                  {assignment === undefined ? (
                    <p>Pilih siapa yang mengisi kursi ini.</p>
                  ) : table.state === "WAITING" ? (
                    <p>
                      {isBot ? "Bot · " : ""}
                      {assignment.ready ? "Siap" : "Belum siap"}
                    </p>
                  ) : null}
                </div>
                {crown}
                <button
                  ref={closeButtonRef}
                  className="participant-portal-close"
                  type="button"
                  onClick={() => setPortalOpen(false)}
                  aria-label="Tutup menu pemain"
                >
                  ×
                </button>
              </header>
              <div className="participant-portal-actions">
                {assignment !== undefined || !canTakeSeat ? null : (
                  <button
                    type="button"
                    disabled={disabled}
                    onClick={() => {
                      setPortalOpen(false);
                      onCommand("table.take_seat", { seat });
                    }}
                  >
                    Duduk di kursi
                  </button>
                )}
                {!canAddBot ? null : (
                  <button
                    type="button"
                    disabled={disabled}
                    onClick={() => {
                      setPortalOpen(false);
                      onCommand("table.add_bot", { seat });
                    }}
                  >
                    Tambah bot
                  </button>
                )}
                {!canManageOwnSeat ? null : (
                  <button
                    type="button"
                    disabled={disabled}
                    onClick={() => {
                      setPortalOpen(false);
                      onCommand("table.set_ready", {
                        ready: !assignment.ready,
                      });
                    }}
                  >
                    {assignment.ready ? "Batalkan siap" : "Saya siap"}
                  </button>
                )}
                {!canManageOwnSeat ? null : (
                  <button
                    type="button"
                    disabled={disabled}
                    onClick={() => {
                      setPortalOpen(false);
                      onCommand("table.leave_seat");
                    }}
                  >
                    Berdiri dari kursi
                  </button>
                )}
                {!canRemove ? null : (
                  <button
                    type="button"
                    disabled={disabled}
                    onClick={() => {
                      setPortalOpen(false);
                      onRemove(assignment.participantId);
                    }}
                  >
                    {isViewer ? "Serahkan & keluar" : "Keluarkan"}
                  </button>
                )}
                {!canReplaceWithBot ? null : (
                  <button
                    type="button"
                    disabled={disabled}
                    onClick={() => {
                      setPortalOpen(false);
                      onCommand("table.replace_with_bot", {
                        participant_id: assignment.participantId,
                      });
                    }}
                  >
                    Keluarkan &amp; ganti bot
                  </button>
                )}
                {!canRemoveBot ? null : (
                  <button
                    type="button"
                    disabled={disabled}
                    onClick={() => {
                      setPortalOpen(false);
                      onCommand("table.remove_bot", { seat });
                    }}
                  >
                    Keluarkan bot
                  </button>
                )}
                {!isViewer || onLeaveTable === undefined ? null : (
                  <button
                    type="button"
                    disabled={disabled}
                    onClick={() => {
                      setPortalOpen(false);
                      onLeaveTable();
                    }}
                  >
                    Tinggalkan meja
                  </button>
                )}
                {hasPortalActions ? null : (
                  <p>Belum ada tindakan untuk pemain ini.</p>
                )}
              </div>
            </section>
          </div>,
          document.body,
        )
      : null;

  return (
    <div
      className={`player-position player-${position}`}
      data-occupied={assignment !== undefined}
      data-turn={turn}
      data-presence={
        isBot
          ? "bot"
          : participantPresence === undefined
            ? "unknown"
            : participantPresence.online
              ? "online"
              : "offline"
      }
    >
      {assignment === undefined ? (
        <button
          ref={triggerRef}
          className="empty-seat-trigger"
          type="button"
          onClick={() => setPortalOpen(true)}
          aria-label={`Buka menu kursi kosong ${seat}`}
        >
          <span className="player-seat">{seat}</span>
          <span>Kosong</span>
        </button>
      ) : (
        <button
          ref={triggerRef}
          className="player-trigger"
          type="button"
          onClick={() => setPortalOpen(true)}
          aria-label={`Buka menu ${name}, kursi ${seat}`}
        >
          <span className="player-seat">{seat}</span>
          <span className="player-copy">
            <strong>
              {name}
              {crown}
            </strong>
            {table.state === "WAITING" ? (
              <span>
                {isBot ? "Bot · " : ""}
                {assignment.ready ? "Siap" : "Belum siap"}
              </span>
            ) : null}
            <span className="sr-only">
              {isBot
                ? "Bot"
                : participantPresence?.online === false
                  ? "Tidak tersambung"
                  : "Tersambung"}
              {turn ? ", sedang bermain" : ""}
            </span>
          </span>
        </button>
      )}
      {portal}
    </div>
  );
}

function AuctionTable({
  game,
}: {
  game: NonNullable<LiveTableProjection["game"]>;
}) {
  const rows = auctionRows(game.auction.dealer, game.auction.calls);
  return (
    <div className="auction-table-wrap">
      <div className="auction-caption">
        <strong>Lelang</strong>
        <span>Dealer {game.auction.dealer}</span>
      </div>
      <table className="auction-table">
        <thead>
          <tr>
            {AUCTION_SEATS.map((seat) => (
              <th key={seat} scope="col" data-turn={game.turn === seat}>
                {seat}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((row, _rowIndex) => (
            <tr key={_rowIndex}>
              {AUCTION_SEATS.map((seat) => {
                const record = row[seat];
                return (
                  <td
                    key={seat}
                    className={
                      record?.call.strain === "H" || record?.call.strain === "D"
                        ? "red-call"
                        : ""
                    }
                  >
                    {record === undefined ? null : callLabel(record.call)}
                  </td>
                );
              })}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function BiddingBox({
  legalCalls,
  disabled,
  onCall,
}: {
  legalCalls: Call[];
  disabled: boolean;
  onCall: (call: Call) => void;
}) {
  const [selectedLevel, setSelectedLevel] = useState<number | null>(null);
  const legalKeys = new Set(legalCalls.map(callKey));
  const legalLevels = new Set(
    legalCalls.filter((call) => call.kind === "BID").map((call) => call.level),
  );
  const actionCalls: Array<{ label: string; call: Call; shortcut: string }> = [
    { label: "Pass", call: { kind: "PASS" }, shortcut: "P" },
    { label: "X", call: { kind: "DOUBLE" }, shortcut: "X" },
    { label: "XX", call: { kind: "REDOUBLE" }, shortcut: "R" },
  ];
  return (
    <section className="bidding-box" aria-label="Kotak lelang">
      <div className="call-actions">
        {actionCalls.map(({ label, call, shortcut }) => (
          <button
            type="button"
            key={label}
            disabled={disabled || !legalKeys.has(callKey(call))}
            onClick={() => {
              setSelectedLevel(null);
              onCall(call);
            }}
          >
            {label}
            <kbd>{shortcut}</kbd>
          </button>
        ))}
      </div>
      <div className="bid-levels" aria-label="Pilih level bid">
        {[1, 2, 3, 4, 5, 6, 7].map((level) => (
          <button
            type="button"
            key={level}
            aria-pressed={selectedLevel === level}
            disabled={disabled || !legalLevels.has(level)}
            onClick={() => setSelectedLevel(level)}
          >
            {level}
          </button>
        ))}
      </div>
      {selectedLevel === null ? (
        <p className="bid-hint">Pilih level, lalu pilih strain.</p>
      ) : (
        <div
          className="bid-strains"
          aria-label={`Pilih strain untuk level ${selectedLevel}`}
        >
          {STRAINS.map((strain) => {
            const call: Call = { kind: "BID", level: selectedLevel, strain };
            return (
              <button
                className={strain === "H" || strain === "D" ? "red-call" : ""}
                type="button"
                key={strain}
                disabled={disabled || !legalKeys.has(callKey(call))}
                onClick={() => {
                  setSelectedLevel(null);
                  onCall(call);
                }}
              >
                <span className="sr-only">Bid {selectedLevel} </span>
                {strain === "NT" ? "NT" : suitLabels[strain]}
              </button>
            );
          })}
        </div>
      )}
    </section>
  );
}

function CurrentTrick({
  game,
  orientation,
}: {
  game: NonNullable<LiveTableProjection["game"]>;
  orientation: TableOrientation;
}) {
  return (
    <div className="current-trick" aria-label="Trick saat ini">
      <span className="trick-center">
        {game.currentTrick.plays.length === 0 ? "Lead" : ""}
      </span>
      {game.currentTrick.plays.map((play) => (
        <div
          className={`trick-slot trick-${visualPositionForSeat(orientation, play.seat)}`}
          key={play.seat}
        >
          <span>{play.seat}</span>
          <PlayingCard card={play.card} variant="trick" />
        </div>
      ))}
    </div>
  );
}

function ConsensusControls({
  table,
  disabled,
  onCommand,
}: {
  table: LiveTableProjection;
  disabled: boolean;
  onCommand: TableSession["sendCommand"];
}) {
  const game = table.game;
  const request = table.actionRequest;
  const dummy =
    game?.auction.contract === undefined
      ? undefined
      : oppositeSeat(game.auction.contract.declarer);
  const hasBot = table.participants.some((participant) => participant.isBot);
  const canClaim =
    !hasBot &&
    request === undefined &&
    game?.phase === "PLAY" &&
    game.currentTrick.plays.length === 0 &&
    table.viewerSeat !== undefined &&
    table.viewerSeat !== dummy;
  const remainingTricks =
    game === undefined ? 0 : 13 - game.completedTricks.length;
  if (request !== undefined) {
    const responseCommand =
      request.kind === "CLAIM" ? "game.respond_claim" : "game.respond_undo";
    return (
      <section className="consensus-controls" aria-live="polite">
        <strong>
          {request.kind === "CLAIM"
            ? `${request.requesterSeat} mengajukan ${request.claimTricks} trick`
            : `${request.requesterSeat} meminta undo`}
        </strong>
        <span>{request.approvedBy.length} persetujuan diterima</span>
        {!request.canRespond ? (
          <span>Menunggu pemain lain.</span>
        ) : (
          <div>
            <button
              className="primary-button"
              type="button"
              disabled={disabled}
              onClick={() => onCommand(responseCommand, { accepted: true })}
            >
              Terima
            </button>
            <button
              type="button"
              disabled={disabled}
              onClick={() => onCommand(responseCommand, { accepted: false })}
            >
              Tolak
            </button>
          </div>
        )}
      </section>
    );
  }
  if (!canClaim && !table.canRequestUndo) return null;
  return (
    <div className="consensus-controls consensus-actions">
      {canClaim ? (
        <details>
          <summary>Claim</summary>
          <div>
            {Array.from({ length: remainingTricks + 1 }, (_, tricks) => (
              <button
                type="button"
                key={tricks}
                disabled={disabled}
                onClick={() => onCommand("game.request_claim", { tricks })}
              >
                {tricks}
              </button>
            ))}
          </div>
        </details>
      ) : null}
      {table.canRequestUndo ? (
        <button
          type="button"
          disabled={disabled}
          onClick={() => onCommand("game.request_undo")}
        >
          Undo aksi terakhir
        </button>
      ) : null}
    </div>
  );
}

function TableSurface({
  table,
  orientation,
  session,
  commandDisabled,
  children,
}: {
  table: LiveTableProjection;
  orientation: TableOrientation;
  session: TableSession;
  commandDisabled: boolean;
  children: ReactNode;
}) {
  const isOwner = table.viewerRole === "OWNER";
  return (
    <div className="table-surface">
      {(Object.entries(orientation) as Array<[VisualPosition, Seat]>).map(
        ([position, seat]) => (
          <PlayerPosition
            key={seat}
            table={table}
            presence={session.tableState.presence}
            seat={seat}
            position={position}
            disabled={commandDisabled}
            turn={table.game?.turn === seat}
            onCommand={session.sendCommand}
            {...(isOwner
              ? {
                  onRemove: (participantId: string) =>
                    session.sendCommand("table.remove_participant", {
                      participant_id: participantId,
                    }),
                }
              : {})}
          />
        ),
      )}
      {children}
    </div>
  );
}

function WaitingRoom({
  table,
  orientation,
  session,
  commandDisabled,
  shareUrl,
  copied,
  onCopy,
  onLeaveTable,
}: {
  table: LiveTableProjection;
  orientation: TableOrientation;
  session: TableSession;
  commandDisabled: boolean;
  shareUrl: string;
  copied: boolean;
  onCopy: () => void;
  onLeaveTable: () => void;
}) {
  const allReady = SEATS.every((seat) => table.seats[seat]?.ready);
  const isOwner = table.viewerRole === "OWNER";
  return (
    <div className="waiting-room">
      <div className="waiting-copy">
        <p className="eyebrow">Ruang tunggu</p>
        <h1>Susun empat kursi.</h1>
        <p>
          {table.participants.length}/4 pemain sudah masuk. Pilih kursi, tandai
          siap, lalu pemilik dapat memulai board.
        </p>
        {session.inviteCode === null ? null : (
          <div className="invite-inline">
            <span>Kode undangan</span>
            <strong>{session.inviteCode}</strong>
            <button type="button" onClick={onCopy}>
              {copied ? "Tautan disalin" : "Salin tautan"}
            </button>
            <span className="sr-only">{shareUrl}</span>
          </div>
        )}
      </div>
      <div className="waiting-table table-surface">
        {(Object.entries(orientation) as Array<[VisualPosition, Seat]>).map(
          ([position, seat]) => (
            <PlayerPosition
              key={seat}
              table={table}
              presence={session.tableState.presence}
              seat={seat}
              position={position}
              disabled={commandDisabled}
              turn={false}
              onCommand={session.sendCommand}
              {...(isOwner
                ? {
                    onRemove: (participantId: string) =>
                      session.sendCommand("table.remove_participant", {
                        participant_id: participantId,
                      }),
                  }
                : {})}
              {...(table.viewerParticipantId ===
                table.seats[seat]?.participantId && !isOwner
                ? { onLeaveTable }
                : {})}
            />
          ),
        )}
        <div className="waiting-center">
          <strong>{table.locked ? "Meja dikunci" : "Meja terbuka"}</strong>
          <span>
            {SEATS.filter((seat) => table.seats[seat]?.ready).length}/4 siap
          </span>
        </div>
      </div>
      <div className="waiting-controls">
        {table.viewerSeat === undefined ? (
          <p>Pilih kursi kosong pada meja.</p>
        ) : null}
        {isOwner ? (
          <>
            <button
              type="button"
              disabled={commandDisabled}
              onClick={() =>
                session.sendCommand("table.lock", { locked: !table.locked })
              }
            >
              {table.locked ? "Buka meja" : "Kunci meja"}
            </button>
            <button
              className="start-board-button"
              type="button"
              disabled={commandDisabled || !allReady}
              onClick={() => session.sendCommand("table.start_game")}
            >
              Mulai board
            </button>
            <button
              type="button"
              disabled={commandDisabled}
              onClick={() => session.sendCommand("table.finish")}
            >
              Akhiri meja
            </button>
          </>
        ) : null}
      </div>
    </div>
  );
}

function BoardResult({
  table,
  session,
  commandDisabled,
}: {
  table: LiveTableProjection;
  session: TableSession;
  commandDisabled: boolean;
}) {
  const result = table.game?.result;
  if (result === undefined) return null;
  return (
    <section className="board-result" aria-labelledby="result-title">
      <p>Board {table.game?.board.number} selesai</p>
      <h2 id="result-title">
        {result.passedOut
          ? "Passed out"
          : contractLabel(result.contract, false)}
      </h2>
      <strong className="result-outcome">{boardResultLabel(result)}</strong>
      <dl>
        <div>
          <dt>Declarer</dt>
          <dd>{result.contract?.declarer ?? "—"}</dd>
        </div>
        <div>
          <dt>Trick</dt>
          <dd>{result.tricksDeclarer}</dd>
        </div>
        <div>
          <dt>NS</dt>
          <dd>
            {result.scoreNS > 0 ? "+" : ""}
            {result.scoreNS}
          </dd>
        </div>
        <div>
          <dt>EW</dt>
          <dd>
            {result.scoreNS < 0 ? "+" : ""}
            {-result.scoreNS}
          </dd>
        </div>
      </dl>
      {table.viewerRole === "OWNER" && table.state === "BETWEEN_BOARDS" ? (
        <div className="result-actions">
          <button
            className="primary-button"
            type="button"
            disabled={commandDisabled}
            onClick={() => session.sendCommand("table.next_board")}
          >
            Board berikutnya
          </button>
          <button
            type="button"
            disabled={commandDisabled}
            onClick={() => session.sendCommand("table.finish")}
          >
            Akhiri meja
          </button>
        </div>
      ) : (
        <p className="result-waiting">
          Menunggu pemilik menentukan board berikutnya.
        </p>
      )}
    </section>
  );
}

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
        <header className="table-status-bar">
          <Link
            className="table-wordmark"
            href="/lobby"
            aria-label="Kembali ke lobby"
          >
            BridgeYok
          </Link>
          <span>Meja tunggu</span>
          <div
            className="connection-status"
            data-state={session.connectionState}
          >
            <span className="status-mark" />
            {CONNECTION_LABELS[session.connectionState]}
          </div>
        </header>
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
      <header className="table-status-bar">
        <Link
          className="table-wordmark"
          href="/lobby"
          aria-label="Kembali ke lobby"
        >
          BY
        </Link>
        <dl className="table-facts">
          <div>
            <dt>Board</dt>
            <dd>{game?.board.number ?? table.boardNumber}</dd>
          </div>
          <div>
            <dt>Dealer</dt>
            <dd>{game?.board.dealer ?? "—"}</dd>
          </div>
          <div>
            <dt>Vul</dt>
            <dd>
              {game === undefined
                ? "—"
                : VULNERABILITY_LABELS[game.board.vulnerability]}
            </dd>
          </div>
          <div>
            <dt>Kontrak</dt>
            <dd>
              {game === undefined
                ? "—"
                : contractLabel(game.auction.contract, false)}
            </dd>
          </div>
          <div>
            <dt>Trick</dt>
            <dd>
              {game === undefined ? "—" : `${game.tricksNS}–${game.tricksEW}`}
            </dd>
          </div>
        </dl>
        <div className="status-actions">
          <div
            className="connection-status"
            data-state={session.connectionState}
            role="status"
          >
            <span className="status-mark" />
            {CONNECTION_LABELS[session.connectionState]}
          </div>
          <details className="table-menu">
            <summary aria-label="Buka menu meja">•••</summary>
            <div>
              <Link href="/lobby">Lobby</Link>
              {session.inviteCode === null ? null : (
                <button type="button" onClick={() => void copyInvite()}>
                  {copied ? "Tautan disalin" : "Salin undangan"}
                </button>
              )}
              <span>Revision {table.revision}</span>
            </div>
          </details>
        </div>
      </header>

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
            session={session}
            commandDisabled={gameplayDisabled}
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
