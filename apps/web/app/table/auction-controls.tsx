import { useLayoutEffect, useRef, useState } from "react";
import {
  auctionRows,
  type Call,
  type LiveTableProjection,
  type Seat,
} from "../table-state";
import {
  callKey,
  callLabel,
  contractLabel,
  suitLabels,
} from "./gameplay-presentation";

const auctionSeats: Seat[] = ["W", "N", "E", "S"];
const strains: Array<"C" | "D" | "H" | "S" | "NT"> = [
  "C",
  "D",
  "H",
  "S",
  "NT",
];
const actionCalls: Array<{ label: string; call: Call; shortcut: string }> = [
  { label: "Pass", call: { kind: "PASS" }, shortcut: "P" },
  { label: "X", call: { kind: "DOUBLE" }, shortcut: "X" },
  { label: "XX", call: { kind: "REDOUBLE" }, shortcut: "R" },
];
const callColor: Partial<Record<"C" | "D" | "H" | "S" | "NT", string>> = {
  S: "s-call",
  H: "h-call",
  D: "d-call",
  C: "c-call",
};
export function AuctionTable({
  game,
  followLatest = true,
  showSummary = true,
}: {
  game: NonNullable<LiveTableProjection["game"]>;
  followLatest?: boolean;
  showSummary?: boolean;
}) {
  const auctionTableRef = useRef<HTMLDivElement>(null);
  const followScrollRef = useRef(true);
  const rows = auctionRows(game.auction.dealer, game.auction.calls);

  useLayoutEffect(() => {
    const auctionTable = auctionTableRef.current;

    if (auctionTable && followLatest && followScrollRef.current) {
      auctionTable.scrollTop = auctionTable.scrollHeight;
    }
  }, [followLatest, game.auction.calls.length]);

  function getCallClass(call?: Call) {
    if (!call || call.kind !== "BID") return "";

    if (call.strain === undefined) return "";

    if (call.strain === "NT") return "nt-call";

    return callColor[call.strain] ?? "";
  }

  return (
    <div
      ref={auctionTableRef}
      className="auction-table-wrap"
      onScroll={(event) => {
        const table = event.currentTarget;
        followScrollRef.current =
          table.scrollHeight - table.scrollTop - table.clientHeight < 8;
      }}
    >
      <table className="auction-table">
        {showSummary ? (
          <caption>
            <span>
              Dealer <strong>{game.auction.dealer}</strong>
            </span>
            <span>
              {game.auction.contract === undefined ? (
                game.auction.passedOut ? "Passed out" : "Kontrak belum ditentukan"
              ) : (
                <>
                  Kontrak <strong>{contractLabel(game.auction.contract)}</strong>
                  <span aria-hidden="true"> · </span>
                  Deklarer <strong>{game.auction.contract.declarer}</strong>
                </>
              )}
            </span>
          </caption>
        ) : null}
        <thead>
          <tr>
            {auctionSeats.map((seat) => (
              <th
                key={seat}
                scope="col"
                aria-label={`${seat}${game.auction.dealer === seat ? ", dealer" : ""}${game.turn === seat ? ", giliran" : ""}`}
                data-dealer={game.auction.dealer === seat}
                data-turn={game.turn === seat}
                data-vulnerable={game.board.vulnerability === "BOTH" || game.board.vulnerability === (seat === "N" || seat === "S" ? "NS" : "EW")}
              >
                <span>{seat}</span>
                {showSummary && game.auction.dealer === seat ? <small>Dealer</small> : null}
              </th>
            ))}
          </tr>
        </thead>

        <tbody>
          {rows.map((row, _rowIndex) => (
            <tr key={_rowIndex}>
              {auctionSeats.map((seat) => {
                const record = row[seat];

                return (
                  <td
                    key={seat}
                    className={[
                      getCallClass(record?.call),
                      record?.call.alert === true ? "artificial-call" : "",
                    ].filter(Boolean).join(" ")}
                    aria-label={record?.call.alert === true ? `${record.call.kind === "BID" ? callLabel(record.call) : "Call"}, artificial` : undefined}
                  >
                    {record === undefined ? null : (
                      <>
                        <span>{callLabel(record.call)}</span>
                        {record.call.alert === true ? (
                          <span className="auction-alert-marker" title="Bid artificial">A</span>
                        ) : null}
                      </>
                    )}
                  </td>
                );
              })}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

export function BiddingBox({
  legalCalls,
  disabled,
  canCall,
  onCall,
}: {
  legalCalls: Call[];
  disabled: boolean;
  canCall: (call: Call) => boolean;
  onCall: (call: Call) => void;
}) {
  const [selectedBid, setSelectedBid] = useState<{
    level: number;
    signature: string;
  } | null>(null);
  const [alertIntent, setAlertIntent] = useState<{ signature: string; enabled: boolean } | null>(null);
  const legalKeys = new Set(legalCalls.map(callKey));
  const legalCallSignature = legalCalls.map(callKey).join("|");
  const legalBidLevels = [1, 2, 3, 4, 5, 6, 7].filter((level) =>
    strains.some((strain) => legalKeys.has(callKey({ kind: "BID", level, strain }))),
  );
  const alertEnabled = alertIntent?.signature === legalCallSignature && alertIntent.enabled;
  const canAlert = !disabled && legalBidLevels.length > 0;
  const activeLevel = disabled || selectedBid === null || selectedBid.signature !== legalCallSignature || !legalBidLevels.includes(selectedBid.level)
    ? null
    : selectedBid.level;

  function selectLevel(level: number) {
    if (disabled || !legalBidLevels.includes(level)) return;
    setSelectedBid({ level, signature: legalCallSignature });
  }

  function submitBid(strain: (typeof strains)[number]) {
    if (activeLevel === null) return;
    const call: Call = {
      kind: "BID",
      level: activeLevel,
      strain,
      ...(alertEnabled ? { alert: true } : {}),
    };
    if (!legalKeys.has(callKey(call)) || !canCall(call)) return;
    setSelectedBid(null);
    onCall(call);
  }

  return (
    <section className="bidding-box" aria-label="Kotak lelang">
      <div className="call-actions">
        {actionCalls.map(({ label, call, shortcut }) => (
          <button
            type="button"
            key={label}
            disabled={disabled || !legalKeys.has(callKey(call)) || !canCall(call)}
            onClick={() => {
              setSelectedBid(null);
              onCall(call);
            }}
          >
            {label}
            <kbd>{shortcut}</kbd>
          </button>
        ))}
      </div>
      <div className="bid-alert-control">
        <button
          className={alertEnabled ? "bid-alert-toggle is-active" : "bid-alert-toggle"}
          type="button"
          aria-pressed={alertEnabled}
          disabled={!canAlert}
          onClick={() => setAlertIntent({ signature: legalCallSignature, enabled: !alertEnabled })}
        >
          <span className="bid-alert-marker" aria-hidden="true">A</span>
          <span>Alert</span>
          <span className="bid-alert-state">{alertEnabled ? "ON" : "OFF"}</span>
        </button>
        <span>Bid berikutnya artificial</span>
      </div>
      {activeLevel === null ? (
        <div className="bid-levels" role="group" aria-label="Pilih level bid">
          {legalBidLevels.map((level) => (
            <button
              type="button"
              key={level}
              aria-label={`Pilih level ${level}`}
              disabled={disabled || !strains.some((strain) => {
                const call: Call = { kind: "BID", level, strain };
                return legalKeys.has(callKey(call)) && canCall(call);
              })}
              onClick={() => selectLevel(level)}
            >
              {level}
            </button>
          ))}
        </div>
      ) : (
        <div className="bid-strain-stage">
          <div className="bid-stage-heading">
            <button
              className="bid-stage-level"
              type="button"
              aria-label="Ubah level bid"
              onClick={() => setSelectedBid(null)}
            >
              {activeLevel}
            </button>
            <span aria-hidden="true">→</span>
            <span>Denom.</span>
          </div>
          <div
            className="bid-strains"
            role="group"
            aria-label={`Pilih denomination untuk level ${activeLevel}`}
          >
            {strains.map((strain) => {
              const call: Call = { kind: "BID", level: activeLevel, strain };
              const available = legalKeys.has(callKey(call));
              return available ? (
                <button
                  className={callColor[strain]}
                  type="button"
                  key={strain}
                  aria-label={`Bid ${callLabel(call)}`}
                  disabled={disabled || !canCall(call)}
                  onClick={() => submitBid(strain)}
                >
                  {strain === "NT" ? "NT" : suitLabels[strain]}
                </button>
              ) : null;
            })}
          </div>
        </div>
      )}
    </section>
  );
}
