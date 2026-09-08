import { useDialogDrag } from "./use-dialog-drag";
import { useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
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
import { BridgeHand } from "./playing-card";
import { TableSurface } from "./table-surface";

const orientation: TableOrientation = {
  top: "N",
  right: "E",
  bottom: "S",
  left: "W",
};

export function BoardReplayModal({
  table,
  entry,
  loadBoardReplay,
  onClose,
}: {
  table: LiveTableProjection;
  entry: LiveTableProjection["scoreSheet"][number];
  loadBoardReplay: TableSession["loadBoardReplay"];
  onClose: () => void;
}) {
  const dialogDrag = useDialogDrag();
  const dialogRef = useRef<HTMLDialogElement>(null);
  const [replay, setReplay] = useState<BoardReplay | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [attempt, setAttempt] = useState(0);
  const [step, setStep] = useState(0);

  useEffect(() => {
    const dialog = dialogRef.current;
    const previousFocus = document.activeElement;
    dialog?.showModal();
    return () => {
      dialog?.close();
      if (previousFocus instanceof HTMLElement && previousFocus.isConnected)
        previousFocus.focus();
    };
  }, []);

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
  const replayTable: LiveTableProjection = {
    ...table,
    boardId: entry.boardId,
    boardNumber: entry.boardNumber,
    ...(game === undefined ? {} : { game }),
  };
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
        if (event.key === "ArrowLeft" || event.key === "ArrowRight") {
          event.preventDefault();
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
          <div className="replay-surface-wrap">
            <TableSurface
              table={replayTable}
              orientation={orientation}
              presence={{}}
              canSendCommand={() => false}
              onCommand={() => {}}
              seatLabels={seatLabels}
            >
              {(
                Object.entries(orientation) as Array<
                  [keyof TableOrientation, Seat]
                >
              ).map(([position, seat]) => (
                <div
                  key={seat}
                  className={`replay-hand replay-hand-${position}`}
                  data-seat={seat}
                >
                  <BridgeHand
                    cards={frame.hands[seat]}
                    title={`Kartu ${seat}`}
                    variant="dummy"
                    contractStrain={game.auction.contract?.strain}
                    position={position}
                  />
                </div>
              ))}
              <div className="replay-center">
                {step === 0 ? <AuctionTable game={game} /> : null}
                {frame.trick === undefined ? null : (
                  <CurrentTrick
                    trick={frame.trick}
                    orientation={orientation}
                    stage="winner"
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
              </div>
            </TableSurface>
          </div>
          <nav className="replay-navigation" aria-label="Navigasi replay">
            <button
              type="button"
              aria-label="Trick sebelumnya"
              disabled={step === 0}
              onClick={() => setStep((current) => current - 1)}
            >
              ←
            </button>
            <span
              role="status"
              aria-label={frame.showResult ? "Hasil board" : `Trick ${step}`}
            >
              {Math.min(step, frame.lastStep - 1)} / {frame.lastStep - 1}
            </span>
            <button
              type="button"
              aria-label="Trick berikutnya"
              disabled={frame.showResult}
              onClick={() => setStep((current) => current + 1)}
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
