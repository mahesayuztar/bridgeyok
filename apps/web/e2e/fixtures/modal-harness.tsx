import { useState } from "react";
import { createRoot } from "react-dom/client";
import { ActiveTableStatusBar } from "../../app/table/table-status-bar";
import { ParticipantPosition } from "../../app/table/participant-position";
import { CompletedDeal } from "../../app/table/completed-deal";
import { TableSurface } from "../../app/table/table-surface";
import { tableOrientation, type LiveTableProjection } from "../../app/table-state";

const table = {
  tableId: "drag-table", boardId: "drag-board", state: "ACTIVE", viewerSeat: "S", viewerRole: "OWNER", viewerParticipantId: "south",
  participants: [{ id: "south", nickname: "South", role: "OWNER" }], seats: { N: { participantId: "north" }, E: { participantId: "east" }, S: { participantId: "south" }, W: { participantId: "west" } }, scoreSheet: [],
  game: { board: { number: 1, dealer: "N", vulnerability: "NONE" }, phase: "PLAY", auction: { contract: { level: 1, strain: "NT", declarer: "S", doubling: "UNDOUBLED" }, calls: [] },
    currentTrick: { plays: [] }, completedTrickCount: 1, tricksNS: 1, tricksEW: 0, completedTricks: [{ winner: "N", plays: ["N", "E", "S", "W"].map((seat) => ({ seat, card: { suit: "S", rank: "A" } })) }] },
} as unknown as LiveTableProjection;
const idle = () => {};
const unavailable = () => false;

export function renderPreview(fullDeal: NonNullable<NonNullable<LiveTableProjection["game"]>["fullDeal"]>, width: number, height: number) {
  const container = document.createElement("div");
  container.id = "canonical-preview";
  container.className = "active-table-client";
  container.style.cssText = `position:fixed;left:-10000px;width:${width}px;height:${height}px;display:block`;
  document.body.append(container);
  createRoot(container).render(<TableSurface table={table} orientation={tableOrientation()} presence={{}} canSendCommand={unavailable} onCommand={idle} seatLabels={{ N: "N", E: "E", S: "S", W: "W" }}><CompletedDeal game={{ ...table.game!, phase: "BOARD_SCORED", fullDeal }} orientation={tableOrientation()} /></TableSurface>);
}

function DialogHarness() {
  const [request, setRequest] = useState<LiveTableProjection["actionRequest"]>(undefined);
  const activeTable = { ...table, ...(request ? { actionRequest: request } : {}) };
  return <main className="active-table-client">
    <ActiveTableStatusBar table={activeTable} connectionState="connected" inviteCode={null} canSendCommand={() => true} onCommand={() => setRequest(undefined)} analysisControl={null} soundMuted onSoundMutedChange={idle} onLeaveTable={idle} loadBoardReplay={async () => { throw new Error("Unused"); }} loadPositionAnalysis={async () => { throw new Error("Unused"); }} />
    <ParticipantPosition table={table} presence={{}} seat="S" position="bottom" canSendCommand={unavailable} onCommand={idle} turn={false} />
    <button onClick={() => setRequest({ kind: "CLAIM", requesterSeat: "N", claimTricks: 4, approvedBy: [], canRespond: true } as unknown as NonNullable<LiveTableProjection["actionRequest"]>)}>Receive claim</button>
  </main>;
}
if (!document.getElementById("root")!.hasChildNodes()) createRoot(document.getElementById("root")!).render(<DialogHarness />);
