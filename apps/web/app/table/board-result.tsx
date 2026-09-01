"use client";

import { useEffect, useState } from "react";
import { boardResultLabel, type LiveTableProjection } from "../table-state";
import type { TableSession } from "../use-table-session";
import { contractSummaryLabel } from "./gameplay-presentation";

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

  useEffect(() => {
    if (result === undefined || hidden) return;
    let fadeTimer: ReturnType<typeof setTimeout> | null = null;
    let dismissalStarted = false;

    function dismissResult(event?: MouseEvent) {
      if (event?.defaultPrevented || dismissalStarted) return;
      dismissalStarted = true;
      setExiting(true);
      fadeTimer = setTimeout(() => {
        setHidden(true);
        if (canSendCommand("table.next_board")) {
          onCommand("table.next_board");
        }
      }, RESULT_FADE_DURATION);
    }

    const visibilityTimer = setTimeout(dismissResult, RESULT_VISIBLE_DURATION);
    document.addEventListener("click", dismissResult);
    return () => {
      clearTimeout(visibilityTimer);
      if (fadeTimer !== null) clearTimeout(fadeTimer);
      document.removeEventListener("click", dismissResult);
    };
  }, [canSendCommand, hidden, onCommand, result, table.game?.board.number]);

  if (result === undefined) return null;
  if (hidden) return null;
  return (
    <section
      className="board-result"
      aria-labelledby="result-title"
      data-exiting={exiting}
    >
      <p>Board {table.game?.board.number} selesai</p>
      <h2 id="result-title">
        {result.passedOut
          ? "Passed out"
          : contractSummaryLabel(result.contract)}
      </h2>
      <strong className="result-outcome">{boardResultLabel(result)}</strong>
      <dl>
        <div>
          <dt>Trick</dt>
          <dd>{result.tricksDeclarer}</dd>
        </div>
        <div>
          <dt>NS</dt>
          <dd>
            {result.scoreNS > 0 ? "+" : ""}
            {result.scoreNS}
          </dd>
        </div>
        <div>
          <dt>EW</dt>
          <dd>
            {result.scoreNS < 0 ? "+" : ""}
            {-result.scoreNS}
          </dd>
        </div>
      </dl>
      {table.viewerRole === "OWNER" && table.state === "BETWEEN_BOARDS" ? (
        <div className="result-actions">
          <button
            type="button"
            disabled={!canSendCommand("table.finish")}
            onClick={(event) => {
              event.preventDefault();
              onCommand("table.finish");
            }}
          >
            Akhiri meja
          </button>
        </div>
      ) : null}
    </section>
  );
}
