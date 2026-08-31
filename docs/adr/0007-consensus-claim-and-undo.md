# ADR 0007: Consensus claim and undo

- Status: accepted
- Date: 31 August 2026

## Context

The initial MVP excluded claim, concession, and undo to keep the authoritative game state linear. Closed-beta play requires a safe way to finish obvious positions and correct the immediately preceding accidental action without giving one player unilateral rollback authority.

## Decision

BridgeYok supports one pending consensus request per table.

- A non-dummy player may claim from zero through all remaining tricks for their partnership only between completed tricks and after dummy is visible.
- Both opponents must accept the exact claim. Any rejection clears the request and resumes the unchanged game.
- Only the actor of the latest accepted call or play may request undo.
- The other three seats must accept undo. Any rejection clears the request without changing the game.
- While a request is pending, new calls and plays are rejected.
- Accepted undo restores the private server snapshot immediately before that action and cannot be redone.
- Requests, responses, acceptance, and rejection are durable ordered table events. The server remains authoritative and idempotency/revision fencing still applies.

This is a closed-table consent workflow, not automated Director adjudication. A contested claim resumes play; BridgeYok does not judge a proposed line of play or apply WBF rectification.

## Consequences

The private table snapshot stores one prior game state as the undo candidate. Recipient projections expose only request progress, response eligibility, and whether that recipient may request undo; they never expose the private prior state. Claim-completed results can contain fewer than thirteen played tricks while still recording a canonical thirteen-trick score allocation.
