import { useState } from "react";
import { createRoot } from "react-dom/client";
import { usePositionAnalysis } from "../../app/use-position-analysis";
import { BridgeHand, PlayingCard } from "../../app/table/playing-card";
import { ScoreSheet } from "../../app/table/score-sheet";
import { BoardReplayModal } from "../../app/table/board-replay-modal";
import type { BoardReplay } from "../../app/board-replay";
import type { LiveTableProjection, Seat, Suit } from "../../app/table-state";
import type { PositionAnalysis } from "../../app/use-position-analysis";

const ranks = ["A", "K", "Q", "J", "T", "9", "8", "7", "6", "5", "4", "3", "2"] as const;
export const fullDeal = Object.fromEntries(["north", "east", "south", "west"].map((hand, _seatIndex) => [hand, (["S", "H", "D", "C"] as Suit[]).flatMap((suit, _suitIndex) => ranks.filter((_, _rankIndex) => (_rankIndex + _suitIndex) % 4 === _seatIndex).map((rank) => ({ suit, rank })))])) as BoardReplay["fullDeal"];

const entry = { boardId: "board-one", boardNumber: 1, lineup: { seats: {} } } as LiveTableProjection["scoreSheet"][number];
const replay: BoardReplay = {
  boardId: entry.boardId, fullDeal,
  game: {
    rulesetVersion: "bridgeyok_duplicate_v1", dummyRevealed: true, completedTrickCount: 1, tricksNS: 1, tricksEW: 12, ownHand: [],
    phase: "BOARD_SCORED", board: { number: 1, dealer: "N", vulnerability: "NONE" },
    auction: { dealer: "N", complete: true, passedOut: false, calls: [{seat: "N", call: {kind: "BID", level: 1, strain: "NT"}}], contract: { level: 1, strain: "NT", declarer: "N", doubling: "UNDOUBLED" } },
    completedTricks: [{ leader: "E", winner: "N", plays: (["E", "S", "W", "N"] as Seat[]).map((seat, _index) => ({ seat, card: { suit: "S", rank: (["K", "Q", "J", "A"] as const)[_index]! } })) }],
    currentTrick: { plays: [] }, result: { passedOut: false, vulnerability: "NONE", tricksNS: 1, tricksEW: 12, tricksDeclarer: 1, scoreNS: -300 }, claimed: true,
  },
};
const pending = new Map<string, (result: PositionAnalysis) => void>();
const loadBoardReplay = async () => replay;
const loadPositionAnalysis = (_boardId: string, _key: string, step: number | undefined) => new Promise<PositionAnalysis>((resolve) => { pending.set(`${_boardId}:${step ?? _key}`, resolve); });

export function resolvePosition(step: number, tricks: number) {
  const turn = (["E", "S", "W", "N", "N"] as Seat[])[step]!;
  const hand = { N: fullDeal.north, E: fullDeal.east, S: fullDeal.south, W: fullDeal.west }[turn];
  pending.get(`${entry.boardId}:${step}`)?.({ boardId: entry.boardId, positionKey: String(step), turn, cards: hand.map((card) => ({ card, tricks })) });
}

export function resolveLivePosition(step: number, tricks: number) {
  pending.get(`live-board:${step}`)?.({ boardId: "live-board", positionKey: String(step), turn: "N", cards: fullDeal.north.slice(step).map((card) => ({ card, tricks })) });
}

function Harness() {
  const [liveStep, setLiveStep] = useState(0);
  const liveAnalysis = usePositionAnalysis(loadPositionAnalysis, "live-board", String(liveStep), true);
  const [open, setOpen] = useState(true);
  const [bids, setBids] = useState(0);
  return <main className="table-client active-table-client"><BridgeHand className="own-hand" title="Live hand" cards={fullDeal.north.slice(liveStep)} playableCards={fullDeal.north.slice(liveStep)} contractStrain="NT" predictions={liveAnalysis.result?.cards} analysisPending={liveAnalysis.pending} onPlay={() => setLiveStep((step) => step + 1)} /><button style={{ position: "fixed", bottom: 130, left: 20 }} onClick={() => setBids((count) => count + 1)}>Bid on table {bids}</button>{open ? <BoardReplayModal table={{ boardId: entry.boardId } as LiveTableProjection} entry={entry} loadBoardReplay={loadBoardReplay} loadPositionAnalysis={loadPositionAnalysis} onClose={() => setOpen(false)} /> : null}</main>;
}
const root = createRoot(document.getElementById("root")!);
root.render(<Harness />);


export function renderHistory() {
  const container = document.createElement("div");
  document.body.append(container);
  const table = { tableId: "history-table", scoreSheet: [{ ...entry, result: replay.game.result }] } as LiveTableProjection;
  createRoot(container).render(<ScoreSheet table={table} loadBoardReplay={loadBoardReplay} loadPositionAnalysis={loadPositionAnalysis} />);
}

export function renderDenseDummy(position: "left" | "right") {
  const cards = [
    ...ranks.slice(0, 8).map(rank => ({ suit: "C" as const, rank })),
    { suit: "H" as const, rank: "4" as const },
    ...ranks.slice(0, 4).map(rank => ({ suit: "S" as const, rank }))
  ];
  root.render(<main className="table-client active-table-client">
    <header className="table-status-bar">Dense dummy geometry</header>
    <div className="table-surface"><div className="board-play-zone">
      <BridgeHand className={`dummy-hand dummy-${position}`} title="Dense dummy" variant="dummy" position={position} cards={cards} contractStrain="C" />
      <div className="current-trick" data-dummy-position={position}>
        {(["top", "right", "bottom", "left"] as const).map((side, _index) => <div key={side} className={`trick-slot trick-${side}`}><PlayingCard variant="trick" card={{ suit: "D", rank: ranks[_index]! }} /></div>)}
      </div>
    </div></div>
    <BridgeHand className="own-hand" title="Own hand" cards={fullDeal.north} contractStrain="C" />
  </main>);
}
