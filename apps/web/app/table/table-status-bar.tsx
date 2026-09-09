import { TableInvite } from "./table-social";
import { useDialogDrag } from "./use-dialog-drag";
import { useRef, type ReactNode } from "react";
import type { LiveTableProjection } from "../table-state";
import type { TableSession } from "../use-table-session";
import { ConsensusControls } from "./consensus-controls";
import { AuctionTable } from "./auction-controls";
import { contractLabel } from "./gameplay-presentation";
import { ScoreSheet } from "./score-sheet";
import { TrickIndicator } from "./trick-indicator";

const vulnerabilityLabels = {
  NONE: "Tidak ada",
  NS: "NS",
  EW: "EW",
  BOTH: "Keduanya",
};
const connectionLabels = {
  idle: "Belum terhubung",
  connecting: "Menghubungkan",
  syncing: "Menyelaraskan",
  connected: "Terhubung",
  degraded: "Koneksi terganggu",
  offline: "Offline",
};

export function WaitingTableStatusBar({
  inviteCode,
  table,
  connectionState,
  onLeaveTable,
  loadBoardReplay,
  loadPositionAnalysis,
}: {
  inviteCode: string | null;
  table: LiveTableProjection;
  connectionState: TableSession["connectionState"];
  onLeaveTable: () => void;
  loadBoardReplay: TableSession["loadBoardReplay"];
  loadPositionAnalysis: TableSession["loadPositionAnalysis"];
}) {
  return (
    <header className="table-status-bar">
      <span className="table-wordmark">BridgeYok</span>
      <span>Meja tunggu</span>
      <div className="status-actions">
        <TableInvite tableId={table.tableId} inviteCode={inviteCode} disabled={table.locked || table.participants.length >= 4} />
        <ScoreSheet loadBoardReplay={loadBoardReplay} loadPositionAnalysis={loadPositionAnalysis} table={table} />
        <div className="connection-status" data-state={connectionState}>
          <span className="status-mark" />
          {connectionLabels[connectionState]}
        </div>
        <button className="quiet-button" type="button" onClick={onLeaveTable}>
          Keluar
        </button>
      </div>
    </header>
  );
}

export function ActiveTableStatusBar({
  table,
  connectionState,
  inviteCode,
  canSendCommand,
  onCommand,
  analysisControl,
  soundMuted,
  onSoundMutedChange,
  onLeaveTable,
  loadBoardReplay,
  loadPositionAnalysis,
}: {
  table: LiveTableProjection;
  connectionState: TableSession["connectionState"];
  inviteCode: string | null;
  canSendCommand: TableSession["canSendCommand"];
  onCommand: TableSession["sendCommand"];
  analysisControl: ReactNode;
  soundMuted: boolean;
  onSoundMutedChange: (muted: boolean) => void;
  onLeaveTable: () => void;
  loadBoardReplay: TableSession["loadBoardReplay"];
  loadPositionAnalysis: TableSession["loadPositionAnalysis"];
}) {
  const leaveDrag = useDialogDrag();
  const auctionDrag = useDialogDrag();
  const leaveDialogRef = useRef<HTMLDialogElement>(null);
  const cancelLeaveRef = useRef<HTMLButtonElement>(null);
  const game = table.game;
  const contract = game?.auction.contract;
  return (
    <header className="table-status-bar play-status-bar">
      <button
        className="table-leave-button"
        type="button"
        aria-label="Keluar dari meja"
        onClick={() => { leaveDialogRef.current?.showModal(); cancelLeaveRef.current?.focus(); }}
      >
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <path d="M10 5H5v14h5M14 8l4 4-4 4M9 12h9" />
        </svg>
      </button>
      <dialog
        ref={leaveDialogRef}
        className="leave-table-dialog"
        aria-labelledby="leave-table-title"
        aria-describedby="leave-table-description"
      >
        <h2 {...leaveDrag} id="leave-table-title">Keluar dari meja?</h2>
        <p id="leave-table-description">
          Kursi Anda akan dilepas.
          {table.viewerRole === "OWNER" && table.participants.length === 1
            ? " Meja akan ditutup."
            : ""}
        </p>
        <div>
          <button
            type="button"
            ref={cancelLeaveRef}
            onClick={() => leaveDialogRef.current?.close()}
          >
            Batal
          </button>
          <button
            className="primary-button"
            type="button"
            onClick={() => {
              leaveDialogRef.current?.close();
              onLeaveTable();
            }}
          >
            Keluar
          </button>
        </div>
      </dialog>
      <div className="play-status-facts">
        <ScoreSheet loadBoardReplay={loadBoardReplay} loadPositionAnalysis={loadPositionAnalysis} table={table} compact />
        <div
          className="board-marker"
          role="img"
          aria-label={`Board ${game?.board.number ?? table.boardNumber}, dealer ${game?.board.dealer ?? "—"}, vulnerability ${game === undefined ? "—" : vulnerabilityLabels[game.board.vulnerability]}`}
        >
          <strong>{game?.board.number ?? table.boardNumber}</strong>
          {(["N", "E", "S", "W"] as const).map((seat) => (
            <span
              key={seat}
              className={`board-edge board-edge-${seat.toLowerCase()}`}
              data-vulnerable={
                game?.board.vulnerability === "BOTH" ||
                game?.board.vulnerability ===
                  (seat === "N" || seat === "S" ? "NS" : "EW")
              }
            >
              {game?.board.dealer === seat ? "D" : ""}
            </span>
          ))}
        </div>
        <div className="contract-tricks">
          <button
            className="present-contract"
            type="button"
            popoverTarget={`auction-history-${table.tableId}`}
            aria-label="Buka riwayat auction"
            aria-description={
              contract === undefined
                ? "Belum ada kontrak"
                : `${contractLabel(contract)} ${contract.declarer}`
            }
            aria-haspopup="dialog"
            disabled={game === undefined}
          >
            <strong data-strain={contract?.strain}>
              {contract === undefined ? "—" : contractLabel(contract)}
            </strong>
            <span>
              {contract?.declarer ??
                (game?.phase === "AUCTION" ? "Auction" : "—")}
            </span>
          </button>
          <TrickIndicator table={table} />
        </div>
      </div>
      {game === undefined ? null : (
        <section
          className="auction-history-popover"
          id={`auction-history-${table.tableId}`}
          popover="auto"
          role="dialog"
          aria-labelledby="auction-history-title"
        >
          <header {...auctionDrag}>
            <h2 id="auction-history-title">Auction</h2>
            <button
              type="button"
              popoverTarget={`auction-history-${table.tableId}`}
              popoverTargetAction="hide"
              aria-label="Tutup riwayat auction"
            >
              ×
            </button>
          </header>
          <AuctionTable game={game} />
        </section>
      )}
      <ConsensusControls
        analysisControl={analysisControl}
        table={table}
        canSendCommand={canSendCommand}
        onCommand={onCommand}
      />
      <div className="status-actions">
        <div
          className="connection-status"
          data-state={connectionState}
          role="status"
        >
          <span className="status-mark" />
          <span className="connection-label">
            {connectionLabels[connectionState]}
          </span>
        </div>
        <details className="table-menu">
          <summary aria-label="Buka menu meja">•••</summary>
          <div>
            {table.viewerRole === "OWNER" &&
            table.state === "BETWEEN_BOARDS" ? (
              <button
                type="button"
                disabled={!canSendCommand("table.finish")}
                onClick={(event) => {
                  event.preventDefault();
                  onCommand("table.finish");
                }}
              >
                Akhiri meja
              </button>
            ) : null}
            <TableInvite tableId={table.tableId} inviteCode={inviteCode} disabled={table.locked || table.participants.length >= 4} />
            {inviteCode === null ? null : (
              <span className="table-menu-invite">
                <small>Kode undangan</small>
                <code className="invite-code">{inviteCode}</code>
              </span>
            )}
            <label className="table-menu-toggle">
              <input
                type="checkbox"
                checked={!soundMuted}
                onChange={(event) => onSoundMutedChange(!event.target.checked)}
              />
              Suara giliran
            </label>
          </div>
        </details>
      </div>
    </header>
  );
}
