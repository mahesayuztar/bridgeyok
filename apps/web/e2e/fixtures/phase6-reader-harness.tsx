import { useState } from "react";
import { createRoot } from "react-dom/client";
import { AuctionTable } from "../../app/table/auction-controls";
import { TrickIndicator } from "../../app/table/trick-indicator";
import type { LiveTableProjection, Seat, Trick } from "../../app/table-state";

const seats: Seat[] = ["E", "S", "W", "N"];
const calls = [
  { kind: "BID" as const, level: 1, strain: "C" as const },
  { kind: "PASS" as const },
  { kind: "BID" as const, level: 1, strain: "D" as const },
  { kind: "DOUBLE" as const },
  { kind: "REDOUBLE" as const },
  { kind: "PASS" as const },
  { kind: "BID" as const, level: 1, strain: "H" as const },
  { kind: "PASS" as const },
  { kind: "BID" as const, level: 1, strain: "S" as const },
  { kind: "PASS" as const },
  { kind: "BID" as const, level: 1, strain: "NT" as const },
  { kind: "PASS" as const },
  { kind: "BID" as const, level: 2, strain: "C" as const },
  { kind: "PASS" as const },
  { kind: "BID" as const, level: 2, strain: "D" as const },
  { kind: "PASS" as const },
  { kind: "BID" as const, level: 2, strain: "H" as const },
  { kind: "PASS" as const },
  { kind: "BID" as const, level: 2, strain: "S" as const },
  { kind: "PASS" as const },
  { kind: "BID" as const, level: 2, strain: "NT" as const },
  { kind: "PASS" as const },
  { kind: "BID" as const, level: 3, strain: "C" as const },
  { kind: "PASS" as const },
  { kind: "PASS" as const },
  { kind: "PASS" as const },
].map((call, _index) => ({ seat: seats[_index % seats.length]!, call }));

const tricks: Trick[] = [
  {
    leader: "E",
    winner: "N",
    plays: [
      { seat: "E", card: { suit: "S", rank: "Q" } },
      { seat: "S", card: { suit: "S", rank: "9" } },
      { seat: "W", card: { suit: "S", rank: "6" } },
      { seat: "N", card: { suit: "S", rank: "A" } },
    ],
  },
  {
    leader: "N",
    winner: "E",
    plays: [
      { seat: "N", card: { suit: "H", rank: "K" } },
      { seat: "E", card: { suit: "H", rank: "A" } },
      { seat: "S", card: { suit: "H", rank: "7" } },
      { seat: "W", card: { suit: "H", rank: "3" } },
    ],
  },
  {
    leader: "E",
    winner: "W",
    plays: [
      { seat: "E", card: { suit: "D", rank: "6" } },
      { seat: "S", card: { suit: "D", rank: "A" } },
      { seat: "W", card: { suit: "D", rank: "K" } },
      { seat: "N", card: { suit: "D", rank: "9" } },
    ],
  },
  {
    leader: "W",
    winner: "S",
    plays: [
      { seat: "W", card: { suit: "C", rank: "Q" } },
      { seat: "N", card: { suit: "C", rank: "2" } },
      { seat: "E", card: { suit: "C", rank: "K" } },
      { seat: "S", card: { suit: "C", rank: "A" } },
    ],
  },
];

const initialTable: LiveTableProjection = {
  tableId: "phase6-reader-table",
  boardId: "20000000-0000-4000-8000-000000000001",
  boardNumber: 1,
  state: "ACTIVE",
  locked: true,
  revision: 30,
  lastSeq: 30,
  viewerRole: "PARTICIPANT",
  viewerSeat: "W",
  viewerParticipantId: "west",
  participants: [
    { id: "north", nickname: "North", role: "OWNER", isBot: false },
    { id: "east", nickname: "East", role: "PARTICIPANT", isBot: false },
    { id: "south", nickname: "South", role: "PARTICIPANT", isBot: false },
    { id: "west", nickname: "West", role: "PARTICIPANT", isBot: false },
  ],
  seats: {
    N: { participantId: "north", ready: true, controllerEpoch: 1 },
    E: { participantId: "east", ready: true, controllerEpoch: 1 },
    S: { participantId: "south", ready: true, controllerEpoch: 1 },
    W: { participantId: "west", ready: true, controllerEpoch: 1 },
  },
  scoreSheet: [],
  pairScoreTotals: [],
  canRequestUndo: false,
  game: {
    rulesetVersion: "bridgeyok_duplicate_v1",
    board: { number: 1, dealer: "E", vulnerability: "EW" },
    phase: "PLAY",
    auction: {
      dealer: "E",
      complete: true,
      passedOut: false,
      calls,
      contract: {
        level: 3,
        strain: "C",
        doubling: "UNDOUBLED",
        declarer: "E",
      },
    },
    turn: "S",
    dummyRevealed: true,
    currentTrick: { leader: "N", plays: [] },
    completedTrickCount: 3,
    completedTricks: tricks.slice(0, 3),
    tricksNS: 1,
    tricksEW: 2,
    ownHand: [],
    dummyHand: [],
  },
};

function ReaderHarness() {
  const [table, setTable] = useState(initialTable);

  function updateTricks(completedTrickCount: number) {
    setTable((current) => ({
      ...current,
      revision: current.revision + 1,
      game: {
        ...current.game!,
        completedTrickCount,
        completedTricks: tricks.slice(0, completedTrickCount),
      },
    }));
  }

  return (
    <main className="active-table-client">
      <div hidden>
        <button id="remote-call" type="button" onClick={() => setTable((current) => ({
          ...current,
          revision: current.revision + 1,
          game: {
            ...current.game!,
            auction: {
              ...current.game!.auction,
              calls: [...current.game!.auction.calls, { seat: "E", call: { kind: "PASS" } }],
            },
          },
        }))}>Remote call</button>
        <button id="remote-trick" type="button" onClick={() => updateTricks(4)}>Remote trick</button>
        <button id="undo-to-one" type="button" onClick={() => updateTricks(1)}>Undo to one</button>
        <button id="next-board" type="button" onClick={() => setTable((current) => ({
          ...current,
          boardId: "20000000-0000-4000-8000-000000000002",
          boardNumber: 2,
          revision: current.revision + 1,
          game: {
            ...current.game!,
            board: { ...current.game!.board, number: 2 },
            completedTrickCount: 1,
            completedTricks: tricks.slice(0, 1),
          },
        }))}>Next board</button>
        <button id="defender-view" type="button" onClick={() => setTable((current) => ({
          ...current,
          viewerSeat: "N",
          viewerParticipantId: "north",
          revision: current.revision + 1,
          game: {
            ...current.game!,
            completedTrickCount: 4,
            completedTricks: tricks.slice(3),
          },
        }))}>Defender view</button>
      </div>
      <button
        type="button"
        popoverTarget="reader-auction"
        aria-label="Buka riwayat auction"
      >
        Auction
      </button>
      <section
        className="auction-history-popover"
        id="reader-auction"
        popover="auto"
        role="dialog"
        aria-labelledby="reader-auction-title"
      >
        <header>
          <h2 id="reader-auction-title">Auction</h2>
          <button
            type="button"
            popoverTarget="reader-auction"
            popoverTargetAction="hide"
            aria-label="Tutup riwayat auction"
          >
            ×
          </button>
        </header>
        <AuctionTable game={table.game!} followLatest={false} />
      </section>
      <TrickIndicator key={table.boardId ?? table.boardNumber} table={table} />
    </main>
  );
}

createRoot(document.getElementById("root")!).render(<ReaderHarness />);
