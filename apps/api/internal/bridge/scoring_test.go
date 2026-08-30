package bridge

import (
	"encoding/json"
	"os"
	"testing"
)

type goldenScoreCase struct {
	Name            string        `json:"name"`
	Level           int           `json:"level"`
	Strain          Strain        `json:"strain"`
	Doubling        Doubling      `json:"doubling"`
	Declarer        Seat          `json:"declarer"`
	Vulnerability   Vulnerability `json:"vulnerability"`
	Tricks          int           `json:"tricks"`
	DeclarerScore   int           `json:"declarerScore"`
	ExpectedScoreNS int           `json:"scoreNS"`
}

type goldenScoreMatrix struct {
	SourceURL string            `json:"sourceUrl"`
	Law       string            `json:"law"`
	Cases     []goldenScoreCase `json:"cases"`
}

func TestScoreContractGoldenMatrix(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("testdata/duplicate_scores.json")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	var matrix goldenScoreMatrix
	if err := json.Unmarshal(data, &matrix); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if matrix.SourceURL != "https://www.worldbridge.org/wp-content/uploads/2017/03/2017LawsofDuplicateBridge-nohighlights.pdf" || matrix.Law != "77" {
		t.Fatalf("golden matrix provenance = %q Law %q", matrix.SourceURL, matrix.Law)
	}
	tests := matrix.Cases
	if len(tests) < 30 {
		t.Fatalf("golden matrix has %d cases, want at least 30", len(tests))
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			t.Parallel()
			contract := Contract{
				Level:    test.Level,
				Strain:   test.Strain,
				Doubling: test.Doubling,
				Declarer: test.Declarer,
			}
			result, err := ScoreContract(contract, test.Vulnerability, test.Tricks)
			if err != nil {
				t.Fatalf("ScoreContract() error = %v", err)
			}
			declarerScore := result.ScoreNS
			if contract.Declarer.Partnership() == EastWest {
				declarerScore = -declarerScore
			}
			if declarerScore != test.DeclarerScore {
				t.Errorf("declarer score = %d, want %d", declarerScore, test.DeclarerScore)
			}
			if result.ScoreNS != test.ExpectedScoreNS {
				t.Errorf("ScoreNS = %d, want %d", result.ScoreNS, test.ExpectedScoreNS)
			}
			if result.TricksNS+result.TricksEW != 13 {
				t.Errorf("partnership tricks = %d + %d, want 13", result.TricksNS, result.TricksEW)
			}
			if err := result.Validate(); err != nil {
				t.Errorf("Validate() error = %v", err)
			}
		})
	}
}

func TestScoreContractRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	valid := Contract{Level: 4, Strain: StrainHearts, Doubling: Undoubled, Declarer: North}
	tests := []struct {
		name          string
		contract      Contract
		vulnerability Vulnerability
		tricks        int
	}{
		{name: "invalid contract", contract: Contract{Level: 8, Strain: StrainHearts, Doubling: Undoubled, Declarer: North}, vulnerability: VulnerabilityNone, tricks: 13},
		{name: "invalid vulnerability", contract: valid, vulnerability: "INVALID", tricks: 10},
		{name: "negative tricks", contract: valid, vulnerability: VulnerabilityNone, tricks: -1},
		{name: "too many tricks", contract: valid, vulnerability: VulnerabilityNone, tricks: 14},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, err := ScoreContract(test.contract, test.vulnerability, test.tricks); err == nil {
				t.Fatal("ScoreContract() error = nil")
			}
		})
	}
}

func TestPassedOutResult(t *testing.T) {
	t.Parallel()

	result, err := PassedOutResult(VulnerabilityBoth)
	if err != nil {
		t.Fatalf("PassedOutResult() error = %v", err)
	}
	if !result.PassedOut || result.ScoreNS != 0 || result.Contract != nil || result.RulesetVersion != RulesetVersion {
		t.Fatalf("PassedOutResult() = %+v", result)
	}
	if err := result.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if _, err := PassedOutResult("INVALID"); err == nil {
		t.Fatal("PassedOutResult(invalid) error = nil")
	}
}

func TestScoreContractPartnershipSymmetry(t *testing.T) {
	t.Parallel()

	for level := 1; level <= 7; level++ {
		for _, strain := range strains {
			for _, doubling := range []Doubling{Undoubled, Doubled, Redoubled} {
				for tricks := 0; tricks <= 13; tricks++ {
					north, err := ScoreContract(Contract{Level: level, Strain: strain, Doubling: doubling, Declarer: North}, VulnerabilityBoth, tricks)
					if err != nil {
						t.Fatalf("ScoreContract(N) error = %v", err)
					}
					east, err := ScoreContract(Contract{Level: level, Strain: strain, Doubling: doubling, Declarer: East}, VulnerabilityBoth, tricks)
					if err != nil {
						t.Fatalf("ScoreContract(E) error = %v", err)
					}
					if north.ScoreNS != -east.ScoreNS {
						t.Fatalf("level %d %s %s tricks %d: N score %d, E score %d", level, strain, doubling, tricks, north.ScoreNS, east.ScoreNS)
					}
				}
			}
		}
	}
}

func TestResultValidateRejectsTampering(t *testing.T) {
	t.Parallel()

	result, err := ScoreContract(Contract{Level: 4, Strain: StrainSpades, Doubling: Doubled, Declarer: North}, VulnerabilityNS, 8)
	if err != nil {
		t.Fatalf("ScoreContract() error = %v", err)
	}

	tests := []struct {
		name   string
		mutate func(Result) Result
	}{
		{name: "ruleset", mutate: func(result Result) Result { result.RulesetVersion = "v2"; return result }},
		{name: "score", mutate: func(result Result) Result { result.ScoreNS++; return result }},
		{name: "tricks", mutate: func(result Result) Result { result.TricksEW++; return result }},
		{name: "missing contract", mutate: func(result Result) Result { result.Contract = nil; return result }},
		{name: "false passed out", mutate: func(result Result) Result { result.PassedOut = true; return result }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if err := test.mutate(result).Validate(); err == nil {
				t.Fatal("Validate() error = nil")
			}
		})
	}
}

func BenchmarkScoreContract(b *testing.B) {
	contract := Contract{Level: 4, Strain: StrainSpades, Doubling: Doubled, Declarer: North}
	for b.Loop() {
		_, _ = ScoreContract(contract, VulnerabilityNS, 10)
	}
}
