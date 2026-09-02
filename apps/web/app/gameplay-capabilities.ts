import {
  oppositeSeat,
  playableHand,
  type Call,
  type Card,
  type CommandName,
  type LiveTableProjection,
  type Seat,
} from "./table-state.ts";

export type CommandCapabilityContext = {
  table: LiveTableProjection | null;
  connected: boolean;
  controllerState: "current" | "mirror" | "resyncing" | "readyToTakeover" | "takeoverPending";
  hasPendingCommand: boolean;
};

function isRecord(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

function isSeat(value: unknown): value is Seat {
  return value === "N" || value === "E" || value === "S" || value === "W";
}

function isCall(value: unknown): value is Call {
  if (!isRecord(value)) return false;
  if (value.kind === "PASS" || value.kind === "DOUBLE" || value.kind === "REDOUBLE") return true;
  return value.kind === "BID" && Number.isInteger(value.level) && ["C", "D", "H", "S", "NT"].includes(String(value.strain));
}

function isCard(value: unknown): value is Card {
  return isRecord(value) && ["C", "D", "H", "S"].includes(String(value.suit)) && ["2", "3", "4", "5", "6", "7", "8", "9", "T", "J", "Q", "K", "A"].includes(String(value.rank));
}

function callMatches(firstCall: Call, secondCall: Call) {
  return firstCall.kind === secondCall.kind && firstCall.level === secondCall.level && firstCall.strain === secondCall.strain;
}

function cardMatches(firstCard: Card, secondCard: Card) {
  return firstCard.suit === secondCard.suit && firstCard.rank === secondCard.rank;
}

export function canSendTableCommand(
  context: CommandCapabilityContext,
  name: CommandName,
  payload: Record<string, unknown> = {},
) {
  const table = context.table;
  if (!context.connected || table === null || context.hasPendingCommand) return false;
  if (name === "table.takeover") return context.controllerState === "readyToTakeover";
  if (context.controllerState !== "current") return false;

  const game = table.game;
  const isOwner = table.viewerRole === "OWNER";
  const seat = isSeat(payload.seat) ? payload.seat : undefined;
  const participantId = typeof payload.participant_id === "string" ? payload.participant_id : undefined;
  const participant = table.participants.find((candidate) => candidate.id === participantId);
  const hasBot = table.participants.some((candidate) => candidate.isBot);

  switch (name) {
    case "game.make_call":
      return table.state === "ACTIVE" && table.actionRequest === undefined && game?.phase === "AUCTION" && game.turn === table.viewerSeat && isCall(payload.call) && game.legalCalls?.some((call) => callMatches(call, payload.call as Call)) === true;
    case "game.play_card": {
      if (table.state !== "ACTIVE" || table.actionRequest !== undefined || (game?.phase !== "OPENING_LEAD" && game?.phase !== "PLAY") || !isCard(payload.card)) return false;
      const playable = playableHand(table);
      return playable?.hand.some((card) => cardMatches(card, payload.card as Card)) === true;
    }
    case "game.request_claim": {
      const dummy = game?.auction.contract === undefined ? undefined : oppositeSeat(game.auction.contract.declarer);
      const remainingTricks = game === undefined ? 0 : 13 - game.completedTricks.length;
      return !hasBot && table.actionRequest === undefined && game?.phase === "PLAY" && game.currentTrick.plays.length === 0 && table.viewerSeat !== undefined && table.viewerSeat !== dummy && Number.isInteger(payload.tricks) && Number(payload.tricks) >= 0 && Number(payload.tricks) <= remainingTricks;
    }
    case "game.request_undo":
      return !hasBot && table.actionRequest === undefined && table.canRequestUndo;
    case "game.respond_claim":
      return table.actionRequest?.kind === "CLAIM" && table.actionRequest.canRespond && typeof payload.accepted === "boolean";
    case "game.respond_undo":
      return table.actionRequest?.kind === "UNDO" && table.actionRequest.canRespond && typeof payload.accepted === "boolean";
    case "table.take_seat":
      return seat !== undefined && table.seats[seat] === undefined && (table.state === "WAITING" || table.viewerSeat === undefined && (table.state === "ACTIVE" || table.state === "BETWEEN_BOARDS"));
    case "table.leave_seat":
      return table.state === "WAITING" && table.viewerSeat !== undefined;
    case "table.set_ready":
      return table.state === "WAITING" && table.viewerSeat !== undefined && typeof payload.ready === "boolean";
    case "table.lock":
      return isOwner && table.state === "WAITING" && typeof payload.locked === "boolean";
    case "table.add_bot":
      return isOwner && table.state !== "FINISHED" && seat !== undefined && table.seats[seat] === undefined;
    case "table.remove_bot":
      return isOwner && table.state !== "FINISHED" && seat !== undefined && table.seats[seat]?.isBot === true;
    case "table.replace_with_bot":
      return isOwner && table.state !== "FINISHED" && participant !== undefined && participant.role === "PARTICIPANT" && participant.id !== table.viewerParticipantId && participant.isBot !== true && Object.values(table.seats).some((assignment) => assignment.participantId === participant.id);
    case "table.remove_participant":
      return isOwner && table.state !== "FINISHED" && participant !== undefined && participant.isBot !== true && (participant.id !== table.viewerParticipantId || table.participants.length > 1);
    case "table.start_game":
      return isOwner && table.state === "WAITING" && (["N", "E", "S", "W"] as Seat[]).every((candidate) => table.seats[candidate]?.ready === true);
    case "table.next_board":
      return isOwner && table.state === "BETWEEN_BOARDS" && table.actionRequest === undefined;
    case "table.finish":
      return isOwner && (table.state === "WAITING" || table.state === "BETWEEN_BOARDS") && table.actionRequest === undefined;
    case "table.leave":
      return table.state === "WAITING" || table.state === "FINISHED";
  }
}
