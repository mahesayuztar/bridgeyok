"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useMemo, useRef, useState, type CSSProperties, type ReactNode } from "react";
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
  type Suit,
  type TableOrientation,
  type VisualPosition
} from "./table-state";
import { useTableSession, type TableSession } from "./use-table-session";

const SEATS: Seat[] = ["N", "E", "S", "W"];
const AUCTION_SEATS: Seat[] = ["W", "N", "E", "S"];
const STRAINS: Array<"C" | "D" | "H" | "S" | "NT"> = ["C", "D", "H", "S", "NT"];
const SUIT_LABELS: Record<Suit, string> = { S: "♠", H: "♥", D: "♦", C: "♣" };
const SEAT_LABELS: Record<Seat, string> = { N: "Utara", E: "Timur", S: "Selatan", W: "Barat" };
const VULNERABILITY_LABELS = { NONE: "Tidak ada", NS: "NS", EW: "EW", BOTH: "Keduanya" };
const CONNECTION_LABELS = {
  idle: "Belum terhubung",
  connecting: "Menghubungkan",
  syncing: "Menyelaraskan",
  connected: "Terhubung",
  degraded: "Koneksi terganggu",
  offline: "Offline"
};

function participantName(table: LiveTableProjection, participantId: string | undefined) {
  if (participantId === undefined) {
    return null;
  }
  return table.participants.find((participant) => participant.id === participantId)?.nickname ?? "Pemain";
}

function callLabel(call: Call) {
  if (call.kind === "PASS") return "Pass";
  if (call.kind === "DOUBLE") return "X";
  if (call.kind === "REDOUBLE") return "XX";
  const strain = call.strain === "NT" ? "NT" : SUIT_LABELS[call.strain ?? "C"];
  return `${call.level}${strain}`;
}

function callKey(call: Call) {
  return `${call.kind}:${call.level ?? ""}:${call.strain ?? ""}`;
}

function contractLabel(contract: NonNullable<LiveTableProjection["game"]>["auction"]["contract"], includeDeclarer = true) {
  if (contract === undefined) return "Belum ada kontrak";
  const doubling = contract.doubling === "DOUBLED" ? " X" : contract.doubling === "REDOUBLED" ? " XX" : "";
  const strain = contract.strain === "NT" ? "NT" : SUIT_LABELS[contract.strain];
  return `${contract.level}${strain}${doubling}${includeDeclarer ? ` oleh ${contract.declarer}` : ""}`;
}

function cardKey(card: Card) {
  return `${card.suit}${card.rank}`;
}

function presenceLabel(presence: ParticipantPresence | undefined, now: number, isOwner: boolean) {
  if (presence === undefined) return "Status belum tersedia";
  if (presence.online) return "Online";
  if (presence.expiresAt === undefined) return "Offline";
  if (now === 0) return "Offline · menghitung…";
  const remainingSeconds = Math.max(0, Math.ceil((Date.parse(presence.expiresAt) - now) / 1000));
  if (remainingSeconds === 0) return isOwner ? "Offline · menunggu master baru" : "Offline · segera keluar";
  return `Offline · 0:${String(remainingSeconds).padStart(2, "0")}`;
}

function PlayingCard({ card, variant, disabled = false, playable = false, onPlay }: {
  card: Card;
  variant: "hand" | "dummy" | "trick";
  disabled?: boolean;
  playable?: boolean;
  onPlay?: (card: Card) => void;
}) {
  const rank = card.rank === "T" ? "10" : card.rank;
  const content = <><span className="card-corner"><strong>{rank}</strong><span>{SUIT_LABELS[card.suit]}</span></span><span className="card-suit" aria-hidden="true">{SUIT_LABELS[card.suit]}</span></>;
  const className = `physical-card suit-${card.suit.toLowerCase()} card-${variant}`;
  const label = `${rank} ${SUIT_LABELS[card.suit]}`;

  if (onPlay === undefined) {
    return <span className={className} aria-label={label}>{content}</span>;
  }
  return <button className={className} type="button" disabled={disabled || !playable} onClick={() => onPlay(card)} aria-label={`Mainkan ${label}`}>{content}</button>;
}

function BridgeHand({ cards, title, variant = "hand", playableCards = [], disabled = false, onPlay, className = "" }: {
  cards: Card[];
  title: string;
  variant?: "hand" | "dummy";
  playableCards?: Card[];
  disabled?: boolean;
  onPlay?: (card: Card) => void;
  className?: string;
}) {
  const playableKeys = new Set(playableCards.map(cardKey));
  const style = { "--card-count": Math.max(cards.length, 1) } as CSSProperties;
  return (
    <section className={`bridge-hand ${className}`} data-variant={variant} aria-label={title}>
      <h3>{title}</h3>
      <div className="hand-cards" style={style}>
        {cards.map((card) => <PlayingCard card={card} variant={variant} key={cardKey(card)} disabled={disabled} playable={playableKeys.has(cardKey(card))} {...(onPlay === undefined ? {} : { onPlay })} />)}
      </div>
    </section>
  );
}

function PlayerPosition({ table, presence, now, seat, position, disabled, turn, onCommand, onRemove }: {
  table: LiveTableProjection;
  presence: Record<string, ParticipantPresence>;
  now: number;
  seat: Seat;
  position: VisualPosition;
  disabled: boolean;
  turn: boolean;
  onCommand: TableSession["sendCommand"];
  onRemove?: (participantId: string) => void;
}) {
  const assignment = table.seats[seat];
  const isViewer = table.viewerParticipantId === assignment?.participantId;
  const name = participantName(table, assignment?.participantId);
  const participant = table.participants.find((candidate) => candidate.id === assignment?.participantId);
  const participantPresence = assignment === undefined ? undefined : presence[assignment.participantId];
  const canTakeSeat = table.state === "WAITING" || table.viewerSeat === undefined && (table.state === "ACTIVE" || table.state === "BETWEEN_BOARDS");
  const canRemove = onRemove !== undefined && (!isViewer || table.participants.length > 1);
  return (
    <div className={`player-position player-${position}`} data-occupied={assignment !== undefined} data-turn={turn} data-presence={participantPresence === undefined ? "unknown" : participantPresence.online ? "online" : "offline"}>
      <span className="player-seat">{seat}</span>
      {assignment === undefined ? (
        <button type="button" disabled={disabled || !canTakeSeat} onClick={() => onCommand("table.take_seat", { seat })}>Duduk <span className="seat-name">di {SEAT_LABELS[seat]}</span></button>
      ) : (
        <div className="player-copy">
          <strong>{name}{isViewer ? " · kamu" : ""}{participant?.role === "OWNER" ? " · master" : ""}</strong>
          <span>{table.state === "WAITING" ? assignment.ready ? "Siap" : "Belum siap" : turn ? "Giliran" : SEAT_LABELS[seat]}</span>
          <span className="player-presence"><span className="presence-mark" />{presenceLabel(participantPresence, now, participant?.role === "OWNER")}</span>
          {!canRemove ? null : <button type="button" disabled={disabled} onClick={() => onRemove(assignment.participantId)}>{isViewer ? "Serahkan & keluar" : "Keluarkan"}</button>}
        </div>
      )}
    </div>
  );
}

function AuctionTable({ game }: { game: NonNullable<LiveTableProjection["game"]> }) {
  const rows = auctionRows(game.auction.dealer, game.auction.calls);
  return (
    <div className="auction-table-wrap">
      <div className="auction-caption"><strong>Lelang</strong><span>Dealer {game.auction.dealer}</span></div>
      <table className="auction-table">
        <thead><tr>{AUCTION_SEATS.map((seat) => <th key={seat} scope="col" data-turn={game.turn === seat}>{seat}</th>)}</tr></thead>
        <tbody>{rows.map((row, _rowIndex) => <tr key={_rowIndex}>{AUCTION_SEATS.map((seat) => {
          const record = row[seat];
          return <td key={seat} className={record?.call.strain === "H" || record?.call.strain === "D" ? "red-call" : ""}>{record === undefined ? null : callLabel(record.call)}</td>;
        })}</tr>)}</tbody>
      </table>
    </div>
  );
}

function BiddingBox({ legalCalls, disabled, onCall }: { legalCalls: Call[]; disabled: boolean; onCall: (call: Call) => void }) {
  const legalKeys = new Set(legalCalls.map(callKey));
  const actionCalls: Array<{ label: string; call: Call; shortcut: string }> = [
    { label: "Pass", call: { kind: "PASS" }, shortcut: "P" },
    { label: "X", call: { kind: "DOUBLE" }, shortcut: "X" },
    { label: "XX", call: { kind: "REDOUBLE" }, shortcut: "R" }
  ];
  return (
    <section className="bidding-box" aria-label="Kotak lelang">
      <div className="call-actions">{actionCalls.map(({ label, call, shortcut }) => <button type="button" key={label} disabled={disabled || !legalKeys.has(callKey(call))} onClick={() => onCall(call)}>{label}<kbd>{shortcut}</kbd></button>)}</div>
      <div className="bid-matrix">{STRAINS.map((strain) => [1, 2, 3, 4, 5, 6, 7].map((level) => {
        const call: Call = { kind: "BID", level, strain };
        return <button className={strain === "H" || strain === "D" ? "red-call" : ""} type="button" key={`${level}${strain}`} disabled={disabled || !legalKeys.has(callKey(call))} onClick={() => onCall(call)}><span>{level}</span>{strain === "NT" ? "NT" : SUIT_LABELS[strain]}</button>;
      }))}</div>
    </section>
  );
}

function CurrentTrick({ game, orientation }: { game: NonNullable<LiveTableProjection["game"]>; orientation: TableOrientation }) {
  return (
    <div className="current-trick" aria-label="Trick saat ini">
      <span className="trick-center">{game.currentTrick.plays.length === 0 ? "Lead" : ""}</span>
      {game.currentTrick.plays.map((play) => <div className={`trick-slot trick-${visualPositionForSeat(orientation, play.seat)}`} key={play.seat}><span>{play.seat}</span><PlayingCard card={play.card} variant="trick" /></div>)}
    </div>
  );
}

function TableSurface({ table, orientation, session, presenceNow, commandDisabled, children }: { table: LiveTableProjection; orientation: TableOrientation; session: TableSession; presenceNow: number; commandDisabled: boolean; children: ReactNode }) {
  const isOwner = table.viewerRole === "OWNER";
  return (
    <div className="table-surface">
      {(Object.entries(orientation) as Array<[VisualPosition, Seat]>).map(([position, seat]) => <PlayerPosition key={seat} table={table} presence={session.tableState.presence} now={presenceNow} seat={seat} position={position} disabled={commandDisabled} turn={table.game?.turn === seat} onCommand={session.sendCommand} {...(isOwner ? { onRemove: (participantId: string) => session.sendCommand("table.remove_participant", { participant_id: participantId }) } : {})} />)}
      {children}
    </div>
  );
}

function WaitingRoom({ table, orientation, session, presenceNow, commandDisabled, shareUrl, copied, onCopy }: {
  table: LiveTableProjection;
  orientation: TableOrientation;
  session: TableSession;
  presenceNow: number;
  commandDisabled: boolean;
  shareUrl: string;
  copied: boolean;
  onCopy: () => void;
}) {
  const viewerAssignment = table.viewerSeat === undefined ? undefined : table.seats[table.viewerSeat];
  const allReady = SEATS.every((seat) => table.seats[seat]?.ready);
  const isOwner = table.viewerRole === "OWNER";
  return (
    <div className="waiting-room">
      <div className="waiting-copy">
        <p className="eyebrow">Ruang tunggu</p><h1>Susun empat kursi.</h1>
        <p>{table.participants.length}/4 pemain sudah masuk. Pilih kursi, tandai siap, lalu pemilik dapat memulai board.</p>
        {session.inviteCode === null ? null : <div className="invite-inline"><span>Kode undangan</span><strong>{session.inviteCode}</strong><button type="button" onClick={onCopy}>{copied ? "Tautan disalin" : "Salin tautan"}</button><span className="sr-only">{shareUrl}</span></div>}
      </div>
      <div className="waiting-table table-surface">
        {(Object.entries(orientation) as Array<[VisualPosition, Seat]>).map(([position, seat]) => <PlayerPosition key={seat} table={table} presence={session.tableState.presence} now={presenceNow} seat={seat} position={position} disabled={commandDisabled} turn={false} onCommand={session.sendCommand} {...(isOwner ? { onRemove: (participantId: string) => session.sendCommand("table.remove_participant", { participant_id: participantId }) } : {})} />)}
        <div className="waiting-center"><strong>{table.locked ? "Meja dikunci" : "Meja terbuka"}</strong><span>{SEATS.filter((seat) => table.seats[seat]?.ready).length}/4 siap</span></div>
      </div>
      <div className="waiting-controls">
        {table.viewerSeat === undefined ? <p>Pilih kursi kosong pada meja.</p> : <><button className="primary-button" type="button" disabled={commandDisabled} onClick={() => session.sendCommand("table.set_ready", { ready: !viewerAssignment?.ready })}>{viewerAssignment?.ready ? "Batalkan siap" : "Saya siap"}</button><button type="button" disabled={commandDisabled} onClick={() => session.sendCommand("table.leave_seat")}>Berdiri dari kursi</button></>}
        {isOwner ? <><button type="button" disabled={commandDisabled} onClick={() => session.sendCommand("table.lock", { locked: !table.locked })}>{table.locked ? "Buka meja" : "Kunci meja"}</button><button className="start-board-button" type="button" disabled={commandDisabled || !allReady} onClick={() => session.sendCommand("table.start_game")}>Mulai board</button><button type="button" disabled={commandDisabled} onClick={() => session.sendCommand("table.finish")}>Akhiri meja</button>{table.viewerSeat !== undefined || table.participants.length < 2 ? null : <button type="button" disabled={commandDisabled} onClick={() => session.sendCommand("table.remove_participant", { participant_id: table.viewerParticipantId })}>Serahkan master & keluar</button>}</> : null}
      </div>
    </div>
  );
}

function BoardResult({ table, session, commandDisabled }: { table: LiveTableProjection; session: TableSession; commandDisabled: boolean }) {
  const result = table.game?.result;
  if (result === undefined) return null;
  return (
    <section className="board-result" aria-labelledby="result-title">
      <p>Board {table.game?.board.number} selesai</p><h2 id="result-title">{result.passedOut ? "Passed out" : contractLabel(result.contract, false)}</h2><strong className="result-outcome">{boardResultLabel(result)}</strong>
      <dl><div><dt>Declarer</dt><dd>{result.contract?.declarer ?? "—"}</dd></div><div><dt>Trick</dt><dd>{result.tricksDeclarer}</dd></div><div><dt>NS</dt><dd>{result.scoreNS > 0 ? "+" : ""}{result.scoreNS}</dd></div><div><dt>EW</dt><dd>{result.scoreNS < 0 ? "+" : ""}{-result.scoreNS}</dd></div></dl>
      {table.viewerRole === "OWNER" && table.state === "BETWEEN_BOARDS" ? <div className="result-actions"><button className="primary-button" type="button" disabled={commandDisabled} onClick={() => session.sendCommand("table.next_board")}>Board berikutnya</button><button type="button" disabled={commandDisabled} onClick={() => session.sendCommand("table.finish")}>Akhiri meja</button></div> : <p className="result-waiting">Menunggu pemilik menentukan board berikutnya.</p>}
    </section>
  );
}

export default function BridgeTable({ expectedTableId }: { expectedTableId: string }) {
  const router = useRouter();
  const session = useTableSession();
  const { openTable, sendCommand } = session;
  const attemptedTableIdRef = useRef<string | null>(null);
  const [copied, setCopied] = useState(false);
  const [presenceNow, setPresenceNow] = useState(0);
  const table = session.tableState.table;
  const game = table?.game;
  const hasPendingCommand = Object.keys(session.tableState.pending).length > 0;
  const commandDisabled = session.connectionState !== "connected" || hasPendingCommand || session.tableState.controllerState !== "current";
  const orientation = useMemo(() => tableOrientation(table?.viewerSeat), [table?.viewerSeat]);
  const legalPlay = table === null ? null : playableHand(table);
  const viewerTurn = game?.turn === table?.viewerSeat;
  const hasPresenceCountdown = Object.values(session.tableState.presence).some((presence) => !presence.online && presence.expiresAt !== undefined);

  useEffect(() => {
    if (!hasPresenceCountdown) return;
    const initialTick = window.setTimeout(() => setPresenceNow(Date.now()), 0);
    const interval = window.setInterval(() => setPresenceNow(Date.now()), 1000);
    return () => {
      window.clearTimeout(initialTick);
      window.clearInterval(interval);
    };
  }, [hasPresenceCountdown]);

  useEffect(() => {
    if (session.initializing) return;
    if (session.nickname === null) {
      router.replace("/");
      return;
    }
    if (table?.tableId !== expectedTableId && attemptedTableIdRef.current !== expectedTableId) {
      attemptedTableIdRef.current = expectedTableId;
      void openTable(expectedTableId);
    }
  }, [expectedTableId, openTable, router, session.initializing, session.nickname, table?.tableId]);

  useEffect(() => {
    function handleAuctionKeyboard(event: KeyboardEvent) {
      if (event.target instanceof HTMLInputElement || event.target instanceof HTMLSelectElement || event.target instanceof HTMLTextAreaElement || game?.phase !== "AUCTION" || !viewerTurn || commandDisabled) return;
      const shortcutCalls: Record<string, Call> = { p: { kind: "PASS" }, x: { kind: "DOUBLE" }, r: { kind: "REDOUBLE" } };
      const call = shortcutCalls[event.key.toLowerCase()];
      if (call !== undefined && game.legalCalls?.some((legalCall) => callKey(legalCall) === callKey(call))) {
        event.preventDefault();
        sendCommand("game.make_call", { call });
      }
    }
    window.addEventListener("keydown", handleAuctionKeyboard);
    return () => window.removeEventListener("keydown", handleAuctionKeyboard);
  }, [commandDisabled, game, sendCommand, viewerTurn]);

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

  if (session.initializing || session.nickname === null || table?.tableId !== expectedTableId) {
    return <main className="table-route-state">{session.tableState.issue === null ? <p role="status">Menyiapkan meja…</p> : <IssueNotice issue={session.tableState.issue} onDismiss={session.dismissIssue} onAction={(action) => {
      if (action === "backToLobby" || action === "editInvite") router.replace("/lobby");
      else if (action === "signInAgain") router.replace("/");
      else if (action === "retry") {
        attemptedTableIdRef.current = null;
        void openTable(expectedTableId);
      }
    }} />}</main>;
  }

  if (table.state === "WAITING") {
    return (
      <main className="table-client waiting-client">
        <header className="table-status-bar"><Link className="table-wordmark" href="/lobby" aria-label="Kembali ke lobby">BridgeYok</Link><span>Meja tunggu</span><div className="connection-status" data-state={session.connectionState}><span className="status-mark" />{CONNECTION_LABELS[session.connectionState]}</div></header>
        {session.tableState.issue === null ? null : <IssueNotice compact issue={session.tableState.issue} onDismiss={session.dismissIssue} onAction={(action) => {
          if (action === "retry") session.reconnect();
          else if (action === "resync") session.resync();
        }} />}
        <WaitingRoom table={table} orientation={orientation} session={session} presenceNow={presenceNow} commandDisabled={commandDisabled} shareUrl={shareUrl} copied={copied} onCopy={() => void copyInvite()} />
        {table.viewerRole === "PARTICIPANT" ? <button className="leave-table-button" type="button" disabled={session.busy} onClick={() => void returnToLobby()}>Tinggalkan meja</button> : null}
      </main>
    );
  }

  const dummySeat = game?.auction.contract === undefined ? undefined : oppositeSeat(game.auction.contract.declarer);
  const dummyPosition = dummySeat === undefined ? undefined : visualPositionForSeat(orientation, dummySeat);
  return (
    <main className="table-client active-table-client">
      <header className="table-status-bar">
        <Link className="table-wordmark" href="/lobby" aria-label="Kembali ke lobby">BY</Link>
        <dl className="table-facts"><div><dt>Board</dt><dd>{game?.board.number ?? table.boardNumber}</dd></div><div><dt>Dealer</dt><dd>{game?.board.dealer ?? "—"}</dd></div><div><dt>Vul</dt><dd>{game === undefined ? "—" : VULNERABILITY_LABELS[game.board.vulnerability]}</dd></div><div><dt>Kontrak</dt><dd>{game === undefined ? "—" : contractLabel(game.auction.contract, false)}</dd></div><div><dt>Trick</dt><dd>{game === undefined ? "—" : `${game.tricksNS}–${game.tricksEW}`}</dd></div></dl>
        <div className="status-actions"><div className="connection-status" data-state={session.connectionState} role="status"><span className="status-mark" />{CONNECTION_LABELS[session.connectionState]}</div><details className="table-menu"><summary aria-label="Buka menu meja">•••</summary><div><Link href="/lobby">Lobby</Link>{session.inviteCode === null ? null : <button type="button" onClick={() => void copyInvite()}>{copied ? "Tautan disalin" : "Salin undangan"}</button>}<span>Revision {table.revision}</span></div></details></div>
      </header>

      <div className="table-feedback" aria-live="polite">
        {session.tableState.issue === null ? null : <IssueNotice compact issue={session.tableState.issue} onDismiss={session.dismissIssue} onAction={(action) => {
          if (action === "resync") session.resync();
          else if (action === "takeover") session.sendCommand("table.takeover");
          else if (action === "retry") session.reconnect();
          else if (action === "backToLobby") router.push("/lobby");
          else if (action === "signInAgain") void session.logout().then(() => router.replace("/"));
        }} />}
        {session.tableState.notice === null ? null : <div className="success-notice" role="status"><span>{session.tableState.notice}</span><button type="button" onClick={session.dismissNotice} aria-label="Tutup konfirmasi">×</button></div>}
      </div>

      <TableSurface table={table} orientation={orientation} session={session} presenceNow={presenceNow} commandDisabled={commandDisabled}>
        {game?.phase === "AUCTION" ? <div className="auction-workspace"><AuctionTable game={game} /><BiddingBox legalCalls={game.legalCalls ?? []} disabled={commandDisabled || !viewerTurn} onCall={(call) => session.sendCommand("game.make_call", { call })} /></div> : null}
        {game !== undefined && (game.phase === "OPENING_LEAD" || game.phase === "PLAY") ? <>{game.dummyHand === undefined || dummyPosition === undefined ? null : <BridgeHand className={`dummy-hand dummy-${dummyPosition}`} title={`Dummy · ${dummySeat}`} variant="dummy" cards={game.dummyHand} playableCards={legalPlay?.source === "dummy" ? legalPlay.hand : []} disabled={commandDisabled} onPlay={(card) => session.sendCommand("game.play_card", { card })} />}<CurrentTrick game={game} orientation={orientation} /></> : null}
        {game?.result === undefined && table.state !== "FINISHED" ? null : table.state === "FINISHED" ? <section className="board-result finished-result"><p>Meja selesai</p><h2>Terima kasih sudah bermain.</h2><button className="primary-button" type="button" disabled={session.busy} onClick={() => void returnToLobby()}>Kembali ke lobby</button></section> : <BoardResult table={table} session={session} commandDisabled={commandDisabled} />}
      </TableSurface>

      {game === undefined ? null : <BridgeHand className="own-hand" title={`Kartu kamu · ${table.viewerSeat ?? ""}${legalPlay?.source === "own" ? " · giliranmu" : ""}`} cards={game.ownHand} playableCards={legalPlay?.source === "own" ? legalPlay.hand : []} disabled={commandDisabled} {...(game.phase === "OPENING_LEAD" || game.phase === "PLAY" ? { onPlay: (card: Card) => session.sendCommand("game.play_card", { card }) } : {})} />}
    </main>
  );
}
