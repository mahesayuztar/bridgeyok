import { useLayoutEffect, useRef } from "react";
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
                    className={getCallClass(record?.call)}
                  >
                    {record === undefined
                      ? null
                      : callLabel(record.call)}
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
  const legalKeys = new Set(legalCalls.map(callKey));
  return (
    <section className="bidding-box" aria-label="Kotak lelang">
      <div className="call-actions">
        {actionCalls.map(({ label, call, shortcut }) => (
          <button
            type="button"
            key={label}
            disabled={disabled || !legalKeys.has(callKey(call)) || !canCall(call)}
            onClick={() => onCall(call)}
          >
            {label}
            <kbd>{shortcut}</kbd>
          </button>
        ))}
      </div>
      <div className="bid-strains bid-matrix" role="group" aria-label="Pilih bid">
        <div className="bid-matrix-header" aria-hidden="true">
          <span />
          {strains.map((strain) => (
            <span className={callColor[strain]} key={strain}>
              {strain === "NT" ? "NT" : suitLabels[strain]}
            </span>
          ))}
        </div>
        {[1, 2, 3, 4, 5, 6, 7].map((level) => (
          <div className="bid-matrix-row" key={level}>
            <span className="bid-level-label" aria-hidden="true">{level}</span>
            {strains.map((strain) => {
              const call: Call = { kind: "BID", level, strain };
              return (
                <button
                  className={callColor[strain]}
                  type="button"
                  key={strain}
                  aria-label={`Bid ${callLabel(call)}`}
                  disabled={disabled || !legalKeys.has(callKey(call)) || !canCall(call)}
                  onClick={() => onCall(call)}
                >
                  {callLabel(call)}
                </button>
              );
            })}
          </div>
        ))}
      </div>
    </section>
  );
}
