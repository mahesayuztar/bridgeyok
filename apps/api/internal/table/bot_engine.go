package table

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/mahesayuztar/bridgeyok/apps/api/internal/analysis"
	"github.com/mahesayuztar/bridgeyok/apps/api/internal/bridge"
)

// BotPositionSolver evaluates every legal card in one complete position.
type BotPositionSolver interface {
	SolvePosition(context.Context, bridge.State) ([]analysis.CardPrediction, error)
}

// BotDecisionEngineOptions bounds bot computation and its process-local cache.
type BotDecisionEngineOptions struct {
	MaxCandidates      int
	SampleCount        int
	ProgressiveSamples []int
	TimeBudget         time.Duration
	CacheCapacity      int
	Logger             *slog.Logger
}

// BotDecisionEngine chooses cards from visible information and sampled deals.
type BotDecisionEngine struct {
	solver  BotPositionSolver
	options BotDecisionEngineOptions

	cacheMutex sync.Mutex
	cache      map[string][]analysis.CardPrediction
	cacheOrder []string
}

type botCardStats struct {
	card      bridge.Card
	heuristic float64
	expected  float64
	successes int
	samples   int
}

type botDecision struct {
	card           bridge.Card
	samples        int
	evaluated      int
	ddsCalls       int
	cacheHits      int
	cacheMisses    int
	expectedTricks float64
	contractChance float64
	confidence     float64
}

func NewBotDecisionEngine(solver BotPositionSolver, options BotDecisionEngineOptions) *BotDecisionEngine {
	if options.MaxCandidates < 1 {
		options.MaxCandidates = 5
	}
	if options.SampleCount < 1 {
		options.SampleCount = 4
	}
	if len(options.ProgressiveSamples) == 0 {
		firstBudget := 2
		if options.SampleCount < firstBudget {
			firstBudget = options.SampleCount
		}
		options.ProgressiveSamples = []int{firstBudget, options.SampleCount}
	}
	if options.TimeBudget <= 0 {
		options.TimeBudget = 400 * time.Millisecond
	}
	if options.CacheCapacity < 1 {
		options.CacheCapacity = 512
	}
	if options.Logger == nil {
		options.Logger = slog.Default()
	}
	return &BotDecisionEngine{
		solver:  solver,
		options: options,
		cache:   make(map[string][]analysis.CardPrediction, options.CacheCapacity),
	}
}

func chooseBotCard(ctx context.Context, aggregate Aggregate, actor bridge.Seat, legal []bridge.Card, engine *BotDecisionEngine) bridge.Card {
	state := *aggregate.Game
	fallback := rankedBotCards(state, actor, legal)[0]
	if engine == nil || engine.solver == nil {
		return fallback
	}
	decision, err := engine.decideCard(ctx, state, actor, legal)
	if err != nil {
		engine.options.Logger.WarnContext(ctx, "bot_decision_fallback", "seat", actor, "error", err, "selected_card", fallback.String())
		return fallback
	}
	if !slices.Contains(legal, decision.card) {
		engine.options.Logger.ErrorContext(ctx, "bot_decision_invalid_card", "seat", actor, "selected_card", decision.card.String(), "fallback_card", fallback.String())
		return fallback
	}
	engine.options.Logger.InfoContext(ctx, "bot_decision",
		"seat", actor,
		"selected_card", decision.card.String(),
		"samples", decision.samples,
		"evaluated_candidates", decision.evaluated,
		"dds_calls", decision.ddsCalls,
		"cache_hits", decision.cacheHits,
		"cache_misses", decision.cacheMisses,
		"expected_tricks", decision.expectedTricks,
		"contract_probability", decision.contractChance,
		"confidence", decision.confidence,
	)
	return decision.card
}

func (engine *BotDecisionEngine) decideCard(ctx context.Context, state bridge.State, actor bridge.Seat, legal []bridge.Card) (botDecision, error) {
	ctx, cancel := context.WithTimeout(ctx, engine.options.TimeBudget)
	defer cancel()
	candidates := rankedBotCards(state, actor, legal)
	if len(candidates) > engine.options.MaxCandidates {
		candidates = candidates[:engine.options.MaxCandidates]
	}
	stats := make(map[bridge.Card]*botCardStats, len(candidates))
	for _, card := range candidates {
		stats[card] = &botCardStats{card: card, heuristic: botCardHeuristic(state, actor, card)}
	}

	completedSamples := 0
	ddsCalls := 0
	cacheHits := 0
	cacheMisses := 0
	for _, budget := range engine.options.ProgressiveSamples {
		if budget > engine.options.SampleCount {
			budget = engine.options.SampleCount
		}
		for completedSamples < budget {
			if err := ctx.Err(); err != nil {
				break
			}
			sampledState, err := sampleBotState(state, actor, completedSamples)
			if err != nil {
				return botDecision{}, err
			}
			predictions, hit, err := engine.solveCached(ctx, sampledState)
			if err != nil {
				completedSamples++
				continue
			}
			if hit {
				cacheHits++
			} else {
				cacheMisses++
				ddsCalls++
			}
			for _, prediction := range predictions {
				stat := stats[prediction.Card]
				if stat == nil {
					continue
				}
				stat.expected += float64(prediction.Tricks)
				stat.samples++
				if botCardMeetsObjective(state, actor, prediction.Tricks) {
					stat.successes++
				}
			}
			completedSamples++
		}
		if completedSamples >= 2 && botDecisionHasClearLeader(stats, completedSamples) {
			break
		}
	}
	if completedSamples == 0 {
		return botDecision{}, fmt.Errorf("DDS produced no usable bot evaluations")
	}

	best, second := bestBotStats(stats)
	if best.samples == 0 {
		return botDecision{}, fmt.Errorf("DDS produced no predictions for legal candidates")
	}
	confidence := float64(best.samples)
	if second != nil && second.samples > 0 {
		confidence = (best.expected/float64(best.samples) - second.expected/float64(second.samples)) / 13
	}
	return botDecision{
		card:           best.card,
		samples:        completedSamples,
		evaluated:      len(stats),
		ddsCalls:       ddsCalls,
		cacheHits:      cacheHits,
		cacheMisses:    cacheMisses,
		expectedTricks: best.expected / float64(best.samples),
		contractChance: float64(best.successes) / float64(best.samples),
		confidence:     confidence,
	}, nil
}

func (engine *BotDecisionEngine) solveCached(ctx context.Context, state bridge.State) ([]analysis.CardPrediction, bool, error) {
	key, err := botPositionKey(state)
	if err != nil {
		return nil, false, err
	}
	engine.cacheMutex.Lock()
	if predictions, ok := engine.cache[key]; ok {
		copyPredictions := append([]analysis.CardPrediction(nil), predictions...)
		engine.cacheMutex.Unlock()
		return copyPredictions, true, nil
	}
	engine.cacheMutex.Unlock()

	predictions, err := engine.solver.SolvePosition(ctx, state)
	if err != nil {
		return nil, false, err
	}
	copyPredictions := append([]analysis.CardPrediction(nil), predictions...)
	engine.cacheMutex.Lock()
	if len(engine.cache) >= engine.options.CacheCapacity {
		oldest := engine.cacheOrder[0]
		delete(engine.cache, oldest)
		engine.cacheOrder = engine.cacheOrder[1:]
	}
	engine.cache[key] = copyPredictions
	engine.cacheOrder = append(engine.cacheOrder, key)
	engine.cacheMutex.Unlock()
	return append([]analysis.CardPrediction(nil), copyPredictions...), false, nil
}

func botPositionKey(state bridge.State) (string, error) {
	encoded, err := json.Marshal(state)
	if err != nil {
		return "", fmt.Errorf("encode bot position: %w", err)
	}
	digest := sha256.Sum256(encoded)
	return fmt.Sprintf("%x", digest), nil
}

func bestBotStats(stats map[bridge.Card]*botCardStats) (*botCardStats, *botCardStats) {
	ordered := make([]*botCardStats, 0, len(stats))
	for _, stat := range stats {
		ordered = append(ordered, stat)
	}
	sort.SliceStable(ordered, func(_leftIndex, _rightIndex int) bool {
		leftValue := botCardUtility(ordered[_leftIndex])
		rightValue := botCardUtility(ordered[_rightIndex])
		if leftValue != rightValue {
			return leftValue > rightValue
		}
		if ordered[_leftIndex].heuristic != ordered[_rightIndex].heuristic {
			return ordered[_leftIndex].heuristic > ordered[_rightIndex].heuristic
		}
		return ordered[_leftIndex].card.String() < ordered[_rightIndex].card.String()
	})
	if len(ordered) == 0 {
		return nil, nil
	}
	var second *botCardStats
	if len(ordered) > 1 {
		second = ordered[1]
	}
	return ordered[0], second
}

func botCardUtility(stat *botCardStats) float64 {
	if stat.samples == 0 {
		return -1e9
	}
	contractProbability := float64(stat.successes) / float64(stat.samples)
	expectedTricks := stat.expected / float64(stat.samples)
	return contractProbability*100 + expectedTricks/13 + stat.heuristic/100
}

func botDecisionHasClearLeader(stats map[bridge.Card]*botCardStats, samples int) bool {
	best, second := bestBotStats(stats)
	if best == nil || second == nil || best.samples != samples || second.samples != samples {
		return false
	}
	return botCardUtility(best)-botCardUtility(second) >= 2
}

func botCardMeetsObjective(state bridge.State, actor bridge.Seat, tricks int) bool {
	contract := state.Auction.Contract
	if contract == nil {
		return false
	}
	if actor.Partnership() == contract.Declarer.Partnership() {
		return tricks >= contract.TargetTricks()
	}
	return 13-tricks < contract.TargetTricks()
}

func rankedBotCards(state bridge.State, actor bridge.Seat, legal []bridge.Card) []bridge.Card {
	ordered := append([]bridge.Card(nil), legal...)
	sort.SliceStable(ordered, func(_leftIndex int, _rightIndex int) bool {
		leftScore := botCardHeuristic(state, actor, ordered[_leftIndex])
		rightScore := botCardHeuristic(state, actor, ordered[_rightIndex])
		if leftScore != rightScore {
			return leftScore > rightScore
		}
		return ordered[_leftIndex].String() < ordered[_rightIndex].String()
	})
	return ordered
}

func botCardHeuristic(state bridge.State, actor bridge.Seat, card bridge.Card) float64 {
	score := float64(botRankValue(card.Rank))
	handSeat := actor
	if state.Auction.Contract != nil && state.Turn == state.Auction.Contract.Dummy() {
		handSeat = state.Turn
	}
	hand := state.Deal.Hand(handSeat)
	suitLength := 0
	for _, held := range hand {
		if held.Suit == card.Suit {
			suitLength++
		}
	}
	score += float64(suitLength) * 1.5
	if len(state.CurrentTrick.Plays) == 0 {
		if card.Rank == bridge.Ace || card.Rank == bridge.King {
			score += 4
		}
		if suitLength >= 4 {
			score += 3
		}
	} else if botCardWinsCurrentTrick(state, card) {
		score += 30
	} else {
		score -= float64(botRankValue(card.Rank)) / 2
	}
	return score
}

func botCardWinsCurrentTrick(state bridge.State, card bridge.Card) bool {
	if len(state.CurrentTrick.Plays) == 0 || state.Auction.Contract == nil {
		return false
	}
	ledSuit := state.CurrentTrick.Plays[0].Card.Suit
	trump := state.Auction.Contract.Strain
	best := state.CurrentTrick.Plays[0].Card
	for _, play := range state.CurrentTrick.Plays[1:] {
		if botCardOutranks(play.Card, best, ledSuit, trump) {
			best = play.Card
		}
	}
	return botCardOutranks(card, best, ledSuit, trump)
}

func botCardOutranks(candidate bridge.Card, current bridge.Card, ledSuit bridge.Suit, trump bridge.Strain) bool {
	trumpSuit := bridge.Suit("")
	switch trump {
	case bridge.StrainClubs:
		trumpSuit = bridge.Clubs
	case bridge.StrainDiamonds:
		trumpSuit = bridge.Diamonds
	case bridge.StrainHearts:
		trumpSuit = bridge.Hearts
	case bridge.StrainSpades:
		trumpSuit = bridge.Spades
	}
	candidateTrump := trumpSuit != "" && candidate.Suit == trumpSuit
	currentTrump := trumpSuit != "" && current.Suit == trumpSuit
	if candidateTrump != currentTrump {
		return candidateTrump
	}
	if candidate.Suit != current.Suit {
		return candidate.Suit == ledSuit && current.Suit != ledSuit
	}
	return botRankValue(candidate.Rank) > botRankValue(current.Rank)
}

func botRankValue(rank bridge.Rank) int {
	switch rank {
	case bridge.Two:
		return 2
	case bridge.Three:
		return 3
	case bridge.Four:
		return 4
	case bridge.Five:
		return 5
	case bridge.Six:
		return 6
	case bridge.Seven:
		return 7
	case bridge.Eight:
		return 8
	case bridge.Nine:
		return 9
	case bridge.Ten:
		return 10
	case bridge.Jack:
		return 11
	case bridge.Queen:
		return 12
	case bridge.King:
		return 13
	case bridge.Ace:
		return 14
	default:
		return 0
	}
}

func chooseBotCall(state bridge.State, actor bridge.Seat, legal []bridge.Call) bridge.Call {
	ordered := append([]bridge.Call(nil), legal...)
	sort.SliceStable(ordered, func(_leftIndex int, _rightIndex int) bool {
		leftScore := botCallHeuristic(state, actor, ordered[_leftIndex])
		rightScore := botCallHeuristic(state, actor, ordered[_rightIndex])
		if leftScore != rightScore {
			return leftScore > rightScore
		}
		return callSortKey(ordered[_leftIndex]) < callSortKey(ordered[_rightIndex])
	})
	return ordered[0]
}

func botCallHeuristic(state bridge.State, actor bridge.Seat, call bridge.Call) float64 {
	hcp := 0
	lengths := map[bridge.Suit]int{}
	for _, card := range state.Deal.Hand(actor) {
		switch card.Rank {
		case bridge.Ace:
			hcp += 4
		case bridge.King:
			hcp += 3
		case bridge.Queen:
			hcp += 2
		case bridge.Jack:
			hcp++
		}
		lengths[card.Suit]++
	}
	switch call.Kind {
	case bridge.CallPass:
		if hcp < 11 {
			return 3
		}
		return 0
	case bridge.CallBid:
		score := float64(hcp-10) - float64(call.Level-1)*4
		score += float64(lengths[strainSuitForCall(call.Strain)]) * 1.5
		return score
	case bridge.CallDouble:
		return float64(hcp-15) / 2
	case bridge.CallRedouble:
		return float64(hcp-15) / 2
	default:
		return -100
	}
}

func strainSuitForCall(strain bridge.Strain) bridge.Suit {
	switch strain {
	case bridge.StrainClubs:
		return bridge.Clubs
	case bridge.StrainDiamonds:
		return bridge.Diamonds
	case bridge.StrainHearts:
		return bridge.Hearts
	case bridge.StrainSpades:
		return bridge.Spades
	default:
		return ""
	}
}

func callSortKey(call bridge.Call) string {
	return string(call.Kind) + string(call.Strain) + fmt.Sprint(call.Level)
}

func sampleBotState(state bridge.State, actor bridge.Seat, sampleIndex int) (bridge.State, error) {
	knownSeats := map[bridge.Seat]bool{actor: true}
	if state.Auction.Contract != nil && state.DummyRevealed {
		knownSeats[state.Auction.Contract.Dummy()] = true
	}
	played := make(map[bridge.Card]bool, 52)
	playedBySeat := make(map[bridge.Seat]int, 4)
	voids := make(map[bridge.Seat]map[bridge.Suit]bool, 4)
	for _, seat := range []bridge.Seat{bridge.North, bridge.East, bridge.South, bridge.West} {
		voids[seat] = make(map[bridge.Suit]bool, 4)
	}
	tricks := append([]bridge.Trick(nil), state.CompletedTricks...)
	tricks = append(tricks, state.CurrentTrick)
	for _, trick := range tricks {
		if len(trick.Plays) == 0 {
			continue
		}
		ledSuit := trick.Plays[0].Card.Suit
		for _, play := range trick.Plays {
			played[play.Card] = true
			playedBySeat[play.Seat]++
			if play.Card.Suit != ledSuit {
				voids[play.Seat][ledSuit] = true
			}
		}
	}

	deck := make([]bridge.Card, 0, 52)
	for _, card := range bridge.FullDeck() {
		if played[card] {
			continue
		}
		known := false
		for seat := range knownSeats {
			if containsCard(state.Deal.Hand(seat), card) {
				known = true
				break
			}
		}
		if !known {
			deck = append(deck, card)
		}
	}

	hiddenSeats := make([]bridge.Seat, 0, 3)
	for _, seat := range []bridge.Seat{bridge.North, bridge.East, bridge.South, bridge.West} {
		if !knownSeats[seat] {
			hiddenSeats = append(hiddenSeats, seat)
		}
	}
	seed := botSampleSeed(state, actor, sampleIndex)
	random := rand.New(rand.NewSource(seed))
	for _attempt := 0; _attempt < 128; _attempt++ {
		shuffled := append([]bridge.Card(nil), deck...)
		random.Shuffle(len(shuffled), func(_leftIndex, _rightIndex int) {
			shuffled[_leftIndex], shuffled[_rightIndex] = shuffled[_rightIndex], shuffled[_leftIndex]
		})
		deal := bridge.Deal{}
		for seat := range knownSeats {
			setBotHand(&deal, seat, state.Deal.Hand(seat))
		}
		cursor := 0
		valid := true
		for _, seat := range hiddenSeats {
			remaining := 13 - playedBySeat[seat]
			if cursor+remaining > len(shuffled) {
				valid = false
				break
			}
			hand := append([]bridge.Card(nil), shuffled[cursor:cursor+remaining]...)
			cursor += remaining
			for _, card := range hand {
				if voids[seat][card.Suit] {
					valid = false
					break
				}
			}
			if !valid {
				break
			}
			setBotHand(&deal, seat, hand)
		}
		if valid && cursor == len(shuffled) {
			sampled := state
			sampled.Deal = deal
			return sampled, nil
		}
	}
	return bridge.State{}, fmt.Errorf("unable to sample a legal hidden deal")
}

func botSampleSeed(state bridge.State, actor bridge.Seat, sampleIndex int) int64 {
	visibleDeal := bridge.Deal{}
	setBotHand(&visibleDeal, actor, state.Deal.Hand(actor))
	if state.Auction.Contract != nil && state.DummyRevealed {
		setBotHand(&visibleDeal, state.Auction.Contract.Dummy(), state.Deal.Hand(state.Auction.Contract.Dummy()))
	}
	encoded, _ := json.Marshal(struct {
		Board           bridge.BoardMetadata
		Auction         bridge.Auction
		Turn            bridge.Seat
		DummyRevealed   bool
		CurrentTrick    bridge.Trick
		CompletedTricks []bridge.Trick
		TricksNS        int
		TricksEW        int
		VisibleDeal     bridge.Deal
		Actor           bridge.Seat
		SampleIndex     int
	}{
		Board:           state.Board,
		Auction:         state.Auction,
		Turn:            state.Turn,
		DummyRevealed:   state.DummyRevealed,
		CurrentTrick:    state.CurrentTrick,
		CompletedTricks: state.CompletedTricks,
		TricksNS:        state.TricksNS,
		TricksEW:        state.TricksEW,
		VisibleDeal:     visibleDeal,
		Actor:           actor,
		SampleIndex:     sampleIndex,
	})
	digest := sha256.Sum256(encoded)
	return int64(binaryLittleEndianUint64(digest[:8]))
}

func binaryLittleEndianUint64(value []byte) uint64 {
	return uint64(value[0]) | uint64(value[1])<<8 | uint64(value[2])<<16 | uint64(value[3])<<24 |
		uint64(value[4])<<32 | uint64(value[5])<<40 | uint64(value[6])<<48 | uint64(value[7])<<56
}

func containsCard(hand bridge.Hand, card bridge.Card) bool {
	for _, held := range hand {
		if held == card {
			return true
		}
	}
	return false
}

func setBotHand(deal *bridge.Deal, seat bridge.Seat, hand bridge.Hand) {
	copyHand := append(bridge.Hand(nil), hand...)
	switch seat {
	case bridge.North:
		deal.North = copyHand
	case bridge.East:
		deal.East = copyHand
	case bridge.South:
		deal.South = copyHand
	case bridge.West:
		deal.West = copyHand
	}
}
