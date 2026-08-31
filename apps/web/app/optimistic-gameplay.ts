import type {
  Call,
  Card,
  CommandName,
  GameProjection,
  LiveTableProjection,
  Seat,
} from "./table-state";

type OptimisticCall = {
  kind: "call";
  actorSeat: Seat;
  call: Call;
  callCountBefore: number;
};

type OptimisticPlay = {
  kind: "play";
  actorSeat: Seat;
  card: Card;
  source: "own" | "dummy";
};

export type PendingTableCommand = {
  requestId: string;
  name: CommandName;
  payload: Record<string, unknown>;
  baseRevision: number;
  status: "sent" | "accepted";
  ackRevision?: number;
  ackSeq?: number;
  optimistic?: OptimisticCall | OptimisticPlay;
};

function isRecord(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

function isCall(value: unknown): value is Call {
  if (!isRecord(value)) return false;
  if (value.kind === "PASS" || value.kind === "DOUBLE" || value.kind === "REDOUBLE") {
    return true;
  }
  return (
    value.kind === "BID" &&
    Number.isInteger(value.level) &&
    typeof value.strain === "string" &&
    ["C", "D", "H", "S", "NT"].includes(value.strain)
  );
}

function isCard(value: unknown): value is Card {
  return (
    isRecord(value) &&
    typeof value.suit === "string" &&
    ["C", "D", "H", "S"].includes(value.suit) &&
    typeof value.rank === "string" &&
    ["2", "3", "4", "5", "6", "7", "8", "9", "T", "J", "Q", "K", "A"].includes(value.rank)
  );
}

function callMatches(firstCall: Call, secondCall: Call) {
  return (
    firstCall.kind === secondCall.kind &&
    firstCall.level === secondCall.level &&
    firstCall.strain === secondCall.strain
  );
}

function cardMatches(firstCard: Card, secondCard: Card) {
  return firstCard.suit === secondCard.suit && firstCard.rank === secondCard.rank;
}

function nextSeat(seat: Seat): Seat {
  const seats: Seat[] = ["N", "E", "S", "W"];
  return seats[(seats.indexOf(seat) + 1) % seats.length]!;
}

function playableSource(
  table: LiveTableProjection,
  card: Card,
): "own" | "dummy" | null {
  const game = table.game;
  const viewerSeat = table.viewerSeat;
  if (game === undefined || viewerSeat === undefined || game.turn === undefined) {
    return null;
  }
  let hand: Card[] | undefined;
  let source: "own" | "dummy" = "own";
  if (game.turn === viewerSeat) {
    hand = game.ownHand;
  } else if (
    game.auction.contract?.declarer === viewerSeat &&
    game.turn === nextSeat(nextSeat(viewerSeat))
  ) {
    hand = game.dummyHand;
    source = "dummy";
  }
  if (hand === undefined || !hand.some((candidate) => cardMatches(candidate, card))) {
    return null;
  }
  const ledSuit = game.currentTrick.plays[0]?.card.suit;
  if (
    ledSuit !== undefined &&
    card.suit !== ledSuit &&
    hand.some((candidate) => candidate.suit === ledSuit)
  ) {
    return null;
  }
  return source;
}

export function createPendingTableCommand(
  table: LiveTableProjection,
  requestId: string,
  name: CommandName,
  payload: Record<string, unknown>,
): PendingTableCommand {
  let optimistic: OptimisticCall | OptimisticPlay | undefined;
  if (
    name === "game.make_call" &&
    table.game?.phase === "AUCTION" &&
    table.viewerSeat !== undefined &&
    table.game.turn === table.viewerSeat &&
    isCall(payload.call) &&
    table.game.legalCalls?.some((call) => callMatches(call, payload.call as Call))
  ) {
    optimistic = {
      kind: "call",
      actorSeat: table.game.turn,
      call: payload.call,
      callCountBefore: table.game.auction.calls.length,
    };
  } else if (name === "game.play_card" && isCard(payload.card)) {
    const source = playableSource(table, payload.card);
    if (source !== null && table.game?.turn !== undefined) {
      optimistic = {
        kind: "play",
        actorSeat: table.game.turn,
        card: payload.card,
        source,
      };
    }
  }
  return {
    requestId,
    name,
    payload,
    baseRevision: table.revision,
    status: "sent",
    ...(optimistic === undefined ? {} : { optimistic }),
  };
}

function operationVisible(
  table: LiveTableProjection,
  operation: OptimisticCall | OptimisticPlay,
) {
  const game = table.game;
  if (game === undefined) return false;
  if (operation.kind === "call") {
    const record = game.auction.calls[operation.callCountBefore];
    return (
      record?.seat === operation.actorSeat &&
      callMatches(record.call, operation.call)
    );
  }
  const played = [game.currentTrick, ...game.completedTricks].some((trick) =>
    trick.plays.some(
      (play) =>
        play.seat === operation.actorSeat && cardMatches(play.card, operation.card),
    ),
  );
  return played;
}

function canApplyOperation(
  table: LiveTableProjection,
  operation: OptimisticCall | OptimisticPlay,
) {
  const game = table.game;
  if (game === undefined || game.turn !== operation.actorSeat) return false;
  if (operation.kind === "call") {
    return (
      game.phase === "AUCTION" &&
      game.auction.calls.length === operation.callCountBefore &&
      game.legalCalls?.some((call) => callMatches(call, operation.call)) === true
    );
  }
  return playableSource(table, operation.card) === operation.source;
}

function applyOperation(
  table: LiveTableProjection,
  operation: OptimisticCall | OptimisticPlay,
): LiveTableProjection {
  const game = table.game;
  if (game === undefined || !canApplyOperation(table, operation)) return table;
  if (operation.kind === "call") {
    const nextTurn = nextSeat(operation.actorSeat);
    return {
      ...table,
      game: {
        ...game,
        turn: nextTurn,
        legalCalls: [],
        auction: {
          ...game.auction,
          turn: nextTurn,
          calls: [
            ...game.auction.calls,
            { seat: operation.actorSeat, call: operation.call },
          ],
        },
      },
    };
  }
  const sourceHand = operation.source === "own" ? game.ownHand : game.dummyHand;
  if (sourceHand === undefined) return table;
  const plays = [
    ...game.currentTrick.plays,
    { seat: operation.actorSeat, card: operation.card },
  ];
  const nextTurn = plays.length < 4 ? nextSeat(operation.actorSeat) : undefined;
  const projectedGame: GameProjection = {
    ...game,
    currentTrick: { ...game.currentTrick, plays },
    ...(operation.source === "own"
      ? {
          ownHand: game.ownHand.filter(
            (card) => !cardMatches(card, operation.card),
          ),
        }
      : {
          dummyHand: sourceHand.filter(
            (card) => !cardMatches(card, operation.card),
          ),
        }),
  };
  if (nextTurn === undefined) {
    delete projectedGame.turn;
  } else {
    projectedGame.turn = nextTurn;
  }
  return {
    ...table,
    game: projectedGame,
  };
}

export function projectOptimisticTable(
  table: LiveTableProjection | null,
  pending: Record<string, PendingTableCommand>,
) {
  if (table === null) return null;
  return Object.values(pending).reduce(
    (projectedTable, command) =>
      command.optimistic === undefined
        ? projectedTable
        : applyOperation(projectedTable, command.optimistic),
    table,
  );
}

export function acknowledgePendingCommand(
  pending: Record<string, PendingTableCommand>,
  requestId: string,
  revision: number,
  seq: number,
  authoritativeRevision: number | undefined,
) {
  const command = pending[requestId];
  if (command === undefined) return pending;
  if (authoritativeRevision !== undefined && authoritativeRevision >= revision) {
    const nextPending = { ...pending };
    delete nextPending[requestId];
    return nextPending;
  }
  return {
    ...pending,
    [requestId]: {
      ...command,
      status: "accepted" as const,
      ackRevision: revision,
      ackSeq: seq,
    },
  };
}

export function reconcilePendingCommands(
  pending: Record<string, PendingTableCommand>,
  table: LiveTableProjection,
) {
  return Object.fromEntries(
    Object.entries(pending).filter(([, command]) => {
      if (
        command.ackRevision !== undefined &&
        table.revision >= command.ackRevision
      ) {
        return false;
      }
      if (command.optimistic === undefined) return true;
      if (operationVisible(table, command.optimistic)) return false;
      return (
        table.revision <= command.baseRevision ||
        canApplyOperation(table, command.optimistic)
      );
    }),
  );
}
