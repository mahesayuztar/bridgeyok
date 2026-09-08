import { useDialogDrag } from "./use-dialog-drag";
import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { playableHand, tableOrientation } from "../table-state";
import { usePositionAnalysis } from "../use-position-analysis";
import { replayFrame, type BoardReplay } from "../board-replay";
import type {
  LiveTableProjection,
  Seat,
  TableOrientation,
} from "../table-state";
import type { TableSession } from "../use-table-session";
import { AuctionTable } from "./auction-controls";
import { BoardResult } from "./board-result";
import { CurrentTrick } from "./current-trick";
import { CompletedDeal } from "./completed-deal";
import { TableSurface } from "./table-surface";

export function BoardReplayModal({
  table,
  entry,
  loadBoardReplay,
  loadPositionAnalysis,
  onClose,
}: {
  table: LiveTableProjection;
  entry: LiveTableProjection["scoreSheet"][number];
  loadBoardReplay: TableSession["loadBoardReplay"];
  loadPositionAnalysis: TableSession["loadPositionAnalysis"];
  onClose: () => void;
}) {
  const dialogDrag = useDialogDrag();
  const orientation = tableOrientation(table.viewerSeat);
  const surfaceWrapRef = useRef<HTMLDivElement>(null);
  const dialogRef = useRef<HTMLDialogElement>(null);
  const [replay, setReplay] = useState<BoardReplay | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [attempt, setAttempt] = useState(0);
  const [doubleDummy, setDoubleDummy] = useState(false);
  const [step, setStep] = useState(0);

  useEffect(() => {
    const dialog = dialogRef.current;
    const previousFocus = document.activeElement;
    const media = window.matchMedia("(max-width: 48rem)");
    function showDialog() {
      dialog?.close();
      if (media.matches) dialog?.show();
      else dialog?.showModal();
    }
    showDialog();
    media.addEventListener("change", showDialog);
    return () => {
      media.removeEventListener("change", showDialog);
      dialog?.close();
      if (previousFocus instanceof HTMLElement && previousFocus.isConnected)
        previousFocus.focus();
    };
  }, []);

  useLayoutEffect(() => {
    const wrap = surfaceWrapRef.current;
    if (!wrap) return;
    function fitPreview() {
      if (!wrap) return;
      const width = Math.max(1000, window.innerWidth);
      const height = Math.max(650, width * 0.56);
      const scale = wrap.clientWidth / width;
      wrap.style.height = `${height * scale}px`;
      const surface = wrap.firstElementChild as HTMLElement | null;
      if (!surface) return;
      surface.style.width = `${width}px`;
      surface.style.height = `${height}px`;
      surface.style.transform = `scale(${scale})`;
    }
    const observer = new ResizeObserver(fitPreview);
    observer.observe(wrap);
    window.addEventListener("resize", fitPreview);
    fitPreview();
    return () => {
      observer.disconnect();
      window.removeEventListener("resize", fitPreview);
    };
  }, [replay]);

  useEffect(() => {
    const controller = new AbortController();
    loadBoardReplay(entry.boardId, controller.signal)
      .then((record) => {
        if (controller.signal.aborted) return;
        if (
          record.boardId !== entry.boardId ||
          record.game.phase !== "BOARD_SCORED" ||
          !record.game.result
        ) {
          throw new Error("Invalid replay");
        }
        setReplay(record);
      })
      .catch(() => {
        if (!controller.signal.aborted) setError("Replay tidak tersedia.");
      });
    return () => controller.abort();
  }, [entry.boardId, loadBoardReplay, attempt]);

  const frame = replay === null ? null : replayFrame(replay, step);
  const game = replay?.game;
  const analysis = usePositionAnalysis(loadPositionAnalysis, entry.boardId, String(step), doubleDummy && frame !== null && !frame.showResult && frame.turn !== undefined, step);
  const replayTable: LiveTableProjection = {
    ...table,
    boardId: entry.boardId,
    boardNumber: entry.boardNumber,
    ...(game === undefined ? {} : { game: frame?.game ?? game }),
  };
  const legalCards = frame?.turn && game ? playableHand({ ...replayTable, viewerSeat: frame.turn, game: { ...game, turn: frame.turn, currentTrick: frame.currentTrick, ownHand: frame.hands[frame.turn] } })?.hand ?? [] : [];
  const seatLabels = Object.fromEntries(
    (Object.keys(orientation) as Array<keyof TableOrientation>).map(
      (position) => {
        const seat = orientation[position];
        return [seat, entry.lineup.seats[seat]?.nickname ?? seat];
      },
    ),
  ) as Record<Seat, string>;

  function closeReplay() {
    dialogRef.current?.close();
    onClose();
  }

  return createPortal(
    <dialog
      ref={dialogRef}
      className="board-replay-modal"
      aria-labelledby="board-replay-title"
      onCancel={(event) => {
        event.preventDefault();
        closeReplay();
      }}
      onKeyDown={(event) => {
        if (event.key === "Escape") { event.preventDefault(); closeReplay(); }
        if (event.key === "ArrowLeft" || event.key === "ArrowRight") {
          event.preventDefault();
          event.currentTarget.querySelector<HTMLElement>(".replay-navigation")?.focus();
          if (frame !== null)
            setStep((current) =>
              Math.max(
                0,
                Math.min(
                  frame.lastStep,
                  current + (event.key === "ArrowRight" ? 1 : -1),
                ),
              ),
            );
        }
      }}
    >
      <header {...dialogDrag} className="score-sheet-header">
        <h2 id="board-replay-title">Replay board {entry.boardNumber}</h2>
        <button type="button" aria-pressed={doubleDummy} onClick={() => setDoubleDummy((value) => !value)} title="Predicted total tricks pasangan yang sedang turn">DD {doubleDummy ? "ON" : "OFF"}</button>
        <button
          type="button"
          className="score-sheet-close"
          aria-label="Tutup replay"
          onClick={closeReplay}
        >
          ×
        </button>
      </header>
      {error !== null ? (
        <div className="replay-message" role="alert">
          <p>{error}</p>
          <button
            type="button"
            onClick={() => {
              setError(null);
              setAttempt((current) => current + 1);
            }}
          >
            Coba lagi
          </button>
        </div>
      ) : frame === null || game === undefined ? (
        <p className="replay-message" role="status">
          Memuat…
        </p>
      ) : (
        <>
          {analysis.failed ? <p role="status">DDS tidak tersedia untuk posisi ini.</p> : null}
          <div ref={surfaceWrapRef} className="replay-surface-wrap" data-turn={frame.turn} data-dummy-revealed={frame.dummyRevealed}>
            <TableSurface
              table={replayTable}
              orientation={orientation}
              presence={{}}
              canSendCommand={() => false}
              onCommand={() => {}}
              seatLabels={seatLabels}
            >
              <CompletedDeal
                game={game}
                orientation={orientation}
                hands={frame.hands}
                turn={frame.turn}
                playableCards={legalCards}
                predictions={analysis.result?.cards}
                analysisPending={analysis.pending}
              />
                {step === 0 ? <AuctionTable game={game} /> : null}
                {frame.trick === undefined ? null : (
                  <CurrentTrick
                    trick={frame.trick}
                    orientation={orientation}
                    stage={frame.trick.winner ? "winner" : "idle"}
                  />
                )}
                {frame.showResult ? (
                  <BoardResult
                    table={replayTable}
                    canSendCommand={() => false}
                    onCommand={() => {}}
                    persistent
                  />
                ) : null}
            </TableSurface>
          </div>
          <nav className="replay-navigation" aria-label="Navigasi replay" tabIndex={-1}>
            <button
              type="button"
              aria-label="Kartu sebelumnya"
              disabled={step === 0}
              onClick={(event) => {
                if (step === 1) event.currentTarget.closest("nav")?.focus();
                setStep((current) => current - 1);
              }}
            >
              ←
            </button>
            <span
              role="status"
              aria-label={frame.showResult ? "Hasil board" : `Kartu ${step}`}
            >
              {Math.min(step, frame.lastStep - 1)} / {frame.lastStep - 1}
            </span>
            <button
              type="button"
              aria-label="Kartu berikutnya"
              disabled={frame.showResult}
              onClick={(event) => {
                if (step + 1 === frame.lastStep) event.currentTarget.closest("nav")?.focus();
                setStep((current) => current + 1);
              }}
            >
              →
            </button>
          </nav>
        </>
      )}
    </dialog>,
    document.body,
  );
}
