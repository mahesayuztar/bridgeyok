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
- Claim and undo are unavailable while any bot is seated because a first-legal-move bot cannot provide informed consent.

## Consequences

Bot state lives in the private aggregate snapshot. Human seats remain mirrored in `table_seats`; bot seats are intentionally skipped by that identity/recovery relation because bots have no credential or recovery token. Restart hydration uses the authoritative private snapshot, preserving bot seats and pending turn state.

The realtime protocol gains owner mutations for add, remove, and replace-with-bot. This decision does not add bidding heuristics, DDS-driven play, machine learning, configurable bot levels, autonomous claims, or external AI dependencies.
