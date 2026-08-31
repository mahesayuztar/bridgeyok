import {
  type LiveTableProjection,
  type ParticipantPresence,
  type Seat,
  type TableOrientation,
  type VisualPosition,
} from "../table-state";
import type { TableSession } from "../use-table-session";
import { ParticipantPosition } from "./participant-position";

const seats: Seat[] = ["N", "E", "S", "W"];

export function WaitingRoom({
  table,
  orientation,
  presence,
  inviteCode,
  commandDisabled,
  shareUrl,
  copied,
  onCopy,
  onLeaveTable,
  onCommand,
}: {
  table: LiveTableProjection;
  orientation: TableOrientation;
  presence: Record<string, ParticipantPresence>;
  inviteCode: string | null;
  commandDisabled: boolean;
  shareUrl: string;
  copied: boolean;
  onCopy: () => void;
  onLeaveTable: () => void;
  onCommand: TableSession["sendCommand"];
}) {
  const allReady = seats.every((seat) => table.seats[seat]?.ready);
  const isOwner = table.viewerRole === "OWNER";
  return (
    <div className="waiting-room">
      <div className="waiting-copy">
        <p className="eyebrow">Ruang tunggu</p>
        <h1>Susun empat kursi.</h1>
        <p>
          {table.participants.length}/4 pemain sudah masuk. Pilih kursi, tandai
          siap, lalu pemilik dapat memulai board.
        </p>
        {inviteCode === null ? null : (
          <div className="invite-inline">
            <span>Kode undangan</span>
            <strong>{inviteCode}</strong>
            <button type="button" onClick={onCopy}>
              {copied ? "Tautan disalin" : "Salin tautan"}
            </button>
            <span className="sr-only">{shareUrl}</span>
          </div>
        )}
      </div>
      <div className="waiting-table table-surface">
        {(Object.entries(orientation) as Array<[VisualPosition, Seat]>).map(
          ([position, seat]) => (
            <ParticipantPosition
              key={seat}
              table={table}
              presence={presence}
              seat={seat}
              position={position}
              disabled={commandDisabled}
              turn={false}
              onCommand={onCommand}
              {...(isOwner
                ? {
                    onRemove: (participantId: string) =>
                      onCommand("table.remove_participant", {
                        participant_id: participantId,
                      }),
                  }
                : {})}
              {...(table.viewerParticipantId ===
                table.seats[seat]?.participantId && !isOwner
                ? { onLeaveTable }
                : {})}
            />
          ),
        )}
        <div className="waiting-center">
          <strong>{table.locked ? "Meja dikunci" : "Meja terbuka"}</strong>
          <span>
            {seats.filter((seat) => table.seats[seat]?.ready).length}/4 siap
          </span>
        </div>
      </div>
      <div className="waiting-controls">
        {table.viewerSeat === undefined ? (
          <p>Pilih kursi kosong pada meja.</p>
        ) : null}
        {isOwner ? (
          <>
            <button
              type="button"
              disabled={commandDisabled}
              onClick={() =>
                onCommand("table.lock", { locked: !table.locked })
              }
            >
              {table.locked ? "Buka meja" : "Kunci meja"}
            </button>
            <button
              className="start-board-button"
              type="button"
              disabled={commandDisabled || !allReady}
              onClick={() => onCommand("table.start_game")}
            >
              Mulai board
            </button>
            <button
              type="button"
              disabled={commandDisabled}
              onClick={() => onCommand("table.finish")}
            >
              Akhiri meja
            </button>
          </>
        ) : null}
      </div>
    </div>
  );
}
