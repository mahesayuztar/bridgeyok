import { oppositeSeat, type LiveTableProjection } from "../table-state";
import type { TableSession } from "../use-table-session";

export function ConsensusControls({
  table,
  canSendCommand,
  onCommand,
}: {
  table: LiveTableProjection;
  canSendCommand: TableSession["canSendCommand"];
  onCommand: TableSession["sendCommand"];
}) {
  const game = table.game;
  const request = table.actionRequest;
  const dummy =
    game?.auction.contract === undefined
      ? undefined
      : oppositeSeat(game.auction.contract.declarer);
  const hasBot = table.participants.some((participant) => participant.isBot);
  const remainingTricks =
    game === undefined ? 0 : 13 - game.completedTricks.length;
  const claimAvailable =
    !hasBot &&
    request === undefined &&
    game?.phase === "PLAY" &&
    game.currentTrick.plays.length === 0 &&
    table.viewerSeat !== undefined &&
    table.viewerSeat !== dummy;
  const undoAvailable =
    !hasBot && request === undefined && table.canRequestUndo;

  return (
    <div className="consensus-navigation">
      <details className="claim-menu">
        <summary
          aria-label={
            claimAvailable ? "Ajukan claim" : "Claim tidak tersedia"
          }
          aria-disabled={!claimAvailable}
          onClick={(event) => {
            if (!claimAvailable) event.preventDefault();
          }}
        >
          <svg viewBox="0 0 24 24" aria-hidden="true">
            <path d="M7 11.5V6.75a1.25 1.25 0 0 1 2.5 0V10m0 0V5.25a1.25 1.25 0 0 1 2.5 0V10m0 0V6.25a1.25 1.25 0 0 1 2.5 0V10m0 0V7.25a1.25 1.25 0 0 1 2.5 0v5.5c0 4-2.25 6.25-6 6.25h-.5c-2.2 0-3.6-.8-4.8-2.6L4 13.8a1.4 1.4 0 0 1 2.2-1.7L8 14" />
          </svg>
          <span>Claim</span>
        </summary>
        {claimAvailable ? (
          <div
            className="claim-selector"
            role="group"
            aria-label="Jumlah trick yang diklaim"
          >
            <strong>Claim trick</strong>
            <div>
              {Array.from(
                { length: remainingTricks + 1 },
                (_, _trickCount) => (
                  <button
                    type="button"
                    key={_trickCount}
                    disabled={
                      !canSendCommand("game.request_claim", {
                        tricks: _trickCount,
                      })
                    }
                    onClick={(event) => {
                      const details = event.currentTarget.closest("details");
                      onCommand("game.request_claim", { tricks: _trickCount });
                      details?.removeAttribute("open");
                      details?.querySelector("summary")?.focus();
                    }}
                    aria-label={`Claim ${_trickCount} trick`}
                  >
                    {_trickCount}
                  </button>
                ),
              )}
            </div>
          </div>
        ) : null}
      </details>
      <button
        className="consensus-action"
        type="button"
        disabled={!undoAvailable || !canSendCommand("game.request_undo")}
        onClick={() => onCommand("game.request_undo")}
        aria-label={undoAvailable ? "Minta undo" : "Undo tidak tersedia"}
      >
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <path d="m9 7-5 5 5 5M5 12h8.5a5.5 5.5 0 1 1 0 11" />
        </svg>
        <span>Undo</span>
      </button>
      {request === undefined ? null : (
        <section className="consensus-request" aria-live="polite">
          <strong>
            {request.kind === "CLAIM"
              ? `${request.requesterSeat} · claim ${request.claimTricks}`
              : `${request.requesterSeat} · undo`}
          </strong>
          <span>{request.approvedBy.length} setuju</span>
          {request.canRespond ? (
            <div>
              <button
                className="primary-button"
                type="button"
                disabled={
                  !canSendCommand(
                    request.kind === "CLAIM"
                      ? "game.respond_claim"
                      : "game.respond_undo",
                    { accepted: true },
                  )
                }
                onClick={() =>
                  onCommand(
                    request.kind === "CLAIM"
                      ? "game.respond_claim"
                      : "game.respond_undo",
                    { accepted: true },
                  )
                }
              >
                Terima
              </button>
              <button
                type="button"
                disabled={
                  !canSendCommand(
                    request.kind === "CLAIM"
                      ? "game.respond_claim"
                      : "game.respond_undo",
                    { accepted: false },
                  )
                }
                onClick={() =>
                  onCommand(
                    request.kind === "CLAIM"
                      ? "game.respond_claim"
                      : "game.respond_undo",
                    { accepted: false },
                  )
                }
              >
                Tolak
              </button>
            </div>
          ) : (
            <span>Menunggu</span>
          )}
        </section>
      )}
    </div>
  );
}
