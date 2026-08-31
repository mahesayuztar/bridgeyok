import type {
  BoardResult,
  Call,
  CallRecord,
  Card,
  Contract,
  GameProjection,
  LiveTableProjection,
  ParticipantPresence,
  Seat,
  Suit,
  Trick
} from "./table-state";

const SEATS: Seat[] = ["N", "E", "S", "W"];
const SUITS: Suit[] = ["C", "D", "H", "S"];
const RANKS: Card["rank"][] = ["2", "3", "4", "5", "6", "7", "8", "9", "T", "J", "Q", "K", "A"];
const STRAINS: Contract["strain"][] = ["C", "D", "H", "S", "NT"];
const VULNERABILITIES: BoardResult["vulnerability"][] = ["NONE", "NS", "EW", "BOTH"];

function isDateTime(value: unknown): value is string {
  return typeof value === "string" && Number.isFinite(Date.parse(value));
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

function isSeat(value: unknown): value is Seat {
  return typeof value === "string" && SEATS.includes(value as Seat);
}

function isFiniteNumber(value: unknown): value is number {
  return typeof value === "number" && Number.isFinite(value);
}

function normalizeArray<T>(value: unknown, normalizeItem: (item: unknown) => T | null): T[] | null {
  if (value === null || value === undefined) {
    return [];
  }
  if (!Array.isArray(value)) {
    return null;
  }
  const items: T[] = [];
  for (const item of value) {
    const normalizedItem = normalizeItem(item);
    if (normalizedItem === null) {
      return null;
    }
    items.push(normalizedItem);
  }
  return items;
}

function normalizeCard(value: unknown): Card | null {
  if (!isRecord(value) || typeof value.suit !== "string" || !SUITS.includes(value.suit as Suit) || typeof value.rank !== "string" || !RANKS.includes(value.rank as Card["rank"])) {
    return null;
  }
  return { suit: value.suit as Suit, rank: value.rank as Card["rank"] };
}

function normalizeCall(value: unknown): Call | null {
  if (!isRecord(value) || !["PASS", "BID", "DOUBLE", "REDOUBLE"].includes(String(value.kind))) {
    return null;
  }
  if (value.kind !== "BID") {
    return { kind: value.kind as Exclude<Call["kind"], "BID"> };
  }
  if (!Number.isInteger(value.level) || !STRAINS.includes(value.strain as Contract["strain"])) {
    return null;
  }
  return { kind: "BID", level: value.level as number, strain: value.strain as Contract["strain"] };
}

function normalizeCallRecord(value: unknown): CallRecord | null {
  if (!isRecord(value) || !isSeat(value.seat)) {
    return null;
  }
  const call = normalizeCall(value.call);
  return call === null ? null : { seat: value.seat, call };
}

function normalizeContract(value: unknown): Contract | null {
  if (
    !isRecord(value) ||
    !Number.isInteger(value.level) ||
    !STRAINS.includes(value.strain as Contract["strain"]) ||
    !["UNDOUBLED", "DOUBLED", "REDOUBLED"].includes(String(value.doubling)) ||
    !isSeat(value.declarer)
  ) {
    return null;
  }
  return {
    level: value.level as number,
    strain: value.strain as Contract["strain"],
    doubling: value.doubling as Contract["doubling"],
    declarer: value.declarer
  };
}

function normalizeTrick(value: unknown): Trick | null {
  if (!isRecord(value)) {
    return null;
  }
  const plays = normalizeArray(value.plays, (play) => {
    if (!isRecord(play) || !isSeat(play.seat)) {
      return null;
    }
    const card = normalizeCard(play.card);
    return card === null ? null : { seat: play.seat, card };
  });
  if (plays === null) {
    return null;
  }
  return {
    ...(isSeat(value.leader) ? { leader: value.leader } : {}),
    plays,
    ...(isSeat(value.winner) ? { winner: value.winner } : {})
  };
}

function normalizeResult(value: unknown): BoardResult | null {
  if (
    !isRecord(value) ||
    typeof value.passedOut !== "boolean" ||
    !isFiniteNumber(value.tricksDeclarer) ||
    !isFiniteNumber(value.tricksNS) ||
    !isFiniteNumber(value.tricksEW) ||
    !VULNERABILITIES.includes(value.vulnerability as BoardResult["vulnerability"]) ||
    !isFiniteNumber(value.scoreNS)
  ) {
    return null;
  }
  const contract = value.contract === null || value.contract === undefined ? undefined : normalizeContract(value.contract);
  if (contract === null) {
    return null;
  }
  return {
    passedOut: value.passedOut,
    ...(contract === undefined ? {} : { contract }),
    tricksDeclarer: value.tricksDeclarer,
    tricksNS: value.tricksNS,
    tricksEW: value.tricksEW,
    vulnerability: value.vulnerability as BoardResult["vulnerability"],
    scoreNS: value.scoreNS
  };
}

function normalizeGame(value: unknown): GameProjection | null {
  if (!isRecord(value) || !isRecord(value.board) || !isRecord(value.auction)) {
    return null;
  }
  const board = value.board;
  const auction = value.auction;
  if (
    typeof value.rulesetVersion !== "string" ||
    !["AUCTION", "OPENING_LEAD", "PLAY", "BOARD_SCORED"].includes(String(value.phase)) ||
    !Number.isInteger(board.number) ||
    !isSeat(board.dealer) ||
    !VULNERABILITIES.includes(board.vulnerability as BoardResult["vulnerability"]) ||
    !isSeat(auction.dealer) ||
    typeof auction.complete !== "boolean" ||
    typeof auction.passedOut !== "boolean" ||
    typeof value.dummyRevealed !== "boolean" ||
    !isFiniteNumber(value.tricksNS) ||
    !isFiniteNumber(value.tricksEW)
  ) {
    return null;
  }

  const calls = normalizeArray(auction.calls, normalizeCallRecord);
  const legalCalls = normalizeArray(value.legalCalls, normalizeCall);
  const currentTrick = value.currentTrick === null || value.currentTrick === undefined
    ? { plays: [] }
    : normalizeTrick(value.currentTrick);
  const completedTricks = normalizeArray(value.completedTricks, normalizeTrick);
  const ownHand = normalizeArray(value.ownHand, normalizeCard);
  if (calls === null || legalCalls === null || currentTrick === null || completedTricks === null || ownHand === null) {
    return null;
  }

  const contract = auction.contract === null || auction.contract === undefined ? undefined : normalizeContract(auction.contract);
  const result = value.result === null || value.result === undefined ? undefined : normalizeResult(value.result);
  const dummyHand = value.dummyHand === null || value.dummyHand === undefined ? undefined : normalizeArray(value.dummyHand, normalizeCard);
  if (contract === null || result === null || dummyHand === null) {
    return null;
  }

  let fullDeal: GameProjection["fullDeal"];
  if (value.fullDeal !== null && value.fullDeal !== undefined) {
    if (!isRecord(value.fullDeal)) {
      return null;
    }
    const north = normalizeArray(value.fullDeal.north, normalizeCard);
    const east = normalizeArray(value.fullDeal.east, normalizeCard);
    const south = normalizeArray(value.fullDeal.south, normalizeCard);
    const west = normalizeArray(value.fullDeal.west, normalizeCard);
    if (north === null || east === null || south === null || west === null) {
      return null;
    }
    fullDeal = { north, east, south, west };
  }

  return {
    rulesetVersion: value.rulesetVersion,
    board: {
      number: board.number as number,
      dealer: board.dealer,
      vulnerability: board.vulnerability as BoardResult["vulnerability"]
    },
    phase: value.phase as GameProjection["phase"],
    auction: {
      dealer: auction.dealer,
      ...(isSeat(auction.turn) ? { turn: auction.turn } : {}),
      calls,
      complete: auction.complete,
      passedOut: auction.passedOut,
      ...(contract === undefined ? {} : { contract })
    },
    legalCalls,
    ...(isSeat(value.turn) ? { turn: value.turn } : {}),
    dummyRevealed: value.dummyRevealed,
    currentTrick,
    completedTricks,
    tricksNS: value.tricksNS,
    tricksEW: value.tricksEW,
    ...(result === undefined ? {} : { result }),
    ownHand,
    ...(dummyHand === undefined ? {} : { dummyHand }),
    ...(fullDeal === undefined ? {} : { fullDeal })
  };
}

export function normalizeLiveTableProjection(value: unknown): LiveTableProjection | null {
  if (!isRecord(value)) {
    return null;
  }
  if (
    typeof value.tableId !== "string" ||
    !["WAITING", "ACTIVE", "BETWEEN_BOARDS", "FINISHED"].includes(String(value.state)) ||
    typeof value.locked !== "boolean" ||
    !isFiniteNumber(value.revision) ||
    !isFiniteNumber(value.lastSeq) ||
    !isFiniteNumber(value.boardNumber) ||
    typeof value.viewerParticipantId !== "string" ||
    !["OWNER", "PARTICIPANT"].includes(String(value.viewerRole))
  ) {
    return null;
  }

  const participants = normalizeArray(value.participants, (participant) => {
    if (!isRecord(participant) || typeof participant.id !== "string" || typeof participant.nickname !== "string" || !["OWNER", "PARTICIPANT"].includes(String(participant.role))) {
      return null;
    }
    return {
      id: participant.id,
      nickname: participant.nickname,
      role: participant.role as "OWNER" | "PARTICIPANT"
    };
  });
  if (participants === null || (value.seats !== null && value.seats !== undefined && !isRecord(value.seats))) {
    return null;
  }

  const rawSeats = isRecord(value.seats) ? value.seats : {};
  const seats: LiveTableProjection["seats"] = {};
  for (const seat of SEATS) {
    const assignment = rawSeats[seat];
    if (assignment === null || assignment === undefined) {
      continue;
    }
    if (!isRecord(assignment) || typeof assignment.participantId !== "string" || typeof assignment.ready !== "boolean" || !isFiniteNumber(assignment.controllerEpoch)) {
      return null;
    }
    seats[seat] = {
      participantId: assignment.participantId,
      ready: assignment.ready,
      controllerEpoch: assignment.controllerEpoch
    };
  }

  const game = value.game === null || value.game === undefined ? undefined : normalizeGame(value.game);
  if (game === null) {
    return null;
  }

  let actionRequest: LiveTableProjection["actionRequest"];
  if (value.actionRequest !== null && value.actionRequest !== undefined) {
    if (!isRecord(value.actionRequest) || !["CLAIM", "UNDO"].includes(String(value.actionRequest.kind)) || !isSeat(value.actionRequest.requesterSeat) || typeof value.actionRequest.canRespond !== "boolean") {
      return null;
    }
    const approvedBy = normalizeArray(value.actionRequest.approvedBy, (seat) => isSeat(seat) ? seat : null);
    if (approvedBy === null || value.actionRequest.kind === "CLAIM" && !Number.isInteger(value.actionRequest.claimTricks)) {
      return null;
    }
    actionRequest = {
      kind: value.actionRequest.kind as "CLAIM" | "UNDO",
      requesterSeat: value.actionRequest.requesterSeat,
      ...(Number.isInteger(value.actionRequest.claimTricks) ? { claimTricks: value.actionRequest.claimTricks as number } : {}),
      approvedBy,
      canRespond: value.actionRequest.canRespond
    };
  }

  return {
    tableId: value.tableId,
    state: value.state as LiveTableProjection["state"],
    locked: value.locked,
    revision: value.revision,
    lastSeq: value.lastSeq,
    ...(typeof value.boardId === "string" ? { boardId: value.boardId } : {}),
    boardNumber: value.boardNumber,
    viewerParticipantId: value.viewerParticipantId,
    viewerRole: value.viewerRole as LiveTableProjection["viewerRole"],
    ...(isSeat(value.viewerSeat) ? { viewerSeat: value.viewerSeat } : {}),
    participants,
    seats,
    ...(game === undefined ? {} : { game }),
    ...(actionRequest === undefined ? {} : { actionRequest }),
    canRequestUndo: typeof value.canRequestUndo === "boolean" ? value.canRequestUndo : false
  };
}

export function normalizeParticipantPresence(value: unknown): ParticipantPresence | null {
  if (!isRecord(value) || typeof value.participantId !== "string" || typeof value.online !== "boolean") {
    return null;
  }
  if (value.offlineSince !== undefined && !isDateTime(value.offlineSince)) {
    return null;
  }
  if (value.expiresAt !== undefined && !isDateTime(value.expiresAt)) {
    return null;
  }
  return {
    participantId: value.participantId,
    online: value.online,
    ...(isDateTime(value.offlineSince) ? { offlineSince: value.offlineSince } : {}),
    ...(isDateTime(value.expiresAt) ? { expiresAt: value.expiresAt } : {})
  };
}

export function normalizePresenceSnapshot(value: unknown): ParticipantPresence[] | null {
  if (!isRecord(value)) {
    return null;
  }
  return normalizeArray(value.participants, normalizeParticipantPresence);
}
