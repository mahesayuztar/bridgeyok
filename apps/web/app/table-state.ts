import type { MutationCommandEnvelope, TableProjection } from "@bridgeyok/contracts/realtime";
import type { ClientIssue } from "./client-issue";

export type Seat = "N" | "E" | "S" | "W";
export type Suit = "C" | "D" | "H" | "S";
export type Rank = "2" | "3" | "4" | "5" | "6" | "7" | "8" | "9" | "T" | "J" | "Q" | "K" | "A";
export type CommandName = MutationCommandEnvelope["name"];

export type Card = {
  suit: Suit;
  rank: Rank;
};

export type Call = {
  kind: "PASS" | "BID" | "DOUBLE" | "REDOUBLE";
  level?: number;
  strain?: "C" | "D" | "H" | "S" | "NT";
};

export type CallRecord = {
  seat: Seat;
  call: Call;
};

export type Contract = {
  level: number;
  strain: "C" | "D" | "H" | "S" | "NT";
  doubling: "UNDOUBLED" | "DOUBLED" | "REDOUBLED";
  declarer: Seat;
};

export type Trick = {
  leader: Seat;
  plays: Array<{ seat: Seat; card: Card }>;
  winner?: Seat;
};

export type BoardResult = {
  passedOut: boolean;
  contract?: Contract;
  tricksDeclarer: number;
  tricksNS: number;
  tricksEW: number;
  vulnerability: "NONE" | "NS" | "EW" | "BOTH";
  scoreNS: number;
};

export type GameProjection = {
  rulesetVersion: string;
  board: {
    number: number;
    dealer: Seat;
    vulnerability: "NONE" | "NS" | "EW" | "BOTH";
  };
  phase: "AUCTION" | "OPENING_LEAD" | "PLAY" | "BOARD_SCORED";
  auction: {
    dealer: Seat;
    turn?: Seat;
    calls: CallRecord[];
    complete: boolean;
    passedOut: boolean;
    contract?: Contract;
  };
  legalCalls?: Call[];
  turn?: Seat;
  dummyRevealed: boolean;
  currentTrick: Trick;
  completedTricks: Trick[];
  tricksNS: number;
  tricksEW: number;
  result?: BoardResult;
  ownHand: Card[];
  dummyHand?: Card[];
  fullDeal?: {
    north: Card[];
    east: Card[];
    south: Card[];
    west: Card[];
  };
};

export type LiveTableProjection = Omit<TableProjection, "game"> & {
  game?: GameProjection;
};

export type VisualPosition = "top" | "right" | "bottom" | "left";
export type TableOrientation = Record<VisualPosition, Seat>;
export type AuctionRow = Partial<Record<Seat, CallRecord>>;

export type TableClientState = {
  activeTableId: string | null;
  table: LiveTableProjection | null;
  lastSeenSeq: number;
  pending: Record<string, CommandName>;
  issue: ClientIssue | null;
  notice: string | null;
  controllerState: "current" | "resyncing" | "readyToTakeover" | "takeoverPending";
};

export type TableAction =
  | { type: "enter"; table: LiveTableProjection }
  | { type: "snapshot"; tableId: string; seq: number; table: LiveTableProjection }
  | { type: "event"; tableId: string; seq: number; table: LiveTableProjection; eventType?: string }
  | { type: "pending"; requestId: string; commandName: CommandName }
  | { type: "settled"; requestId: string; issue?: ClientIssue }
  | { type: "conflict"; issue: ClientIssue }
  | { type: "connectionLost"; issue: ClientIssue }
  | { type: "issue"; issue: ClientIssue | null }
  | { type: "dismissNotice" }
  | { type: "clear" };

export function createEmptyTableState(): TableClientState {
  return {
    activeTableId: null,
    table: null,
    lastSeenSeq: 0,
    pending: {},
    issue: null,
    notice: null,
    controllerState: "current"
  };
}

export function reduceTableState(state: TableClientState, action: TableAction): TableClientState {
  switch (action.type) {
    case "enter":
      return {
        activeTableId: action.table.tableId,
        table: action.table,
        lastSeenSeq: action.table.lastSeq,
        pending: {},
        issue: null,
        notice: null,
        controllerState: "current"
      };
    case "snapshot":
      if (state.activeTableId !== action.tableId || action.table.tableId !== action.tableId || action.seq < state.lastSeenSeq) {
        return state;
      }
      return {
        ...state,
        table: action.table,
        lastSeenSeq: Math.max(action.seq, action.table.lastSeq),
        pending: {},
        issue: state.controllerState === "resyncing" ? {
          kind: "conflict",
          title: "Meja sudah selaras",
          detail: "Keadaan terbaru sudah diterima. Ambil alih bila kamu ingin mengendalikan kursi dari perangkat ini.",
          retryable: true,
          action: "takeover",
          source: "websocket"
        } : null,
        controllerState: state.controllerState === "resyncing" && action.table.viewerSeat !== undefined ? "readyToTakeover" : "current"
      };
    case "event":
      if (state.activeTableId !== action.tableId || action.table.tableId !== action.tableId || action.seq <= state.lastSeenSeq) {
        return state;
      }
      return {
        ...state,
        table: action.table,
        lastSeenSeq: Math.max(action.seq, action.table.lastSeq),
        pending: {},
        issue: action.eventType === "CONTROLLER_REPLACED" && state.controllerState === "takeoverPending" ? null : state.issue,
        notice: action.eventType === "CONTROLLER_REPLACED" && state.controllerState === "takeoverPending" ? "Kendali sudah berpindah ke perangkat ini." : state.notice,
        controllerState: action.eventType === "CONTROLLER_REPLACED" && state.controllerState === "takeoverPending"
          ? "current"
          : state.controllerState === "resyncing" && action.table.viewerSeat !== undefined
            ? "readyToTakeover"
            : state.controllerState
      };
    case "pending":
      return {
        ...state,
        pending: { ...state.pending, [action.requestId]: action.commandName },
        issue: null,
        notice: null,
        controllerState: action.commandName === "table.takeover" ? "takeoverPending" : state.controllerState
      };
    case "settled": {
      const pending = { ...state.pending };
      const commandName = pending[action.requestId];
      delete pending[action.requestId];
      return {
        ...state,
        pending,
        issue: action.issue ?? null,
        controllerState: commandName === "table.takeover" && action.issue !== undefined ? "readyToTakeover" : state.controllerState
      };
    }
    case "conflict":
      return { ...state, pending: {}, issue: action.issue, notice: null, controllerState: "resyncing" };
    case "connectionLost":
      return {
        ...state,
        pending: {},
        issue: action.issue,
        controllerState: state.controllerState === "takeoverPending" ? "resyncing" : state.controllerState
      };
    case "issue":
      return { ...state, issue: action.issue };
    case "dismissNotice":
      return { ...state, notice: null };
    case "clear":
      return createEmptyTableState();
  }
}

export function playableHand(table: LiveTableProjection): { hand: Card[]; source: "own" | "dummy" } | null {
  const game = table.game;
  const viewerSeat = table.viewerSeat;
  if (game === undefined || viewerSeat === undefined || game.turn === undefined) {
    return null;
  }
  let hand: Card[] | undefined;
  let source: "own" | "dummy" = "own";
  if (game.turn === viewerSeat) {
    hand = game.ownHand;
  } else if (game.auction.contract?.declarer === viewerSeat && game.turn === oppositeSeat(viewerSeat)) {
    hand = game.dummyHand;
    source = "dummy";
  }
  if (hand === undefined) {
    return null;
  }
  const ledSuit = game?.currentTrick?.plays?.[0]?.card.suit;
  if (ledSuit === undefined || !hand.some((card) => card.suit === ledSuit)) {
    return { hand, source };
  }
  return { hand: hand.filter((card) => card.suit === ledSuit), source };
}

export function oppositeSeat(seat: Seat): Seat {
  switch (seat) {
    case "N":
      return "S";
    case "E":
      return "W";
    case "S":
      return "N";
    case "W":
      return "E";
  }
}

export function tableOrientation(viewerSeat: Seat = "S"): TableOrientation {
  const clockwiseSeats: Seat[] = ["N", "E", "S", "W"];
  const viewerIndex = clockwiseSeats.indexOf(viewerSeat);
  return {
    top: clockwiseSeats[(viewerIndex + 2) % 4]!,
    right: clockwiseSeats[(viewerIndex + 3) % 4]!,
    bottom: viewerSeat,
    left: clockwiseSeats[(viewerIndex + 1) % 4]!
  };
}

export function visualPositionForSeat(orientation: TableOrientation, seat: Seat): VisualPosition {
  const entry = Object.entries(orientation).find(([, actualSeat]) => actualSeat === seat);
  return (entry?.[0] as VisualPosition | undefined) ?? "bottom";
}

export function auctionRows(dealer: Seat, calls: CallRecord[]): AuctionRow[] {
  const columns: Seat[] = ["W", "N", "E", "S"];
  const dealerIndex = columns.indexOf(dealer);
  const rowCount = Math.max(1, Math.ceil((dealerIndex + calls.length) / columns.length));
  const rows = Array.from({ length: rowCount }, () => ({} as AuctionRow));
  calls.forEach((record, _index) => {
    const slot = dealerIndex + _index;
    rows[Math.floor(slot / columns.length)]![columns[slot % columns.length]!] = record;
  });
  return rows;
}

export function boardResultLabel(result: BoardResult): string {
  if (result.passedOut || result.contract === undefined) {
    return "Passed out";
  }
  const difference = result.tricksDeclarer - (6 + result.contract.level);
  if (difference === 0) {
    return "Tepat kontrak";
  }
  return difference > 0 ? `+${difference}` : String(difference);
}
