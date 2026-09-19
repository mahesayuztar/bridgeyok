import type { components } from "@bridgeyok/contracts/openapi";
import type { Card, LiveTableProjection, Seat, Trick } from "../../app/table-state";

type MatchView = components["schemas"]["MatchView"];

const participantIds = {
  N: "00000000-0000-4000-8000-000000000001",
  E: "00000000-0000-4000-8000-000000000002",
  S: "00000000-0000-4000-8000-000000000003",
  W: "00000000-0000-4000-8000-000000000004",
} satisfies Record<Seat, string>;

const hands = {
  N: ["SA", "SK", "HQ", "HJ", "D9", "D8", "D7", "C6", "C5", "C4", "C3", "C2", "S2"],
  E: ["SQ", "SJ", "S10", "H10", "H9", "H8", "D6", "D5", "D4", "CK", "CQ", "CJ", "C10"],
  S: ["S9", "S8", "S7", "HA", "HK", "H7", "D3", "D2", "C9", "C8", "C7", "CA", "DA"],
  W: ["S6", "S5", "S4", "S3", "H6", "H5", "H4", "H3", "H2", "DK", "DQ", "DJ", "D10"],
} satisfies Record<Seat, string[]>;

function card(encoded: string): Card {
  const suit = encoded[0] as Card["suit"];
  const rank = encoded.slice(1).replace("10", "T") as Card["rank"];
  return { suit, rank };
}

const fullDeal = {
  north: hands.N.map(card),
  east: hands.E.map(card),
  south: hands.S.map(card),
  west: hands.W.map(card),
};

const participants = (["N", "E", "S", "W"] as Seat[]).map(seat => ({
  id: participantIds[seat],
  nickname: `${seat} Fixture`,
  role: seat === "N" ? "OWNER" as const : "PARTICIPANT" as const,
  isBot: false,
}));

const seats = Object.fromEntries((Object.keys(participantIds) as Seat[]).map(seat => [seat, {
  participantId: participantIds[seat],
  ready: true,
  controllerEpoch: 1,
}])) as LiveTableProjection["seats"];

const auction = {
  dealer: "N" as const,
  complete: true,
  passedOut: false,
  calls: [
    { seat: "N" as const, call: { kind: "BID" as const, level: 1, strain: "NT" as const } },
    { seat: "E" as const, call: { kind: "PASS" as const } },
    { seat: "S" as const, call: { kind: "PASS" as const } },
    { seat: "W" as const, call: { kind: "PASS" as const } },
  ],
  contract: { level: 1, strain: "NT" as const, doubling: "UNDOUBLED" as const, declarer: "N" as const },
};

const completedTrick: Trick = {
  leader: "E",
  plays: [
    { seat: "E", card: card("SQ") },
    { seat: "S", card: card("S9") },
    { seat: "W", card: card("S6") },
    { seat: "N", card: card("SA") },
  ],
  winner: "N",
};

function tableFixture(viewerSeat: Seat, revision: number): LiveTableProjection {
  return {
    tableId: "10000000-0000-4000-8000-000000000001",
    state: "ACTIVE",
    locked: true,
    revision,
    lastSeq: revision,
    boardId: "20000000-0000-4000-8000-000000000001",
    boardNumber: 1,
    scoreSheet: [],
    pairScoreTotals: [],
    viewerParticipantId: participantIds[viewerSeat],
    viewerRole: viewerSeat === "N" ? "OWNER" : "PARTICIPANT",
    viewerSeat,
    participants,
    seats,
    canRequestUndo: false,
  };
}

export const phase6SessionFixtures = {
  auction: {
    ...tableFixture("N", 10),
    game: {
      rulesetVersion: "bridgeyok_duplicate_v1",
      board: { number: 1, dealer: "N", vulnerability: "NONE" },
      phase: "AUCTION",
      auction: { dealer: "N", turn: "N", complete: false, passedOut: false, calls: [] },
      legalCalls: [{ kind: "PASS" }, { kind: "BID", level: 1, strain: "NT" }],
      turn: "N",
      dummyRevealed: false,
      currentTrick: { plays: [] },
      completedTrickCount: 0,
      completedTricks: [],
      tricksNS: 0,
      tricksEW: 0,
      ownHand: fullDeal.north,
    },
  },
  openingLead: {
    ...tableFixture("E", 15),
    game: {
      rulesetVersion: "bridgeyok_duplicate_v1",
      board: { number: 1, dealer: "N", vulnerability: "NONE" },
      phase: "OPENING_LEAD",
      auction,
      turn: "E",
      dummyRevealed: false,
      currentTrick: { leader: "E", plays: [] },
      completedTrickCount: 0,
      completedTricks: [],
      tricksNS: 0,
      tricksEW: 0,
      ownHand: fullDeal.east,
    },
  },
  middleTrickDummy: {
    ...tableFixture("S", 21),
    game: {
      rulesetVersion: "bridgeyok_duplicate_v1",
      board: { number: 1, dealer: "N", vulnerability: "NONE" },
      phase: "PLAY",
      auction,
      turn: "S",
      dummyRevealed: true,
      currentTrick: { leader: "N", plays: [{ seat: "N", card: card("SK") }, { seat: "E", card: card("SJ") }] },
      completedTrickCount: 1,
      completedTricks: [completedTrick],
      tricksNS: 1,
      tricksEW: 0,
      ownHand: fullDeal.south.slice(1),
      dummyHand: fullDeal.south.slice(1),
    },
  },
  middleTrickDefender: {
    ...tableFixture("W", 21),
    game: {
      rulesetVersion: "bridgeyok_duplicate_v1",
      board: { number: 1, dealer: "N", vulnerability: "NONE" },
      phase: "PLAY",
      auction,
      turn: "S",
      dummyRevealed: true,
      currentTrick: { leader: "N", plays: [{ seat: "N", card: card("SK") }, { seat: "E", card: card("SJ") }] },
      completedTrickCount: 1,
      completedTricks: [completedTrick],
      tricksNS: 1,
      tricksEW: 0,
      ownHand: fullDeal.west.slice(1),
    },
  },
  scored: {
    ...tableFixture("N", 70),
    state: "BETWEEN_BOARDS" as const,
    canRequestUndo: true,
    game: {
      rulesetVersion: "bridgeyok_duplicate_v1",
      board: { number: 1, dealer: "N", vulnerability: "NONE" },
      phase: "BOARD_SCORED",
      auction,
      dummyRevealed: true,
      currentTrick: { plays: [] },
      completedTrickCount: 13,
      completedTricks: [completedTrick],
      tricksNS: 7,
      tricksEW: 6,
      result: {
        passedOut: false,
        contract: auction.contract,
        tricksDeclarer: 7,
        tricksNS: 7,
        tricksEW: 6,
        vulnerability: "NONE",
        scoreNS: 90,
      },
      ownHand: [],
      dummyHand: [],
      fullDeal,
    },
  },
} satisfies Record<string, LiveTableProjection>;

const activeMatch: MatchView = {
  id: "30000000-0000-4000-8000-000000000001",
  status: "ACTIVE",
  revision: 9,
  isOwner: true,
  tableId: phase6SessionFixtures.auction.tableId,
  tableRevision: phase6SessionFixtures.auction.revision,
  room: "OPEN",
  seat: "N",
  team: "A",
  boardCount: 2,
  ready: true,
  readyCount: 8,
  openCompleted: 1,
  closedCompleted: 0,
  canStart: false,
  canCancel: true,
  syncPending: false,
  teamAIMP: 0,
};

export const phase6MatchFixtures = {
  active: activeMatch,
  complete: {
    ...activeMatch,
    status: "COMPLETE",
    revision: 12,
    openCompleted: 2,
    closedCompleted: 2,
    canCancel: false,
    comparisons: [
      { boardId: "20000000-0000-4000-8000-000000000001", openScoreNS: 90, closedScoreNS: -50, teamAIMP: 4 },
      { boardId: "20000000-0000-4000-8000-000000000002", openScoreNS: -110, closedScoreNS: -110, teamAIMP: 0 },
    ],
    teamAIMP: 4,
  },
} satisfies Record<"active" | "complete", MatchView>;
