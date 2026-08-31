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
  const canClaim =
    !hasBot &&
    request === undefined &&
    game?.phase === "PLAY" &&
    game.currentTrick.plays.length === 0 &&
    table.viewerSeat !== undefined &&
    table.viewerSeat !== dummy;
  const remainingTricks =
    game === undefined ? 0 : 13 - game.completedTricks.length;
  if (request !== undefined) {
    const responseCommand =
      request.kind === "CLAIM" ? "game.respond_claim" : "game.respond_undo";
    return (
      <section className="consensus-controls" aria-live="polite">
        <strong>
          {request.kind === "CLAIM"
            ? `${request.requesterSeat} mengajukan ${request.claimTricks} trick`
            : `${request.requesterSeat} meminta undo`}
        </strong>
        <span>{request.approvedBy.length} persetujuan diterima</span>
        {!request.canRespond ? (
          <span>Menunggu pemain lain.</span>
        ) : (
          <div>
            <button
              className="primary-button"
              type="button"
              disabled={!canSendCommand(responseCommand, { accepted: true })}
              onClick={() => onCommand(responseCommand, { accepted: true })}
            >
              Terima
            </button>
            <button
              type="button"
              disabled={!canSendCommand(responseCommand, { accepted: false })}
              onClick={() => onCommand(responseCommand, { accepted: false })}
            >
              Tolak
            </button>
          </div>
        )}
      </section>
    );
  }
  if (!canClaim && !table.canRequestUndo) return null;
  return (
    <div className="consensus-controls consensus-actions">
      {canClaim ? (
        <details>
          <summary>Claim</summary>
          <div>
            {Array.from({ length: remainingTricks + 1 }, (_, _trickCount) => (
              <button
                type="button"
                key={_trickCount}
                disabled={!canSendCommand("game.request_claim", { tricks: _trickCount })}
                onClick={() =>
                  onCommand("game.request_claim", { tricks: _trickCount })
                }
              >
                {_trickCount}
              </button>
            ))}
          </div>
        </details>
      ) : null}
      {table.canRequestUndo ? (
        <button
          type="button"
          disabled={!canSendCommand("game.request_undo")}
          onClick={() => onCommand("game.request_undo")}
        >
          Undo aksi terakhir
        </button>
      ) : null}
    </div>
  );
}
