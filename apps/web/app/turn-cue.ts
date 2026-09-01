import type { GameProjection, LiveTableProjection, Seat } from "./table-state.ts";

export const TURN_CUE_PEAK_GAIN = 0.18;
export const TURN_CUE_DURATION_SECONDS = 0.22;

export type TurnCueState = {
  tableId: string;
  boardNumber: number;
  phase: GameProjection["phase"];
  turn: Seat | undefined;
  viewerSeat: Seat | undefined;
  revision: number;
};

export function turnCueState(table: LiveTableProjection): TurnCueState | null {
  if (table.game === undefined) return null;
  return {
    tableId: table.tableId,
    boardNumber: table.game.board.number,
    phase: table.game.phase,
    turn: table.game.turn,
    viewerSeat: table.viewerSeat,
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
  const wasViewerTurn = previous.turn === previous.viewerSeat;
  const isViewerTurn = current.turn === current.viewerSeat;
  return !sameSnapshot && !wasViewerTurn && isViewerTurn;
}
