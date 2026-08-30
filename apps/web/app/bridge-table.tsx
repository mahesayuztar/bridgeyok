"use client";

import { useRouter } from "next/navigation";
import { useEffect, useMemo, useState, type FormEvent } from "react";
import { playableHand, type Call, type Card, type LiveTableProjection, type Seat, type Suit } from "./table-state";
import { useTableSession } from "./use-table-session";

const SEATS: Seat[] = ["N", "E", "S", "W"];
const SUITS: Suit[] = ["S", "H", "D", "C"];
const SUIT_LABELS: Record<Suit, string> = { S: "♠", H: "♥", D: "♦", C: "♣" };
const SEAT_LABELS: Record<Seat, string> = { N: "Utara", E: "Timur", S: "Selatan", W: "Barat" };
const CONNECTION_LABELS = {
  idle: "Belum terhubung",
  connecting: "Menghubungkan",
  syncing: "Menyelaraskan meja",
  connected: "Terhubung",
  degraded: "Koneksi terganggu",
  offline: "Perangkat offline"
};

function participantName(table: LiveTableProjection, participantId: string | undefined) {
  if (participantId === undefined) {
    return null;
  }
  return table.participants.find((participant) => participant.id === participantId)?.nickname ?? "Pemain";
}

function callLabel(call: Call) {
  if (call.kind === "PASS") {
    return "Pass";
  }
  if (call.kind === "DOUBLE") {
    return "X";
  }
  if (call.kind === "REDOUBLE") {
    return "XX";
  }
  return `${call.level}${call.strain === "NT" ? "NT" : SUIT_LABELS[call.strain ?? "C"]}`;
}

function contractLabel(contract: NonNullable<LiveTableProjection["game"]>["auction"]["contract"]) {
  if (contract === undefined) {
    return "Belum ada kontrak";
  }
  const doubling = contract.doubling === "DOUBLED" ? " X" : contract.doubling === "REDOUBLED" ? " XX" : "";
  return `${contract.level}${contract.strain === "NT" ? "NT" : SUIT_LABELS[contract.strain]}${doubling} oleh ${contract.declarer}`;
}

function Hand({
  cards,
  title,
  playableCards = [],
  disabled,
  onPlay
}: {
  cards: Card[];
  title: string;
  playableCards?: Card[];
  disabled: boolean;
  onPlay: (card: Card) => void;
}) {
  const playableKeys = new Set(playableCards.map((card) => `${card.suit}${card.rank}`));
  return (
    <section className="card-hand" aria-label={title}>
      <h4>{title}</h4>
      <div className="suit-groups">
        {SUITS.map((suit) => {
          const suitedCards = cards.filter((card) => card.suit === suit);
          return (
            <div className={`suit-row suit-${suit.toLowerCase()}`} key={suit}>
              <span className="suit-symbol" aria-label={suit}>{SUIT_LABELS[suit]}</span>
              <div className="card-ranks">
                {suitedCards.length === 0 ? <span className="empty-suit">—</span> : null}
                {suitedCards.map((card) => {
                  const cardKey = `${card.suit}${card.rank}`;
                  const canPlay = playableKeys.has(cardKey);
                  return (
                    <button
                      className="playing-card"
                      type="button"
                      key={cardKey}
                      disabled={disabled || !canPlay}
                      onClick={() => onPlay(card)}
                      aria-label={`Mainkan ${card.rank} ${SUIT_LABELS[card.suit]}`}
                    >
                      {card.rank}
                    </button>
                  );
                })}
              </div>
            </div>
          );
        })}
      </div>
    </section>
  );
}

function SeatPosition({ table, seat, disabled, onCommand }: {
  table: LiveTableProjection;
  seat: Seat;
  disabled: boolean;
  onCommand: (name: "table.take_seat", payload: Record<string, unknown>) => void;
}) {
  const assignment = table.seats[seat];
  const isViewer = table.viewerParticipantId === assignment?.participantId;
  const name = participantName(table, assignment?.participantId);
  return (
    <div className={`seat-position seat-${seat.toLowerCase()}`} data-occupied={assignment !== undefined}>
      <span className="seat-direction">{seat}</span>
      {assignment === undefined ? (
        <button type="button" disabled={disabled || table.state !== "WAITING"} onClick={() => onCommand("table.take_seat", { seat })}>
          Duduk di {SEAT_LABELS[seat]}
        </button>
      ) : (
        <div>
          <strong>{name}{isViewer ? " (kamu)" : ""}</strong>
          <span>{assignment.ready ? "Siap" : "Belum siap"}</span>
        </div>
      )}
    </div>
  );
}

export default function BridgeTable({ initialInviteCode = "", expectedTableId }: { initialInviteCode?: string; expectedTableId?: string }) {
  const router = useRouter();
  const session = useTableSession();
  const [identityName, setIdentityName] = useState("");
  const [joinCode, setJoinCode] = useState(initialInviteCode.toUpperCase());
  const [bidLevel, setBidLevel] = useState("1");
  const [bidStrain, setBidStrain] = useState<"C" | "D" | "H" | "S" | "NT">("C");
  const table = session.tableState.table;
  const hasPendingCommand = Object.keys(session.tableState.pending).length > 0;
  const commandDisabled = session.connectionState !== "connected" || hasPendingCommand;
  const game = table?.game;
  const legalPlay = table === null ? null : playableHand(table);

  useEffect(() => {
    if (session.initializing) {
      return;
    }
    if (session.nickname === null) {
      router.replace("/");
      return;
    }
    if (expectedTableId !== undefined && table?.tableId !== expectedTableId && !session.busy) {
      void session.openTable(expectedTableId);
    }
  }, [expectedTableId, router, session, table?.tableId]);

  useEffect(() => {
    function handleAuctionKeyboard(event: KeyboardEvent) {
      if (event.target instanceof HTMLInputElement || event.target instanceof HTMLSelectElement || event.target instanceof HTMLTextAreaElement) {
        return;
      }
      if (game?.phase !== "AUCTION" || game.turn !== table?.viewerSeat || commandDisabled) {
        return;
      }
      if (event.key.toLowerCase() === "p") {
        session.sendCommand("game.make_call", { call: { kind: "PASS" } });
      } else if (event.key.toLowerCase() === "x") {
        session.sendCommand("game.make_call", { call: { kind: "DOUBLE" } });
      } else if (event.key.toLowerCase() === "r") {
        session.sendCommand("game.make_call", { call: { kind: "REDOUBLE" } });
      }
    }
    window.addEventListener("keydown", handleAuctionKeyboard);
    return () => window.removeEventListener("keydown", handleAuctionKeyboard);
  }, [commandDisabled, game?.phase, game?.turn, session, table?.viewerSeat]);

  const shareUrl = useMemo(() => {
    if (session.inviteCode === null || typeof window === "undefined") {
      return "";
    }
    const url = new URL(window.location.href);
    url.search = "";
    url.searchParams.set("invite", session.inviteCode);
    return url.toString();
  }, [session.inviteCode]);

  function submitIdentity(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (identityName.trim().length > 0) {
      void session.createIdentity(identityName);
    }
  }

  function submitJoin(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (joinCode.trim().length > 0) {
      void session.joinTable(joinCode);
    }
  }

  if (session.initializing) {
    return <div className="table-loading" role="status">Memulihkan meja terakhir…</div>;
  }

  if (session.nickname === null) {
    return (
      <section className="identity-entry" aria-labelledby="identity-title">
        <div>
          <p className="eyebrow">Mulai tanpa akun</p>
          <h2 id="identity-title">Namamu cukup untuk duduk bermain.</h2>
          <p>Sesi tamu tersimpan di perangkat ini. Tidak ada kata sandi dan tidak ada pembayaran.</p>
        </div>
        <form onSubmit={submitIdentity}>
          <label htmlFor="nickname">Nama di meja</label>
          <input id="nickname" name="nickname" autoComplete="nickname" maxLength={128} required value={identityName} onChange={(event) => setIdentityName(event.target.value)} />
          <button className="primary-button" type="submit" disabled={session.busy}>Masuk sebagai tamu</button>
        </form>
        {session.tableState.message === null ? null : <p className="form-message" role="alert">{session.tableState.message}</p>}
      </section>
    );
  }

  if (table === null) {
    return (
      <section className="lobby" aria-labelledby="lobby-title">
        <div className="lobby-heading">
          <div>
            <p className="eyebrow">Halo, {session.nickname}</p>
            <h2 id="lobby-title">Buka meja atau masuk dengan kode.</h2>
          </div>
          <button className="quiet-button" type="button" disabled={session.busy} onClick={() => void session.logout()}>Ganti nama</button>
        </div>
        <div className="lobby-actions">
          <div className="lobby-option">
            <span className="option-number">01</span>
            <h3>Meja baru</h3>
            <p>Jadilah pemilik meja dan bagikan undangan kepada tiga teman.</p>
            <button className="primary-button" type="button" disabled={session.busy} onClick={() => void session.createTable()}>Buat meja</button>
          </div>
          <form className="lobby-option" onSubmit={submitJoin}>
            <span className="option-number">02</span>
            <h3>Masuk meja</h3>
            <label htmlFor="invite-code">Kode undangan</label>
            <input id="invite-code" name="inviteCode" autoCapitalize="characters" autoComplete="off" spellCheck={false} required value={joinCode} onChange={(event) => setJoinCode(event.target.value.toUpperCase())} />
            <button className="secondary-button" type="submit" disabled={session.busy}>Masuk</button>
          </form>
        </div>
        {session.tableState.message === null ? null : <p className="form-message" role="alert">{session.tableState.message}</p>}
      </section>
    );
  }

  const viewerAssignment = table.viewerSeat === undefined ? undefined : table.seats[table.viewerSeat];
  const allSeatsFilled = SEATS.every((seat) => table.seats[seat] !== undefined);
  const allReady = allSeatsFilled && SEATS.every((seat) => table.seats[seat]?.ready);
  const isOwner = table.viewerRole === "OWNER";
  const viewerTurn = game?.turn === table.viewerSeat;

  return (
    <section className="table-experience" aria-labelledby="table-title">
      <header className="table-toolbar">
        <div>
          <p className="eyebrow">Meja {table.state.toLowerCase().replaceAll("_", " ")}</p>
          <h2 id="table-title">{game === undefined ? "Menunggu empat pemain" : `Board ${game.board.number}`}</h2>
        </div>
        <div className="connection-status" data-state={session.connectionState} role="status" aria-live="polite">
          <span className="status-mark" aria-hidden="true" />
          <span>{CONNECTION_LABELS[session.connectionState]}</span>
          {session.connectionState === "degraded" || session.connectionState === "offline" ? (
            <button type="button" onClick={session.reconnect}>Coba lagi</button>
          ) : null}
        </div>
      </header>

      {session.tableState.message === null ? null : <p className="table-message" role="alert">{session.tableState.message}</p>}

      <div className="table-layout">
        <aside className="table-sidebar">
          {session.inviteCode === null ? null : (
            <div className="invite-block">
              <span>Kode undangan</span>
              <strong>{session.inviteCode}</strong>
              <button type="button" onClick={() => void navigator.clipboard.writeText(shareUrl)}>Salin tautan</button>
            </div>
          )}
          <div className="participant-list">
            <h3>Pemain</h3>
            <ol>
              {table.participants.map((participant) => (
                <li key={participant.id}>
                  <span>{participant.nickname}{participant.id === table.viewerParticipantId ? " (kamu)" : ""}</span>
                  <small>{participant.role === "OWNER" ? "Pemilik" : "Tamu"}</small>
                  {isOwner && participant.id !== table.viewerParticipantId && table.state === "WAITING" ? (
                    <button type="button" disabled={commandDisabled} onClick={() => session.sendCommand("table.remove_participant", { participant_id: participant.id })}>Keluarkan</button>
                  ) : null}
                </li>
              ))}
            </ol>
          </div>
          <div className="table-secondary-actions">
            {viewerAssignment === undefined ? null : (
              <button type="button" disabled={commandDisabled} onClick={() => session.sendCommand("table.takeover")}>Ambil alih kendali</button>
            )}
            {table.state === "FINISHED" ? (
              <button type="button" disabled={session.busy} onClick={() => void session.leaveTable()}>Kembali ke lobby</button>
            ) : table.state === "WAITING" && !isOwner ? (
              <button type="button" disabled={session.busy} onClick={() => void session.leaveTable()}>Tinggalkan meja</button>
            ) : null}
          </div>
        </aside>

        <div className="bridge-board">
          <div className="seat-map" aria-label="Susunan kursi meja">
            {SEATS.map((seat) => <SeatPosition key={seat} table={table} seat={seat} disabled={commandDisabled} onCommand={session.sendCommand} />)}
            <div className="table-center">
              {game === undefined ? (
                <>
                  <span>{table.participants.length}/4 pemain</span>
                  <strong>{table.locked ? "Meja dikunci" : "Meja terbuka"}</strong>
                </>
              ) : (
                <>
                  <span>{game.phase.replaceAll("_", " ")}</span>
                  <strong>NS {game.tricksNS} · {game.tricksEW} EW</strong>
                  {game.turn === undefined ? null : <small>Giliran {game.turn}</small>}
                </>
              )}
            </div>
          </div>

          {table.state === "WAITING" ? (
            <div className="waiting-controls">
              {table.viewerSeat === undefined ? <p>Pilih kursi kosong untuk bergabung dalam permainan.</p> : (
                <>
                  <button type="button" disabled={commandDisabled} onClick={() => session.sendCommand("table.set_ready", { ready: !viewerAssignment?.ready })}>
                    {viewerAssignment?.ready ? "Batalkan siap" : "Saya siap"}
                  </button>
                  <button type="button" disabled={commandDisabled} onClick={() => session.sendCommand("table.leave_seat")}>Berdiri dari kursi</button>
                </>
              )}
              {isOwner ? (
                <>
                  <button type="button" disabled={commandDisabled} onClick={() => session.sendCommand("table.lock", { locked: !table.locked })}>{table.locked ? "Buka meja" : "Kunci meja"}</button>
                  <button className="primary-button" type="button" disabled={commandDisabled || !allReady} onClick={() => session.sendCommand("table.start_game")}>Mulai board</button>
                  <button type="button" disabled={commandDisabled} onClick={() => session.sendCommand("table.finish")}>Akhiri meja</button>
                </>
              ) : null}
            </div>
          ) : null}

          {game?.phase === "AUCTION" ? (
            <div className="auction-panel">
              <div>
                <p className="eyebrow">Lelang · dealer {game.auction.dealer}</p>
                <div className="auction-history" aria-label="Riwayat lelang">
                  {game.auction?.calls?.length == 0 ? <span>Belum ada call</span> : game.auction.calls.map((record, _index) => (
                    <span key={`${record.seat}-${_index}`}><small>{record.seat}</small>{callLabel(record.call)}</span>
                  ))}
                </div>
              </div>
              <div className="auction-actions" aria-label="Pilihan call">
                <button type="button" disabled={commandDisabled || !viewerTurn} onClick={() => session.sendCommand("game.make_call", { call: { kind: "PASS" } })}>Pass <kbd>P</kbd></button>
                <button type="button" disabled={commandDisabled || !viewerTurn} onClick={() => session.sendCommand("game.make_call", { call: { kind: "DOUBLE" } })}>Double <kbd>X</kbd></button>
                <button type="button" disabled={commandDisabled || !viewerTurn} onClick={() => session.sendCommand("game.make_call", { call: { kind: "REDOUBLE" } })}>Redouble <kbd>R</kbd></button>
                <div className="bid-builder">
                  <label htmlFor="bid-level">Level</label>
                  <select id="bid-level" value={bidLevel} onChange={(event) => setBidLevel(event.target.value)}>{[1, 2, 3, 4, 5, 6, 7].map((level) => <option key={level}>{level}</option>)}</select>
                  <label htmlFor="bid-strain">Strain</label>
                  <select id="bid-strain" value={bidStrain} onChange={(event) => setBidStrain(event.target.value as typeof bidStrain)}>
                    <option value="C">♣</option><option value="D">♦</option><option value="H">♥</option><option value="S">♠</option><option value="NT">NT</option>
                  </select>
                  <button className="primary-button" type="button" disabled={commandDisabled || !viewerTurn} onClick={() => session.sendCommand("game.make_call", { call: { kind: "BID", level: Number(bidLevel), strain: bidStrain } })}>Bid</button>
                </div>
              </div>
            </div>
          ) : null}

          {game !== undefined && ["OPENING_LEAD", "PLAY"].includes(game.phase) ? (
            <div className="play-panel">
              <div className="board-facts">
                <span>Dealer {game.board.dealer}</span>
                <span>Vulnerable {game.board.vulnerability}</span>
                <strong>{contractLabel(game.auction.contract)}</strong>
              </div>
              {game.dummyHand === undefined ? null : <Hand title="Dummy" cards={game.dummyHand} playableCards={legalPlay?.source === "dummy" ? legalPlay.hand : []} disabled={commandDisabled} onPlay={(card) => session.sendCommand("game.play_card", { card })} />}
              <div className="current-trick" aria-label="Trick saat ini">
                <h3>Trick saat ini</h3>
                <div>{game?.currentTrick?.plays?.length == 0 ? <span>Menunggu lead</span> : game.currentTrick.plays.map((play) => <span className={`trick-card suit-${play.card.suit.toLowerCase()}`} key={play.seat}><small>{play.seat}</small>{play.card.rank}{SUIT_LABELS[play.card.suit]}</span>)}</div>
              </div>
              <Hand title={`Kartu kamu${legalPlay?.source === "own" ? " · giliranmu" : ""}`} cards={game.ownHand} playableCards={legalPlay?.source === "own" ? legalPlay.hand : []} disabled={commandDisabled} onPlay={(card) => session.sendCommand("game.play_card", { card })} />
            </div>
          ) : null}

          {game?.result !== undefined ? (
            <div className="result-panel">
              <p className="eyebrow">Board selesai</p>
              <h3>{game.result.passedOut ? "Passed out" : contractLabel(game.result.contract)}</h3>
              <div className="score-line"><span>Skor NS</span><strong>{game.result.scoreNS > 0 ? "+" : ""}{game.result.scoreNS}</strong></div>
              <p>Trick NS {game.result.tricksNS} · EW {game.result.tricksEW}</p>
              {isOwner && table.state === "BETWEEN_BOARDS" ? (
                <div className="result-actions">
                  <button className="primary-button" type="button" disabled={commandDisabled} onClick={() => session.sendCommand("table.next_board")}>Board berikutnya</button>
                  <button type="button" disabled={commandDisabled} onClick={() => session.sendCommand("table.finish")}>Akhiri meja</button>
                </div>
              ) : null}
            </div>
          ) : null}

          {table.state === "FINISHED" ? (
            <div className="finished-panel"><p className="eyebrow">Meja selesai</p><h3>Terima kasih sudah bermain.</h3><p>Pemilik telah menutup rangkaian board ini.</p></div>
          ) : null}
        </div>
      </div>
    </section>
  );
}
