package bridge

import "testing"

func TestMetadataForBoard(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		boardNumber   int
		dealer        Seat
		vulnerability Vulnerability
	}{
		{name: "board 1", boardNumber: 1, dealer: North, vulnerability: VulnerabilityNone},
		{name: "board 4", boardNumber: 4, dealer: West, vulnerability: VulnerabilityBoth},
		{name: "board 8", boardNumber: 8, dealer: West, vulnerability: VulnerabilityNone},
		{name: "board 13", boardNumber: 13, dealer: North, vulnerability: VulnerabilityBoth},
		{name: "board 16", boardNumber: 16, dealer: West, vulnerability: VulnerabilityEW},
		{name: "board 17 repeats", boardNumber: 17, dealer: North, vulnerability: VulnerabilityNone},
		{name: "board 32 repeats", boardNumber: 32, dealer: West, vulnerability: VulnerabilityEW},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			metadata, err := MetadataForBoard(test.boardNumber)
			if err != nil {
				t.Fatalf("MetadataForBoard(%d) error = %v", test.boardNumber, err)
			}
			if metadata.Dealer != test.dealer || metadata.Vulnerability != test.vulnerability {
				t.Errorf("MetadataForBoard(%d) = %+v, want dealer %s vulnerability %s", test.boardNumber, metadata, test.dealer, test.vulnerability)
			}
		})
	}
}

func TestMetadataForBoardRejectsNonPositiveNumber(t *testing.T) {
	t.Parallel()

	for _, boardNumber := range []int{0, -1} {
		if _, err := MetadataForBoard(boardNumber); err == nil {
			t.Errorf("MetadataForBoard(%d) error = nil", boardNumber)
		}
	}
}

func TestVulnerabilityIsVulnerable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		vulnerability Vulnerability
		partnership   Partnership
		want          bool
	}{
		{name: "none NS", vulnerability: VulnerabilityNone, partnership: NorthSouth},
		{name: "NS vulnerable", vulnerability: VulnerabilityNS, partnership: NorthSouth, want: true},
		{name: "NS leaves EW safe", vulnerability: VulnerabilityNS, partnership: EastWest},
		{name: "EW vulnerable", vulnerability: VulnerabilityEW, partnership: EastWest, want: true},
		{name: "both NS", vulnerability: VulnerabilityBoth, partnership: NorthSouth, want: true},
		{name: "both EW", vulnerability: VulnerabilityBoth, partnership: EastWest, want: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := test.vulnerability.IsVulnerable(test.partnership); got != test.want {
				t.Errorf("IsVulnerable(%s) = %v, want %v", test.partnership, got, test.want)
			}
		})
	}
}
