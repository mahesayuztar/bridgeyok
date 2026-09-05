# ADR 0008: Simple table bots

- Status: accepted
- Date: 31 August 2026

## Context

A table can otherwise remain blocked when fewer than four people are available or a player leaves during a board. The owner needs to fill an empty seat or replace a non-owner player without introducing an AI service, hidden-hand leak, or a second game-rules implementation.

## Decision

BridgeYok supports table-owned bots with deliberately deterministic behavior.

- Only the table owner may add a bot to an empty seat, remove a bot, or replace a seated non-owner participant with a bot.
- Replacement removes the human participant and assigns the bot to the same seat in one durable revision.
- A bot is a seat occupant, not a guest identity, session, connection, or controller.
- Bot calls and card plays are serialized by the existing table actor and committed through the same command repository as human actions.
- On its turn, the bot submits the first item returned by the authoritative engine's stable legal-call or legal-card list. It does not evaluate hand strength, outcomes, or strategy.
- When a bot is declarer, it also plays the first legal card from dummy when dummy is on turn. A bot seated as dummy remains controlled by a human declarer.
- Bot occupants are included in recipient projections with `isBot: true`; they have no presence record and never receive hidden projections.
- Claim and undo use the deterministic partnership response policy below (ENG-03, accepted 5 September 2026). This explicitly supersedes the original blanket prohibition when any bot is seated.

## Consequences

Bot state lives in the private aggregate snapshot. Human seats remain mirrored in `table_seats`; bot seats are intentionally skipped by that identity/recovery relation because bots have no credential or recovery token. Restart hydration uses the authoritative private snapshot, preserving bot seats and pending turn state.

The realtime protocol gains owner mutations for add, remove, and replace-with-bot. This decision does not add bidding heuristics, DDS-driven play, machine learning, configurable bot levels, autonomous claims, or external AI dependencies.

## ENG-03: deterministic consensus responses

Accepted 5 September 2026 after UX-G1 and ENG-02 PASS. This is a product consent policy, not strategic bot evaluation.

Requests require all four seats occupied. Only humans initiate requests. Claim still requires the two opponents; undo still requires the three non-requester seats. The requester implicitly consents to their own undo, allowing their bot partner to approve immediately. The requester's partner does not vote on a claim.

| Eligible bot's partner | Durable state | Server response |
| --- | --- | --- |
| Bot | Any pending claim/undo | Reject; one terminal rejection clears the request |
| Human requester | Undo request | Accept, following the requester's implicit consent |
| Human responder | Has not approved | Wait; the human remains eligible to respond |
| Human responder | Has approved | Accept |
| Human responder | Rejects | Human rejection immediately terminates; no redundant bot vote is emitted |

The actor selects eligible bot responses in N/E/S/W order and sends each through the existing revision-fenced command repository, with the existing deterministic bot request ID. The aggregate independently validates bot seat, eligibility, and the policy-selected response. Humans never submit or impersonate bot votes over the wire. No bot credential, new protocol field, scheduler, or frontend vote synthesis is introduced.

Pending requests and human approvals remain in the durable snapshot. Actor hydration/refresh resumes any enabled bot response, including after a human approval was committed before an interrupted bot follow-up. Seat removal/replacement continues to cancel pending consensus; connection loss alone preserves it for recovery. Existing command/bot persistence, rejection, and publication error signals cover failures without logging hands or adding participant/score metric labels.

A human may wait for another human who has not answered; a bot never waits for another bot or for the requester to submit an ineligible response. A negative response is terminal immediately, and later responses cannot produce a second outcome. Bot calls/plays stay paused during consensus and resume only after its authoritative terminal transition.
