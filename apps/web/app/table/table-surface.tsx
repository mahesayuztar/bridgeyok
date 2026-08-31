import type { ReactNode } from "react";
import {
  type LiveTableProjection,
  type Seat,
  type TableOrientation,
  type VisualPosition,
} from "../table-state";
import type { TableSession } from "../use-table-session";
import { ParticipantPosition } from "./participant-position";

export function TableSurface({
  table,
  orientation,
  session,
  commandDisabled,
  children,
}: {
  table: LiveTableProjection;
  orientation: TableOrientation;
  session: TableSession;
  commandDisabled: boolean;
  children: ReactNode;
}) {
  const isOwner = table.viewerRole === "OWNER";
  return (
    <div className="table-surface">
      {(Object.entries(orientation) as Array<[VisualPosition, Seat]>).map(
        ([position, seat]) => (
          <ParticipantPosition
            key={seat}
            table={table}
            presence={session.tableState.presence}
            seat={seat}
            position={position}
            disabled={commandDisabled}
            turn={table.game?.turn === seat}
            onCommand={session.sendCommand}
            {...(isOwner
              ? {
                  onRemove: (participantId: string) =>
                    session.sendCommand("table.remove_participant", {
                      participant_id: participantId,
                    }),
                }
              : {})}
          />
        ),
      )}
      {children}
    </div>
  );
}
