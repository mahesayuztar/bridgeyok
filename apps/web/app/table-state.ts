import type { MutationCommandEnvelope, TableProjection } from "@bridgeyok/contracts/realtime";

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

export type TableClientState = {
  activeTableId: string | null;
  table: LiveTableProjection | null;
  lastSeenSeq: number;
  pending: Record<string, CommandName>;
  message: string | null;
};

export type TableAction =
  | { type: "enter"; table: LiveTableProjection }
  | { type: "snapshot"; tableId: string; seq: number; table: LiveTableProjection }
  | { type: "event"; tableId: string; seq: number; table: LiveTableProjection }
  | { type: "pending"; requestId: string; commandName: CommandName }
  | { type: "settled"; requestId: string; message?: string }
  | { type: "connectionLost"; message: string }
  | { type: "message"; message: string | null }
  | { type: "clear" };

export function createEmptyTableState(): TableClientState {
  return {
    activeTableId: null,
    table: null,
    lastSeenSeq: 0,
    pending: {},
    message: null
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
        message: null
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
        message: null
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
        message: null
      };
    case "pending":
      return { ...state, pending: { ...state.pending, [action.requestId]: action.commandName }, message: null };
    case "settled": {
      const pending = { ...state.pending };
      delete pending[action.requestId];
      return { ...state, pending, message: action.message ?? null };
    }
    case "connectionLost":
      return { ...state, pending: {}, message: action.message };
    case "message":
      return { ...state, message: action.message };
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
  const ledSuit = game.currentTrick.plays[0]?.card.suit;
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
