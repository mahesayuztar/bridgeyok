# ADR-010: Automatic Controller Handoff to the Latest Guest Device

- Status: Accepted
- Date: 2 September 2026
- Decision owners: Product/Engineering
- Supersedes: manual takeover UX in ADR-003 and `apps/web/PLAN.md` section 7

## Context

Controller fencing already guarantees that only one connection epoch can mutate a seated guest's cards and table actions. The previous web flow made a newly connected tab or device attempt an action, resync, and then ask the user to confirm **Ambil alih kendali**. That confirmation adds a dead-end interaction even though the user has already authenticated as the same guest and opened the same table.

## Decision

After a new realtime connection for a seated guest receives a fresh recipient projection, the web client automatically sends exactly one `table.takeover` command for that projection revision.

- The handoff has no confirmation button and no success toast.
- The command still carries `request_id`, `expected_revision`, and `controller_epoch` and remains a durable server-authoritative mutation.
- Legal table/game actions stay disabled while synchronization or takeover is pending.
- `CONTROLLER_REPLACED` confirms the new controller epoch before actions are enabled.
- The prior controller is not disconnected. A viewer-epoch change not caused by its own pending takeover immediately marks it mirror/read-only, and the server still rejects its old epoch with `STALE_CONTROLLER`.
- A rejected user gameplay/table mutation is never replayed as part of handoff. The user may repeat that intent only after control is current.
- An unseated guest does not send takeover; `table.take_seat` continues to authorize the connection that takes the seat.
- A new connection that later requests the same seated guest becomes the latest controller. Concurrent devices therefore use last committed takeover wins while preserving one-controller fencing.

The `table.takeover` protocol command remains available for backward compatibility with older clients. This change requires no schema or database migration.

## Consequences

Positive:

- Reload, reconnect, and a newly opened device become usable without an extra confirmation step.
- Existing revision, idempotency, and controller-epoch security boundaries remain intact.
- The old device cannot silently submit a mutation after the handoff.

Trade-offs:

- Two devices repeatedly reopening or requesting the same guest can intentionally move control back and forth.
- Mirror-only behavior is no longer the default web experience for a seated guest.
- A failed takeover needs a fresh resync before another automatic attempt, preventing request loops on one stale projection.

## Verification

- Reducer test: new controller sync → fresh projection → takeover pending → authoritative replacement → current, while the prior device becomes mirror/read-only.
- Two-tab Playwright flow: the newest tab can mutate without a takeover button.
- Existing realtime fencing tests continue to prove that the older connection epoch is rejected.
