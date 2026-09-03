"use client";

import { useEffect, useEffectEvent, useState } from "react";
import { boardResultLabel, type LiveTableProjection } from "../table-state";
import type { TableSession } from "../use-table-session";
import {
  compactContractLabel,
  contractScoreLabel,
} from "./gameplay-presentation";

const RESULT_VISIBLE_DURATION = 5_000;
const RESULT_FADE_DURATION = 180;

export function BoardResult({
  table,
  canSendCommand,
  onCommand,
}: {
  table: LiveTableProjection;
  canSendCommand: TableSession["canSendCommand"];
  onCommand: TableSession["sendCommand"];
}) {
  const result = table.game?.result;
  const [exiting, setExiting] = useState(false);
  const [hidden, setHidden] = useState(false);
  const advanceBoard = useEffectEvent(() => {
    if (canSendCommand("table.next_board")) {
      onCommand("table.next_board");
    }
  });
  const resultKey = result === undefined ? null : table.boardId;

  useEffect(() => {
    if (resultKey === null) return;
    let fadeTimer: ReturnType<typeof setTimeout> | null = null;
    let dismissalStarted = false;

    function dismissResult(event?: MouseEvent) {
      if (
        event?.defaultPrevented ||
        (event?.target instanceof Element &&
          event.target.closest("button, a, input, select, textarea, summary, [role='dialog']") !==
            null) ||
        dismissalStarted
      )
        return;
      dismissalStarted = true;
      setExiting(true);
      fadeTimer = setTimeout(() => {
        setHidden(true);
        advanceBoard();
      }, RESULT_FADE_DURATION);
    }

    const visibilityTimer = setTimeout(dismissResult, RESULT_VISIBLE_DURATION);
    document.addEventListener("click", dismissResult);
    return () => {
      clearTimeout(visibilityTimer);
      if (fadeTimer !== null) clearTimeout(fadeTimer);
      document.removeEventListener("click", dismissResult);
    };
  }, [resultKey]);

  if (result === undefined) return null;
  if (hidden) return null;
  return (
    <section
      className="board-result"
      aria-label={`Hasil board ${table.game?.board.number}`}
      data-exiting={exiting}
    >
      <dl>
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
