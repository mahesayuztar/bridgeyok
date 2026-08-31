import { useEffect, useId, useRef, useState } from "react";
import { createPortal } from "react-dom";
import {
  type LiveTableProjection,
  type ParticipantPresence,
  type Seat,
  type VisualPosition,
} from "../table-state";
import type { TableSession } from "../use-table-session";

function participantName(
  table: LiveTableProjection,
  participantId: string | undefined,
) {
  if (participantId === undefined) return null;
  return (
    table.participants.find((participant) => participant.id === participantId)
      ?.nickname ?? "Pemain"
  );
}

export function ParticipantPosition({
  table,
  presence,
  seat,
  position,
  disabled,
  turn,
  onCommand,
  onRemove,
  onLeaveTable,
}: {
  table: LiveTableProjection;
  presence: Record<string, ParticipantPresence>;
  seat: Seat;
  position: VisualPosition;
  disabled: boolean;
  turn: boolean;
  onCommand: TableSession["sendCommand"];
  onRemove?: (participantId: string) => void;
  onLeaveTable?: () => void;
}) {
  const dialogTitleId = useId();
  const closeButtonRef = useRef<HTMLButtonElement>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const [portalOpen, setPortalOpen] = useState(false);
  const assignment = table.seats[seat];
  const isViewer = table.viewerParticipantId === assignment?.participantId;
  const name = participantName(table, assignment?.participantId);
  const participant = table.participants.find(
    (candidate) => candidate.id === assignment?.participantId,
  );
  const isBot = participant?.isBot === true || assignment?.isBot === true;
  const participantPresence =
    assignment === undefined || isBot
      ? undefined
      : presence[assignment.participantId];
  const canTakeSeat =
    table.state === "WAITING" ||
    (table.viewerSeat === undefined &&
      (table.state === "ACTIVE" || table.state === "BETWEEN_BOARDS"));
  const canAddBot =
    assignment === undefined &&
    table.viewerRole === "OWNER" &&
    table.state !== "FINISHED";
  const canRemove =
    assignment !== undefined &&
    !isBot &&
    onRemove !== undefined &&
    (!isViewer || table.participants.length > 1);
  const canReplaceWithBot =
    assignment !== undefined &&
    !isBot &&
    !isViewer &&
    participant?.role === "PARTICIPANT" &&
    onRemove !== undefined;
  const canRemoveBot =
    assignment !== undefined && isBot && table.viewerRole === "OWNER";
  const canManageOwnSeat = isViewer && table.state === "WAITING";
  const hasPortalActions =
    (assignment === undefined && (canTakeSeat || canAddBot)) ||
    canManageOwnSeat ||
    canRemove ||
    canReplaceWithBot ||
    canRemoveBot ||
    (isViewer && onLeaveTable !== undefined);

  useEffect(() => {
    if (!portalOpen) return;
    const trigger = triggerRef.current;
    closeButtonRef.current?.focus();
    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") setPortalOpen(false);
    }
    window.addEventListener("keydown", handleKeyDown);
    return () => {
      window.removeEventListener("keydown", handleKeyDown);
      trigger?.focus();
    };
  }, [portalOpen]);

  const crown =
    participant?.role === "OWNER" ? (
      <svg
        className="owner-crown"
        viewBox="0 0 24 24"
        role="img"
        aria-label="Pemilik meja"
      >
        <path d="M3 7.5 7.5 12 12 5l4.5 7L21 7.5l-2 10H5l-2-10Zm3 12h12" />
      </svg>
    ) : null;
  const portal =
    portalOpen && typeof document !== "undefined"
      ? createPortal(
          <div
            className="participant-portal-layer"
            role="presentation"
            onMouseDown={() => setPortalOpen(false)}
          >
            <section
              className="participant-portal"
              role="dialog"
              aria-modal="true"
              aria-labelledby={dialogTitleId}
              onMouseDown={(event) => event.stopPropagation()}
            >
              <header>
                <span className="participant-portal-seat">{seat}</span>
                <div>
                  <h2 id={dialogTitleId}>{name ?? "Kursi kosong"}</h2>
                  {assignment === undefined ? (
                    <p>Pilih siapa yang mengisi kursi ini.</p>
                  ) : table.state === "WAITING" ? (
                    <p>
                      {isBot ? "Bot · " : ""}
                      {assignment.ready ? "Siap" : "Belum siap"}
                    </p>
                  ) : null}
                </div>
                {crown}
                <button
                  ref={closeButtonRef}
                  className="participant-portal-close"
                  type="button"
                  onClick={() => setPortalOpen(false)}
                  aria-label="Tutup menu pemain"
                >
                  ×
                </button>
              </header>
              <div className="participant-portal-actions">
                {assignment !== undefined || !canTakeSeat ? null : (
                  <button
                    type="button"
                    disabled={disabled}
                    onClick={() => {
                      setPortalOpen(false);
                      onCommand("table.take_seat", { seat });
                    }}
                  >
                    Duduk di kursi
                  </button>
                )}
                {!canAddBot ? null : (
                  <button
                    type="button"
                    disabled={disabled}
                    onClick={() => {
                      setPortalOpen(false);
                      onCommand("table.add_bot", { seat });
                    }}
                  >
                    Tambah bot
                  </button>
                )}
                {!canManageOwnSeat ? null : (
                  <button
                    type="button"
                    disabled={disabled}
                    onClick={() => {
                      setPortalOpen(false);
                      onCommand("table.set_ready", {
                        ready: !assignment.ready,
                      });
                    }}
                  >
                    {assignment.ready ? "Batalkan siap" : "Saya siap"}
                  </button>
                )}
                {!canManageOwnSeat ? null : (
                  <button
                    type="button"
                    disabled={disabled}
                    onClick={() => {
                      setPortalOpen(false);
                      onCommand("table.leave_seat");
                    }}
                  >
                    Berdiri dari kursi
                  </button>
                )}
                {!canRemove ? null : (
                  <button
                    type="button"
                    disabled={disabled}
                    onClick={() => {
                      setPortalOpen(false);
                      onRemove(assignment.participantId);
                    }}
                  >
                    {isViewer ? "Serahkan & keluar" : "Keluarkan"}
                  </button>
                )}
                {!canReplaceWithBot ? null : (
                  <button
                    type="button"
                    disabled={disabled}
                    onClick={() => {
                      setPortalOpen(false);
                      onCommand("table.replace_with_bot", {
                        participant_id: assignment.participantId,
                      });
                    }}
                  >
                    Keluarkan &amp; ganti bot
                  </button>
                )}
                {!canRemoveBot ? null : (
                  <button
                    type="button"
                    disabled={disabled}
                    onClick={() => {
                      setPortalOpen(false);
                      onCommand("table.remove_bot", { seat });
                    }}
                  >
                    Keluarkan bot
                  </button>
                )}
                {!isViewer || onLeaveTable === undefined ? null : (
                  <button
                    type="button"
                    disabled={disabled}
                    onClick={() => {
                      setPortalOpen(false);
                      onLeaveTable();
                    }}
                  >
                    Tinggalkan meja
                  </button>
                )}
                {hasPortalActions ? null : (
                  <p>Belum ada tindakan untuk pemain ini.</p>
                )}
              </div>
            </section>
          </div>,
          document.body,
        )
      : null;

  return (
    <div
      className={`player-position player-${position}`}
      data-occupied={assignment !== undefined}
      data-turn={turn}
      data-presence={
        isBot
          ? "bot"
          : participantPresence === undefined
            ? "unknown"
            : participantPresence.online
              ? "online"
              : "offline"
      }
    >
      {assignment === undefined ? (
        <button
          ref={triggerRef}
          className="empty-seat-trigger"
          type="button"
          onClick={() => setPortalOpen(true)}
          aria-label={`Buka menu kursi kosong ${seat}`}
        >
          <span className="player-seat">{seat}</span>
          <span>Kosong</span>
        </button>
      ) : (
        <button
          ref={triggerRef}
          className="player-trigger"
          type="button"
          onClick={() => setPortalOpen(true)}
          aria-label={`Buka menu ${name}, kursi ${seat}`}
        >
          <span className="player-seat">{seat}</span>
          <span className="player-copy">
            <strong>
              {name}
              {crown}
            </strong>
            {table.state === "WAITING" ? (
              <span>
                {isBot ? "Bot · " : ""}
                {assignment.ready ? "Siap" : "Belum siap"}
              </span>
            ) : null}
            <span className="sr-only">
              {isBot
                ? "Bot"
                : participantPresence?.online === false
                  ? "Tidak tersambung"
                  : "Tersambung"}
              {turn ? ", sedang bermain" : ""}
            </span>
          </span>
        </button>
      )}
      {portal}
    </div>
  );
}
