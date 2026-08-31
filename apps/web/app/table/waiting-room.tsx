import {
  type LiveTableProjection,
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
  session,
  commandDisabled,
  shareUrl,
  copied,
  onCopy,
  onLeaveTable,
}: {
  table: LiveTableProjection;
  orientation: TableOrientation;
  session: TableSession;
  commandDisabled: boolean;
  shareUrl: string;
  copied: boolean;
  onCopy: () => void;
  onLeaveTable: () => void;
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
        {session.inviteCode === null ? null : (
          <div className="invite-inline">
            <span>Kode undangan</span>
            <strong>{session.inviteCode}</strong>
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
              presence={session.tableState.presence}
              seat={seat}
              position={position}
              disabled={commandDisabled}
              turn={false}
              onCommand={session.sendCommand}
              {...(isOwner
                ? {
                    onRemove: (participantId: string) =>
                      session.sendCommand("table.remove_participant", {
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
                session.sendCommand("table.lock", { locked: !table.locked })
              }
            >
              {table.locked ? "Buka meja" : "Kunci meja"}
            </button>
            <button
              className="start-board-button"
              type="button"
              disabled={commandDisabled || !allReady}
              onClick={() => session.sendCommand("table.start_game")}
            >
              Mulai board
            </button>
            <button
              type="button"
              disabled={commandDisabled}
              onClick={() => session.sendCommand("table.finish")}
            >
              Akhiri meja
            </button>
          </>
        ) : null}
      </div>
    </div>
  );
}
