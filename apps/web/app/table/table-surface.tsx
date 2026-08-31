import type { ReactNode } from "react";
import {
  type LiveTableProjection,
  type ParticipantPresence,
  type Seat,
  type TableOrientation,
  type VisualPosition,
} from "../table-state";
import type { TableSession } from "../use-table-session";
import { ParticipantPosition } from "./participant-position";

export function TableSurface({
  table,
  orientation,
  presence,
  commandDisabled,
  onCommand,
  children,
}: {
  table: LiveTableProjection;
  orientation: TableOrientation;
  presence: Record<string, ParticipantPresence>;
  commandDisabled: boolean;
  onCommand: TableSession["sendCommand"];
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
            presence={presence}
            seat={seat}
            position={position}
            disabled={commandDisabled}
            turn={table.game?.turn === seat}
            onCommand={onCommand}
            {...(isOwner
              ? {
                  onRemove: (participantId: string) =>
                    onCommand("table.remove_participant", {
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
