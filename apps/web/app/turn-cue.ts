import {
  oppositeSeat,
  type GameProjection,
  type LiveTableProjection,
  type Seat,
} from "./table-state.ts";

export const TURN_CUE_PEAK_GAIN = 0.18;
export const TURN_CUE_DURATION_SECONDS = 0.22;

export type TurnCueState = {
  tableId: string;
  boardNumber: number;
  phase: GameProjection["phase"];
  turn: Seat | undefined;
  viewerTurn: boolean;
  revision: number;
};

export function turnCueState(table: LiveTableProjection): TurnCueState | null {
  if (table.game === undefined) return null;
  const { game, viewerSeat } = table;
  const declarer = game.auction.contract?.declarer;
  const dummy = declarer === undefined ? undefined : oppositeSeat(declarer);
  const viewerTurn =
    game.phase === "AUCTION"
      ? game.turn === viewerSeat
      : game.phase === "OPENING_LEAD" || game.phase === "PLAY"
        ? viewerSeat !== dummy &&
          (game.turn === viewerSeat ||
            (viewerSeat === declarer && game.turn === dummy))
        : false;
  return {
    tableId: table.tableId,
    boardNumber: game.board.number,
    phase: game.phase,
    turn: game.turn,
    viewerTurn,
    revision: table.revision,
  };
}

export function shouldPlayTurnCue(
  previous: TurnCueState | null,
  current: TurnCueState | null,
) {
  if (previous === null || current === null) return false;
  if (current.phase !== "AUCTION" && current.phase !== "OPENING_LEAD" && current.phase !== "PLAY") {
    return false;
  }
  const sameSnapshot =
    previous.tableId === current.tableId &&
    previous.boardNumber === current.boardNumber &&
    previous.phase === current.phase &&
    previous.turn === current.turn &&
    previous.revision === current.revision;
  return !sameSnapshot && !previous.viewerTurn && current.viewerTurn;
}
