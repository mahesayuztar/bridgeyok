import type { MutationCommandEnvelope, TableProjection } from "@bridgeyok/contracts/realtime";
import type { ClientIssue } from "./client-issue";
import {
  acknowledgePendingCommand,
  createPendingTableCommand,
  projectOptimisticTable,
  reconcilePendingCommands,
  type PendingTableCommand,
} from "./optimistic-gameplay.ts";

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
  leader?: Seat;
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

export type ParticipantPresence = {
  participantId: string;
  online: boolean;
  offlineSince?: string;
};

export type VisualPosition = "top" | "right" | "bottom" | "left";
export type TableOrientation = Record<VisualPosition, Seat>;
export type AuctionRow = Partial<Record<Seat, CallRecord>>;

export type TableClientState = {
  activeTableId: string | null;
  table: LiveTableProjection | null;
  lastSeenSeq: number;
  pending: Record<string, PendingTableCommand>;
  presence: Record<string, ParticipantPresence>;
  issue: ClientIssue | null;
  notice: string | null;
  controllerState: "current" | "mirror" | "resyncing" | "readyToTakeover" | "takeoverPending";
};

export type TableAction =
  | { type: "enter"; table: LiveTableProjection }
  | { type: "snapshot"; tableId: string; seq: number; table: LiveTableProjection }
  | { type: "event"; tableId: string; seq: number; table: LiveTableProjection; eventType?: string }
  | { type: "presenceSnapshot"; tableId: string; participants: ParticipantPresence[] }
  | { type: "presenceChanged"; tableId: string; participant: ParticipantPresence }
  | { type: "pending"; requestId: string; commandName: CommandName; payload: Record<string, unknown> }
  | { type: "ack"; requestId: string; revision: number; seq: number }
  | { type: "settled"; requestId: string; issue?: ClientIssue }
  | { type: "controllerSyncStarted" }
  | { type: "conflict"; issue: ClientIssue }
  | { type: "connectionLost"; issue?: ClientIssue }
  | { type: "issue"; issue: ClientIssue | null }
  | { type: "dismissNotice" }
  | { type: "clear" };

export function createEmptyTableState(): TableClientState {
  return {
    activeTableId: null,
    table: null,
    lastSeenSeq: 0,
    pending: {},
    presence: {},
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
        presence: {},
        issue: null,
        notice: null,
        controllerState: "current"
      };
    case "snapshot": {
      if (state.activeTableId !== action.tableId || action.table.tableId !== action.tableId || action.seq < state.lastSeenSeq) {
        return state;
      }
      const resyncComplete = state.controllerState === "resyncing";
      const readyToTakeover = action.table.viewerSeat !== undefined && (resyncComplete || state.controllerState === "readyToTakeover");
      const viewerSeat = action.table.viewerSeat;
      const takeoverConfirmed = state.controllerState === "takeoverPending"
        && viewerSeat !== undefined
        && state.table?.seats[viewerSeat]?.controllerEpoch !== action.table.seats[viewerSeat]?.controllerEpoch;
      return {
        ...state,
        table: action.table,
        lastSeenSeq: Math.max(action.seq, action.table.lastSeq),
        pending: {},
        presence: retainActivePresence(state.presence, action.table),
        issue: null,
        controllerState: takeoverConfirmed
          ? "current"
          : readyToTakeover
            ? "readyToTakeover"
            : state.controllerState === "mirror" || state.controllerState === "takeoverPending"
              ? state.controllerState
              : "current"
      };
    }
    case "event": {
      if (state.activeTableId !== action.tableId || action.table.tableId !== action.tableId || action.seq <= state.lastSeenSeq) {
        return state;
      }
      const becameOwner = state.table?.viewerRole === "PARTICIPANT" && action.table.viewerRole === "OWNER";
      const viewerSeat = action.table.viewerSeat;
      const viewerControllerChanged = action.eventType === "CONTROLLER_REPLACED"
        && viewerSeat !== undefined
        && state.table?.seats[viewerSeat]?.controllerEpoch !== action.table.seats[viewerSeat]?.controllerEpoch;
      const controllerReplaced = viewerControllerChanged && state.controllerState === "takeoverPending";
      const controllerReplacedElsewhere = viewerControllerChanged && state.controllerState === "current";
      const resyncComplete = state.controllerState === "resyncing";
      const readyToTakeover = resyncComplete && action.table.viewerSeat !== undefined;
      return {
        ...state,
        table: action.table,
        lastSeenSeq: Math.max(action.seq, action.table.lastSeq),
        pending: reconcilePendingCommands(state.pending, action.table),
        presence: retainActivePresence(state.presence, action.table),
        issue: controllerReplaced || resyncComplete ? null : state.issue,
        notice: becameOwner
          ? "Kamu sekarang menjadi master meja."
          : action.eventType === "PARTICIPANT_TIMED_OUT"
              ? "Pemain offline sudah dikeluarkan dari meja."
              : state.notice,
        controllerState: controllerReplaced
          ? "current"
          : controllerReplacedElsewhere
            ? "mirror"
          : resyncComplete
            ? readyToTakeover ? "readyToTakeover" : "current"
            : state.controllerState
      };
    }
    case "presenceSnapshot": {
      if (state.activeTableId !== action.tableId || state.table === null) {
        return state;
      }
      const activeParticipantIds = new Set(state.table.participants.map((participant) => participant.id));
      return {
        ...state,
        presence: Object.fromEntries(action.participants.filter((participant) => activeParticipantIds.has(participant.participantId)).map((participant) => [participant.participantId, participant]))
      };
    }
    case "presenceChanged":
      if (state.activeTableId !== action.tableId || state.table === null || !state.table.participants.some((participant) => participant.id === action.participant.participantId)) {
        return state;
      }
      return { ...state, presence: { ...state.presence, [action.participant.participantId]: action.participant } };
    case "pending":
      if (state.table === null) return state;
      return {
        ...state,
        pending: {
          ...state.pending,
          [action.requestId]: createPendingTableCommand(
            state.table,
            action.requestId,
            action.commandName,
            action.payload,
          ),
        },
        issue: null,
        notice: null,
        controllerState: action.commandName === "table.takeover" ? "takeoverPending" : state.controllerState
      };
    case "ack":
      return {
        ...state,
        pending: acknowledgePendingCommand(
          state.pending,
          action.requestId,
          action.revision,
          action.seq,
          state.table?.revision,
        ),
      };
    case "settled": {
      const pending = { ...state.pending };
      const commandName = pending[action.requestId]?.name;
      delete pending[action.requestId];
      return {
        ...state,
        pending,
        issue: action.issue ?? null,
        controllerState: commandName === "table.takeover" && action.issue !== undefined ? "readyToTakeover" : state.controllerState
      };
    }
    case "controllerSyncStarted":
      return { ...state, pending: {}, issue: null, controllerState: "resyncing" };
    case "conflict":
      return { ...state, pending: {}, issue: action.issue, notice: null, controllerState: "resyncing" };
    case "connectionLost":
      return {
        ...state,
        pending: {},
        issue: action.issue ?? null,
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

export function projectedTableState(state: TableClientState) {
  return projectOptimisticTable(state.table, state.pending);
}

function retainActivePresence(presence: Record<string, ParticipantPresence>, table: LiveTableProjection) {
  const activeParticipantIds = new Set(table.participants.map((participant) => participant.id));
  return Object.fromEntries(Object.entries(presence).filter(([participantId]) => activeParticipantIds.has(participantId)));
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

export function auctionRows(dealer: Seat, calls: CallRecord[] | null | undefined): AuctionRow[] {
  const columns: Seat[] = ["W", "N", "E", "S"];
  const dealerIndex = columns.indexOf(dealer);
  const safeCalls = calls ?? [];
  const rowCount = Math.max(1, Math.ceil((dealerIndex + safeCalls.length) / columns.length));
  const rows = Array.from({ length: rowCount }, () => ({} as AuctionRow));
  safeCalls.forEach((record, _index) => {
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
    return "=";
  }
  return difference > 0 ? `+${difference}` : String(difference);
}
