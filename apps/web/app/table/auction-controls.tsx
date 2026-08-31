import { useState } from "react";
import {
  auctionRows,
  type Call,
  type LiveTableProjection,
  type Seat,
} from "../table-state";
import {
  callKey,
  callLabel,
  suitLabels,
} from "./gameplay-presentation";

const auctionSeats: Seat[] = ["W", "N", "E", "S"];
const strains: Array<"C" | "D" | "H" | "S" | "NT"> = [
  "S",
  "H",
  "D",
  "C",
  "NT",
];
const actionCalls: Array<{ label: string; call: Call; shortcut: string }> = [
  { label: "Pass", call: { kind: "PASS" }, shortcut: "P" },
  { label: "X", call: { kind: "DOUBLE" }, shortcut: "X" },
  { label: "XX", call: { kind: "REDOUBLE" }, shortcut: "R" },
];

export function AuctionTable({
  game,
}: {
  game: NonNullable<LiveTableProjection["game"]>;
}) {
  const rows = auctionRows(game.auction.dealer, game.auction.calls);
  return (
    <div className="auction-table-wrap">
      <div className="auction-caption">
        <strong>Lelang</strong>
        <span>Dealer {game.auction.dealer}</span>
      </div>
      <table className="auction-table">
        <thead>
          <tr>
            {auctionSeats.map((seat) => (
              <th key={seat} scope="col" data-turn={game.turn === seat}>
                {seat}
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
                    className={
                      record?.call.strain === "H" || record?.call.strain === "D"
                        ? "red-call"
                        : ""
                    }
                  >
                    {record === undefined ? null : callLabel(record.call)}
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
  onCall,
}: {
  legalCalls: Call[];
  disabled: boolean;
  onCall: (call: Call) => void;
}) {
  const [selectedLevel, setSelectedLevel] = useState<number | null>(null);
  const legalKeys = new Set(legalCalls.map(callKey));
  const legalLevels = new Set(
    legalCalls.filter((call) => call.kind === "BID").map((call) => call.level),
  );
  return (
    <section className="bidding-box" aria-label="Kotak lelang">
      <div className="call-actions">
        {actionCalls.map(({ label, call, shortcut }) => (
          <button
            type="button"
            key={label}
            disabled={disabled || !legalKeys.has(callKey(call))}
            onClick={() => {
              setSelectedLevel(null);
              onCall(call);
            }}
          >
            {label}
            <kbd>{shortcut}</kbd>
          </button>
        ))}
      </div>
      <div className="bid-levels" aria-label="Pilih level bid">
        {[1, 2, 3, 4, 5, 6, 7].map((level) => (
          <button
            type="button"
            key={level}
            aria-pressed={selectedLevel === level}
            disabled={disabled || !legalLevels.has(level)}
            onClick={() => setSelectedLevel(level)}
          >
            {level}
          </button>
        ))}
      </div>
      {selectedLevel === null ? (
        <p className="bid-hint">Pilih level, lalu pilih strain.</p>
      ) : (
        <div
          className="bid-strains"
          aria-label={`Pilih strain untuk level ${selectedLevel}`}
        >
          {strains.map((strain) => {
            const call: Call = { kind: "BID", level: selectedLevel, strain };
            return (
              <button
                className={strain === "H" || strain === "D" ? "red-call" : ""}
                type="button"
                key={strain}
                disabled={disabled || !legalKeys.has(callKey(call))}
                onClick={() => {
                  setSelectedLevel(null);
                  onCall(call);
                }}
              >
                <span className="sr-only">Bid {selectedLevel} </span>
                {strain === "NT" ? "NT" : suitLabels[strain]}
              </button>
            );
          })}
        </div>
      )}
    </section>
  );
}
