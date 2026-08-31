# WBF Laws Compliance Matrix

> Status: Phase 4 Work 1 complete; experienced bridge reviewer sign-off pending
>
> Rules baseline: WBF 2017 Laws of Duplicate Bridge, including Laws 73 and 89 revisions effective 1 January 2024
>
> Product ruleset: `bridgeyok_duplicate_v1`
>
> Last reviewed: 31 August 2026

## Classification contract

This matrix classifies BridgeYok's handling of every numbered WBF law using exactly three statuses:

- `mechanically-enforced`: the applicable correct procedure is calculated by the server, or the online command model makes the irregular action impossible to persist. Rectification paragraphs that assume a physical irregularity are not implemented.
- `director-judgement`: the law depends on disputed facts, damage assessment, disclosure, tempo, conduct, unauthorized information, an adjusted score, or another human ruling. BridgeYok does not automate that judgement.
- `not-applicable`: the law concerns a physical board/card procedure, tournament movement, spectator, penalty-card, appeal, or another feature that the MVP deliberately does not provide.

An illegal command is rejected before an event is produced. A rejection does not change the authoritative aggregate, revision, sequence, or result. Evidence names are executable Go tests unless they point to the product contract.

## Matrix

| Law | Subject | Status | BridgeYok boundary and evidence |
|---:|---|---|---|
| 1 | The Pack | mechanically-enforced | The card value model permits the standard 52-card pack and rank/suit set only. `TestFullDeck`; `TestParseCard` |
| 2 | The Duplicate Boards | mechanically-enforced | Board number deterministically fixes dealer and vulnerability on the standard 16-board cycle. `TestMetadataForBoard`; `TestMetadataForBoardRejectsNonPositiveNumber` |
| 3 | Arrangement of Tables | mechanically-enforced | The table has exactly four compass seats in clockwise order. `TestSeatRotationAndPartnership` |
| 4 | Partnerships | mechanically-enforced | N/S and E/W partnerships are fixed domain relationships. `TestSeatRotationAndPartnership` |
| 5 | Assignment of Seats | mechanically-enforced | A participant can occupy at most one unique seat; seat races reject one contender. `TestDecideSeatRaceAndOwnerControls`; `TestAggregateValidateRejectsInvalidRelationships` |
| 6 | The Shuffle and Deal | mechanically-enforced | The injected random source produces four valid 13-card hands; invalid or incomplete deals are rejected before board start. `TestGenerateDeal`; `TestDealValidateRejectsBrokenDeals` |
| 7 | Control of Board and Cards | not-applicable | There is no physical board or card handling. Server ownership and hidden-information projection are governed by Product Contract sections 8 and 12. |
| 8 | Sequence of Rounds | not-applicable | MVP single-table sessions have no tournament rounds or movement. Product Contract section 3.2. |
| 9 | Procedure Following an Irregularity | director-judgement | Persisted irregularities are prevented where mechanical; calling and directing a physical irregularity is outside the product. Product Contract sections 9.2 and 14. |
| 10 | Assessment of Rectification | director-judgement | Rectification selection and waiver require a Director and are not automated. Product Contract section 14. |
| 11 | Forfeiture of the Right to Rectification | director-judgement | Damage and forfeiture after an irregularity require a Director. Product Contract section 14. |
| 12 | Director's Discretionary Powers | director-judgement | Adjusted scores and discretionary remedies are intentionally unsupported. Product Contract sections 3.2 and 14. |
| 13 | Incorrect Number of Cards | mechanically-enforced | A board cannot start unless every hand has exactly 13 cards and the pack is unique. `TestDealValidateRejectsBrokenDeals`; `TestDecideStartRequiresFourReadySeats` |
| 14 | Missing Card | mechanically-enforced | Card conservation is validated across remaining hands, current trick, and completed tricks. `TestStateValidateInvariantsRejectsTampering` |
| 15 | Wrong Board or Hand | mechanically-enforced | Commands are table-bound and a card must belong to the active hand of that board. `TestDecideRejectsMechanicalIrregularitiesWithoutMutation`; `TestDecideRejectsInvalidCommands` |
| 16 | Authorized and Unauthorized Information | director-judgement | Recipient projection mechanically hides other hands, but inference, tempo, and unauthorized-information damage require human judgement. `TestProjectHidesOpponentHands`; Product Contract sections 12 and 14. |
| 17 | The Auction Period | mechanically-enforced | Dealer opens; calls rotate N-E-S-W; the engine closes the auction at the defined terminal condition. `TestAuctionPassedOut`; `TestAuctionDeterminesFinalContractAndFirstDeclarer` |
| 18 | Bids | mechanically-enforced | Bid shape, denomination order, level range, and sufficiency are server-validated. `TestCallValidate`; `TestAuctionRejectsIllegalCallsWithoutMutation` |
| 19 | Doubles and Redoubles | mechanically-enforced | Only the proper opposing side can double and only the doubled declaring side can redouble; a new bid supersedes the multiplier. `TestAuctionDoubleAndRedouble`; `TestAuctionRejectsIllegalCallsWithoutMutation` |
| 20 | Review and Explanation of Calls | director-judgement | The ordered auction is public, but explanations, alerts, mistaken explanations, and disclosure judgement are not modeled. Product Contract sections 3.2, 9.2, and 14. |
| 21 | Misinformation | director-judgement | Determining misinformation and damage requires convention/disclosure evidence and a Director. Product Contract section 14. |
| 22 | End of Auction | mechanically-enforced | Four initial passes pass out the board; otherwise three passes after a bid set the final contract. `TestAuctionPassedOut`; `TestAuctionDeterminesFinalContractAndFirstDeclarer` |
| 23 | Comparable Call | director-judgement | Comparable-call assessment exists only after an irregular call, which the online command path does not store. Product Contract sections 9.2 and 14. |
| 24 | Card Exposed or Led During the Auction | not-applicable | Players cannot expose or lead a physical card through the auction command model. Product Contract sections 9 and 10. |
| 25 | Legal and Illegal Changes of Call | mechanically-enforced | The latest call can only be rolled back through unanimous consent by the other three seats; this house-rule undo is not an automated Law 25 Director rectification. `TestDecideUndoConsensus`; `TestDecideRejectsUnauthorizedConsensusResponses` |
| 26 | Call Withdrawn, Lead Restrictions | not-applicable | Calls cannot be withdrawn, so withdrawal-based lead restrictions never arise. Product Contract sections 3.2 and 9.2. |
| 27 | Insufficient Bid | mechanically-enforced | An insufficient bid is rejected without changing auction or table state. `TestAuctionRejectsIllegalCallsWithoutMutation`; `TestDecideRejectsMechanicalIrregularitiesWithoutMutation` |
| 28 | Calls Considered to Be in Rotation | mechanically-enforced | There is one authoritative caller on turn; no substitute out-of-rotation procedure is accepted. `TestAuctionRejectsIllegalCallsWithoutMutation` |
| 29 | Procedure After a Call Out of Rotation | mechanically-enforced | Out-of-turn calls are rejected before persistence, so no cancellation or later rectification is needed. `TestDecideRejectsMechanicalIrregularitiesWithoutMutation` |
| 30 | Pass Out of Rotation | mechanically-enforced | An out-of-turn pass is rejected before persistence. `TestAuctionRejectsIllegalCallsWithoutMutation` |
| 31 | Bid Out of Rotation | mechanically-enforced | An out-of-turn bid is rejected before persistence. `TestAuctionRejectsIllegalCallsWithoutMutation` |
| 32 | Double or Redouble Out of Rotation | mechanically-enforced | Turn order and side eligibility are both checked before double/redouble acceptance. `TestAuctionRejectsIllegalCallsWithoutMutation` |
| 33 | Simultaneous Calls | mechanically-enforced | Commands are revision-fenced and processed serially; one accepted call advances turn and the competing stale command is rejected. `TestCommandRepositoryOrderingIdempotencyAndHydrate`; `TestAuctionRejectsIllegalCallsWithoutMutation` |
| 34 | Retention of Right to Call | mechanically-enforced | Auction turn remains with the assigned seat until one legal call is accepted. `TestAuctionLegalCalls`; `TestAuctionRejectsIllegalCallsWithoutMutation` |
| 35 | Inadmissible Calls | mechanically-enforced | Calls outside the four supported kinds or after auction completion are rejected. `TestCallValidate`; `TestAuctionRejectsCallAfterCompletion` |
| 36 | Inadmissible Doubles and Redoubles | mechanically-enforced | Invalid side, multiplier state, and timing are rejected without mutation. `TestAuctionRejectsIllegalCallsWithoutMutation` |
| 37 | Action Violating Obligation to Pass | not-applicable | An obligation to pass arises from rectification of a prior irregularity, which the product never stores. Product Contract sections 9.2 and 14. |
| 38 | Bid of More Than Seven | mechanically-enforced | Bid levels outside 1-7 fail structural validation. `TestCallValidate`; `TestAuctionRejectsIllegalCallsWithoutMutation` |
| 39 | Call After the Final Pass | mechanically-enforced | Any call after auction completion is rejected without mutation. `TestAuctionRejectsCallAfterCompletion` |
| 40 | Partnership Understandings | director-judgement | Systems, psychic actions, disclosure, and convention agreements require human judgement; no alert/convention ontology is present. Product Contract sections 3.2 and 14. |
| 41 | Commencement of Play | mechanically-enforced | The left-hand defender leads first and dummy is revealed only after that lead commits. `TestDecideContractAndOpeningLead` |
| 42 | Dummy's Rights | mechanically-enforced | Dummy remains a projected participant, while declarer controls the dummy hand and public play is server-derived. `TestDecideDeclarerControlsDummy`; `TestProjectRevealsOnlyDummyAfterOpeningLead` |
| 43 | Dummy's Limitations | mechanically-enforced | A command sent by the dummy seat for the dummy hand is rejected without mutation. `TestDecideDeclarerControlsDummy`; `TestDecideRejectsMechanicalIrregularitiesWithoutMutation` |
| 44 | Sequence and Procedure of Play | mechanically-enforced | Active hand, follow-suit, trump/led-suit winner, and next leader are calculated by the engine. `TestDecideEnforcesFollowSuit`; `TestTrickWinner`; `TestDecideCompletesBoard` |
| 45 | Card Played | mechanically-enforced | A play names one exact card from the active hand and becomes immutable once accepted. `TestDecideContractAndOpeningLead`; `TestDecideIsDeterministicAndDoesNotMutateInput` |
| 46 | Incomplete or Invalid Designation of a Card from Dummy | not-applicable | UI/protocol commands identify one exact canonical card; spoken or incomplete designation is absent. Product Contract section 10. |
| 47 | Retraction of Card Played | mechanically-enforced | The latest play can only be rolled back through unanimous consent by the other three seats; unilateral retraction is impossible. `TestDecideUndoConsensus`; `TestDecideRejectsUnauthorizedConsensusResponses` |
| 48 | Exposure of Declarer's Cards | not-applicable | There are no physical cards; voluntary disclosure outside the application cannot be adjudicated by the engine. Product Contract sections 10, 12, and 14. |
| 49 | Exposure of a Defender's Cards | not-applicable | There are no physical cards or exposed-card state; hidden projection remains authoritative. Product Contract section 12. |
| 50 | Disposition of Penalty Card | not-applicable | Penalty cards and their rectification are outside MVP scope. Product Contract section 3.2. |
| 51 | Two or More Penalty Cards | not-applicable | Penalty cards and their rectification are outside MVP scope. Product Contract section 3.2. |
| 52 | Failure to Lead or Play a Penalty Card | not-applicable | Penalty cards and their rectification are outside MVP scope. Product Contract section 3.2. |
| 53 | Lead Out of Turn Accepted | mechanically-enforced | A lead by any seat other than the authoritative leader is rejected and cannot be accepted by an opponent. `TestDecideRejectsMechanicalIrregularitiesWithoutMutation` |
| 54 | Faced Opening Lead Out of Turn | mechanically-enforced | Only the defender left of declarer can commit the opening lead. `TestDecideContractAndOpeningLead`; `TestDecideRejectsMechanicalIrregularitiesWithoutMutation` |
| 55 | Declarer's Lead Out of Turn | mechanically-enforced | Declarer can act only for the current declarer/dummy hand; any other lead is rejected. `TestDecideRejectsMechanicalIrregularitiesWithoutMutation` |
| 56 | Defender's Lead Out of Turn | mechanically-enforced | Defender commands are accepted only for the defender currently on turn. `TestDecideRejectsMechanicalIrregularitiesWithoutMutation` |
| 57 | Premature Lead or Play | mechanically-enforced | A play before that hand's turn is rejected without mutation. `TestDecideRejectsMechanicalIrregularitiesWithoutMutation` |
| 58 | Simultaneous Leads or Plays | mechanically-enforced | Serialized, revision-fenced commands yield one accepted play; later stale/out-of-turn commands cannot enter the state. `TestCommandRepositoryOrderingIdempotencyAndHydrate`; `TestDecideRejectsMechanicalIrregularitiesWithoutMutation` |
| 59 | Inability to Lead or Play as Required | mechanically-enforced | Legal-card generation is derived from the exact remaining hand and always offers held cards while play is active. `TestDecideCompletesBoard`; `TestDecideEnforcesFollowSuit` |
| 60 | Play After an Illegal Play | mechanically-enforced | Illegal play produces no event, so no subsequent state can rely on it. `TestDecideRejectsMechanicalIrregularitiesWithoutMutation`; `TestReduceRejectsTamperedEvents` |
| 61 | Failure to Follow Suit; Inquiries Concerning a Revoke | mechanically-enforced | Failure to follow suit while holding the led suit is rejected; legal-card projection contains only led-suit cards. `TestDecideEnforcesFollowSuit`; `TestDecideRejectsMechanicalIrregularitiesWithoutMutation` |
| 62 | Correction of a Revoke | not-applicable | A revoke cannot enter authoritative state, so revoke correction and withdrawn-card consequences do not arise. Product Contract section 10. |
| 63 | Establishment of a Revoke | not-applicable | A revoke cannot enter authoritative state. Product Contract section 10. |
| 64 | Procedure After Establishment of a Revoke | not-applicable | Revoke trick transfer and damage adjustment are unsupported because revoke is prevented. Product Contract sections 3.2, 10, and 14. |
| 65 | Arrangement of Tricks | mechanically-enforced | The server stores cards in play order, closes exactly four-card tricks, and records the winner. `TestDecideCompletesBoard`; `TestStateValidateInvariantsRejectsTampering` |
| 66 | Inspection of Tricks | mechanically-enforced | Current and completed public tricks are retained in ordered authoritative state; no hidden card is inferred. `TestDecideCompletesBoard`; `TestProjectRevealsOnlyDummyAfterOpeningLead` |
| 67 | Defective Trick | mechanically-enforced | A trick cannot complete with other than four unique, in-order plays. `TestTrickWinnerRejectsInvalidTrick`; `TestStateValidateInvariantsRejectsTampering` |
| 68 | Claim or Concession of Tricks | mechanically-enforced | A non-dummy player may claim a precise number of remaining tricks only at a completed-trick boundary; claiming zero concedes the remaining tricks. `TestDecideClaimAtTrickBoundary`; `TestDecideRejectsInvalidClaims` |
| 69 | Agreed Claim or Concession | mechanically-enforced | A claim scores the board only after both opponents accept the exact allocation. `TestDecideClaimConsensus`; `TestDecideClaimAtTrickBoundary` |
| 70 | Contested Claim or Concession | director-judgement | Any opponent rejection cancels the request and resumes play; adjudicating a contested line remains outside the application. Product Contract sections 10 and 14. |
| 71 | Concession Cancelled | mechanically-enforced | A zero-trick claim is a concession and only becomes final after both opponents accept; rejection resumes play without changing the board. `TestDecideClaimRejectionResumesPlay`; `TestDecideClaimConsensus` |
| 72 | General Principles | director-judgement | Intent, damage, concealment, and post-irregularity remedies require a Director; mechanical illegal actions are prevented separately. Product Contract section 14. |
| 73 | Communication, Behaviour, Tempo and Deception | director-judgement | The 2024 revision covers human communication/tempo/deception findings that the engine cannot infer safely. Product Contract section 14. |
| 74 | Conduct and Etiquette | director-judgement | Etiquette, annoyance, and pace require human observation and enforcement. Product Contract section 14. |
| 75 | Mistaken Explanation or Mistaken Call | director-judgement | Convention explanation and misinformation damage are not modeled. Product Contract sections 3.2 and 14. |
| 76 | Spectators | not-applicable | Live spectators are explicitly outside MVP scope and cannot subscribe to an active board. Product Contract sections 3.2 and 12. |
| 77 | Duplicate Bridge Scoring Table | mechanically-enforced | The versioned scorer covers made, defeated, doubled/redoubled, vulnerable, game, slam, overtrick, undertrick, and passed-out outcomes. `TestScoreContractGoldenMatrix`; `TestScoreContractPartnershipSymmetry`; `TestPassedOutResult` |
| 78 | Methods of Scoring and Conditions of Contest | not-applicable | Phase 4 Work 1-2 covers per-board duplicate score only. IMP comparison belongs to Phase 5; matchpoints and other methods remain out of scope. Product Contract sections 3.2 and 11. |
| 79 | Tricks Won | mechanically-enforced | Trick winners and NS/EW totals are server-calculated and the final score is derived after exactly 13 tricks. `TestDecideCompletesBoard`; `TestResultValidateRejectsTampering` |
| 80 | Regulation and Organization | not-applicable | BridgeYok MVP is not a tournament organizer and provides no regulating-authority workflow. Product Contract sections 2 and 3.2. |
| 81 | The Director | director-judgement | No Director console or automated Director authority exists. Product Contract sections 3.2 and 14. |
| 82 | Rectification of Errors of Procedure | director-judgement | Procedural error correction and adjusted scores require a Director. Product Contract section 14. |
| 83 | Notification of the Right to Appeal | director-judgement | Appeals follow an external human process; no in-product ruling flow exists. Product Contract section 14. |
| 84 | Rulings on Agreed Facts | director-judgement | Rulings and rectifications on agreed facts require a Director. Product Contract section 14. |
| 85 | Rulings on Disputed Facts | director-judgement | Fact finding and standard-of-proof decisions require a Director. Product Contract section 14. |
| 86 | Team Play | director-judgement | Substitute/fouled-board and adjusted-result provisions require a Director. Normal two-table IMP orchestration is deferred to Phase 5 and is not evidence for this law's rulings. Product Contract sections 3.1 and 14. |
| 87 | Fouled Board | director-judgement | Determining and scoring a fouled comparison requires fact finding and an adjusted score. Product Contract section 14. |
| 88 | Award of Indemnity Points | director-judgement | Artificial adjusted scores are unsupported and require a Director. Product Contract section 14. |
| 89 | Prohibited Behaviour and Reprehensible Conduct | director-judgement | The 2024 law requires investigation of illicit information/communication and disciplinary judgement; hidden-hand projection is preventive evidence only. Product Contract sections 12 and 14. |
| 90 | Procedural Penalties | director-judgement | Procedural penalties and obstruction/delay findings require a Director. Product Contract section 14. |
| 91 | Penalize or Suspend | director-judgement | Disciplinary penalties, suspension, and disqualification are not automated. Product Contract section 14. |
| 92 | Right to Appeal | director-judgement | No in-product Director ruling or appeal workflow exists. Product Contract section 14. |
| 93 | Procedures of Appeal | director-judgement | Appeal adjudication remains an external human process. Product Contract section 14. |

## Review gate

Engineering evidence is executable and kept in sync by `TestWBFComplianceMatrix`. The Phase 4 exit gate still requires an independent experienced bridge reviewer to approve the classifications and boundary reasoning. That sign-off must not silently change a status: any change must update this matrix, its linked product contract, and the corresponding tests.

## Normative sources

- [WBF 2017 Laws of Duplicate Bridge, revised Laws 73 and 89 effective 1 January 2024](https://www.worldbridge.org/wp-content/uploads/2016/12/2017LawsofDuplicateBridge-paginated.pdf)
- [WBF Laws page and revision history](https://www.worldbridge.org/regulations/2017-laws-of-duplicate-bridge/)
- [WBF 2017 Laws Commentary, revised January 2023](https://www.worldbridge.org/wp-content/uploads/2019/01/2017LawsCommentary.pdf)
