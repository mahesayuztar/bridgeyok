package bridge

import "fmt"

// RulesetVersion is persisted with every result so history remains reproducible.
const RulesetVersion = "bridgeyok_duplicate_v1"

// Result is the canonical duplicate score and its complete scoring inputs.
type Result struct {
	RulesetVersion string        `json:"rulesetVersion"`
	PassedOut      bool          `json:"passedOut"`
	Contract       *Contract     `json:"contract,omitempty"`
	TricksDeclarer int           `json:"tricksDeclarer"`
	TricksNS       int           `json:"tricksNS"`
	TricksEW       int           `json:"tricksEW"`
	Vulnerability  Vulnerability `json:"vulnerability"`
	ScoreNS        int           `json:"scoreNS"`
}

// ScoreContract calculates duplicate score from declarer's result.
func ScoreContract(contract Contract, vulnerability Vulnerability, tricksDeclarer int) (Result, error) {
	if err := contract.Validate(); err != nil {
		return Result{}, fmt.Errorf("contract: %w", err)
	}
	if !vulnerability.Valid() {
		return Result{}, fmt.Errorf("invalid vulnerability %q", vulnerability)
	}
	if tricksDeclarer < 0 || tricksDeclarer > 13 {
		return Result{}, fmt.Errorf("declarer tricks %d outside 0 through 13", tricksDeclarer)
	}

	declarerVulnerable := vulnerability.IsVulnerable(contract.Declarer.Partnership())
	declarerScore := calculateDeclarerScore(contract, declarerVulnerable, tricksDeclarer)
	scoreNS := declarerScore
	tricksNS := tricksDeclarer
	tricksEW := 13 - tricksDeclarer
	if contract.Declarer.Partnership() == EastWest {
		scoreNS = -declarerScore
		tricksNS, tricksEW = tricksEW, tricksNS
	}

	contractCopy := contract
	return Result{
		RulesetVersion: RulesetVersion,
		Contract:       &contractCopy,
		TricksDeclarer: tricksDeclarer,
		TricksNS:       tricksNS,
		TricksEW:       tricksEW,
		Vulnerability:  vulnerability,
		ScoreNS:        scoreNS,
	}, nil
}

// PassedOutResult creates the canonical zero result for an auction with no bid.
func PassedOutResult(vulnerability Vulnerability) (Result, error) {
	if !vulnerability.Valid() {
		return Result{}, fmt.Errorf("invalid vulnerability %q", vulnerability)
	}
	return Result{
		RulesetVersion: RulesetVersion,
		PassedOut:      true,
		Vulnerability:  vulnerability,
	}, nil
}

func calculateDeclarerScore(contract Contract, vulnerable bool, tricksDeclarer int) int {
	target := contract.TargetTricks()
	if tricksDeclarer < target {
		return -undertrickPenalty(target-tricksDeclarer, contract.Doubling, vulnerable)
	}

	multiplier := doublingMultiplier(contract.Doubling)
	contractPoints := contractTrickPoints(contract.Level, contract.Strain) * multiplier
	score := contractPoints
	if contractPoints >= 100 {
		if vulnerable {
			score += 500
		} else {
			score += 300
		}
	} else {
		score += 50
	}

	if contract.Level == 6 {
		if vulnerable {
			score += 750
		} else {
			score += 500
		}
	}
	if contract.Level == 7 {
		if vulnerable {
			score += 1500
		} else {
			score += 1000
		}
	}

	overtricks := tricksDeclarer - target
	if overtricks > 0 {
		score += overtricks * overtrickValue(contract.Strain, contract.Doubling, vulnerable)
	}
	switch contract.Doubling {
	case Doubled:
		score += 50
	case Redoubled:
		score += 100
	case Undoubled:
	}
	return score
}

func contractTrickPoints(level int, strain Strain) int {
	switch strain {
	case StrainClubs, StrainDiamonds:
		return level * 20
	case StrainHearts, StrainSpades:
		return level * 30
	case StrainNoTrump:
		return 40 + (level-1)*30
	default:
		return 0
	}
}

func doublingMultiplier(doubling Doubling) int {
	switch doubling {
	case Doubled:
		return 2
	case Redoubled:
		return 4
	default:
		return 1
	}
}

func overtrickValue(strain Strain, doubling Doubling, vulnerable bool) int {
	switch doubling {
	case Doubled:
		if vulnerable {
			return 200
		}
		return 100
	case Redoubled:
		if vulnerable {
			return 400
		}
		return 200
	default:
		if strain == StrainClubs || strain == StrainDiamonds {
			return 20
		}
		return 30
	}
}

func undertrickPenalty(undertricks int, doubling Doubling, vulnerable bool) int {
	if doubling == Undoubled {
		if vulnerable {
			return undertricks * 100
		}
		return undertricks * 50
	}

	penalty := 0
	if vulnerable {
		penalty = 200 + (undertricks-1)*300
	} else {
		penalty = 100
		if undertricks >= 2 {
			penalty += 200
		}
		if undertricks >= 3 {
			penalty += 200
		}
		if undertricks >= 4 {
			penalty += (undertricks - 3) * 300
		}
	}
	if doubling == Redoubled {
		penalty *= 2
	}
	return penalty
}

// Validate recalculates a result and rejects incomplete or inconsistent score data.
func (result Result) Validate() error {
	if result.RulesetVersion != RulesetVersion {
		return fmt.Errorf("unsupported ruleset version %q", result.RulesetVersion)
	}
	if result.PassedOut {
		expected, err := PassedOutResult(result.Vulnerability)
		if err != nil {
			return err
		}
		if result.Contract != nil || result.TricksDeclarer != 0 || result.TricksNS != 0 || result.TricksEW != 0 || result.ScoreNS != expected.ScoreNS {
			return fmt.Errorf("passed-out result contains play or score data")
		}
		return nil
	}
	if result.Contract == nil {
		return fmt.Errorf("scored result requires a contract")
	}
	expected, err := ScoreContract(*result.Contract, result.Vulnerability, result.TricksDeclarer)
	if err != nil {
		return err
	}
	if result.TricksNS != expected.TricksNS || result.TricksEW != expected.TricksEW || result.ScoreNS != expected.ScoreNS {
		return fmt.Errorf("result score does not match contract inputs")
	}
	return nil
}
