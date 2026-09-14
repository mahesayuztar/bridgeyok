package match

import (
	"fmt"
	"math"
	"testing"
)

func TestCompareScores(t *testing.T) {
	t.Parallel()
	ranges := [][3]int{{0, 10, 0}, {20, 40, 1}, {50, 80, 2}, {90, 120, 3}, {130, 160, 4}, {170, 210, 5}, {220, 260, 6}, {270, 310, 7}, {320, 360, 8}, {370, 420, 9}, {430, 490, 10}, {500, 590, 11}, {600, 740, 12}, {750, 890, 13}, {900, 1090, 14}, {1100, 1290, 15}, {1300, 1490, 16}, {1500, 1740, 17}, {1750, 1990, 18}, {2000, 2240, 19}, {2250, 2490, 20}, {2500, 2990, 21}, {3000, 3490, 22}, {3500, 3990, 23}, {4000, 7600, 24}}
	for _, interval := range ranges {
		for _, score := range []int{interval[0], interval[1]} {
			for _, sign := range []int{-1, 1} {
				t.Run(fmt.Sprintf("%d", sign*score), func(t *testing.T) {
					actual, err := CompareScores(sign*score, 0)
					if err != nil || actual != sign*interval[2] {
						t.Fatalf("got %d, %v; want %d", actual, err, sign*interval[2])
					}
				})
			}
		}
	}
	cases := []struct{ open, closed, want int }{{620, 170, 10}, {170, 620, -10}, {620, -620, 15}, {-620, 620, -15}, {0, 0, 0}, {420, 420, 0}, {7600, -7600, 24}, {-7600, 7600, -24}}
	for _, test := range cases {
		got, err := CompareScores(test.open, test.closed)
		if err != nil || got != test.want {
			t.Fatalf("%+v: %d %v", test, got, err)
		}
	}
	for _, invalid := range []int{math.MinInt, math.MaxInt, -7610, 7610, 1, 15, -25} {
		if _, err := CompareScores(invalid, 0); err == nil {
			t.Fatalf("accepted %d", invalid)
		}
		if _, err := CompareScores(0, invalid); err == nil {
			t.Fatalf("accepted closed %d", invalid)
		}
	}
}
