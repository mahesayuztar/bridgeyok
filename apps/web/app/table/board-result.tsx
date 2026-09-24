"use client";

import { useEffect, useEffectEvent } from "react";
import { boardResultLabel, type LiveTableProjection } from "../table-state";
import type { TableSession } from "../use-table-session";
import {
  compactContractLabel,
  contractScoreLabel,
} from "./gameplay-presentation";

export function BoardResult({
  table,
  canSendCommand,
  onCommand,
  autoAdvance = true,
}: {
  table: LiveTableProjection;
  autoAdvance?: boolean;
  canSendCommand: TableSession["canSendCommand"];
  onCommand: TableSession["sendCommand"];
}) {
  const result = table.game?.result;
  const advanceBoard = useEffectEvent(() => {
    if (canSendCommand("table.next_board")) onCommand("table.next_board");
  });

  useEffect(() => {
    if (
      result === undefined ||
      !autoAdvance ||
      table.viewerRole !== "OWNER"
    ) {
      return;
    }
    const timeout = window.setTimeout(advanceBoard, 5_000);
    return () => window.clearTimeout(timeout);
  }, [autoAdvance, result, table.boardId, table.viewerRole]);

  if (result === undefined) return null;

  return (
    <section
      className="board-result"
      aria-label={`Hasil board ${table.game?.board.number}`}
    >
      <dl>
        <div>
          <dt>Board</dt>
          <dd>{table.game?.board.number}</dd>
        </div>
        <div>
          <dt>Contract</dt>
          <dd>{compactContractLabel(result.contract)}</dd>
        </div>
        <div>
          <dt>Result</dt>
          <dd>{boardResultLabel(result)}</dd>
        </div>
        <div>
          <dt>Score</dt>
          <dd>{contractScoreLabel(result)}</dd>
        </div>
      </dl>
    </section>
  );
}
