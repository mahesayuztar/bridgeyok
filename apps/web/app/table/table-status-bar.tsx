import type { LiveTableProjection } from "../table-state";
import type { TableSession } from "../use-table-session";
import { ConsensusControls } from "./consensus-controls";
import { contractSummaryLabel } from "./gameplay-presentation";
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
  connectionState,
  onLeaveTable,
}: {
  connectionState: TableSession["connectionState"];
  onLeaveTable: () => void;
}) {
  return (
    <header className="table-status-bar">
      <span className="table-wordmark">BridgeYok</span>
      <span>Meja tunggu</span>
      <div className="status-actions">
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
  soundMuted,
  onSoundMutedChange,
  onLeaveTable,
}: {
  table: LiveTableProjection;
  connectionState: TableSession["connectionState"];
  inviteCode: string | null;
  canSendCommand: TableSession["canSendCommand"];
  onCommand: TableSession["sendCommand"];
  soundMuted: boolean;
  onSoundMutedChange: (muted: boolean) => void;
  onLeaveTable: () => void;
}) {
  const game = table.game;
  const contract = game?.auction.contract;
  return (
    <header className="table-status-bar">
      <span className="table-wordmark">BY</span>
      <dl className="table-facts">
        <div>
          <dt>Board</dt>
          <dd>{game?.board.number ?? table.boardNumber}</dd>
        </div>
        <div>
          <dt>Dealer</dt>
          <dd>{game?.board.dealer ?? "—"}</dd>
        </div>
        <div>
          <dt>Vul</dt>
          <dd>
            {game === undefined
              ? "—"
              : vulnerabilityLabels[game.board.vulnerability]}
          </dd>
        </div>
        <div>
          <dt>Kontrak</dt>
          <dd className="contract-fact">
            {game === undefined ? "—" : contractSummaryLabel(contract)}
          </dd>
        </div>
        <div>
          <dt>Trick</dt>
          <dd><TrickIndicator table={table} /></dd>
        </div>
      </dl>
      <ConsensusControls
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
          <span className="connection-label">{connectionLabels[connectionState]}</span>
        </div>
        <details className="table-menu">
          <summary aria-label="Buka menu meja">•••</summary>
          <div>
            <button type="button" onClick={onLeaveTable}>
              Keluar dari meja
            </button>
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
            <span>Rev {table.revision}</span>
          </div>
        </details>
      </div>
    </header>
  );
}
