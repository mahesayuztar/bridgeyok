package match

import "fmt"

// CompareScores returns signed Team A IMPs; Team A occupies NS in open and EW in closed.
func CompareScores(openScoreNS, closedScoreNS int) (int, error) {
	if openScoreNS < -7600 || openScoreNS > 7600 || closedScoreNS < -7600 || closedScoreNS > 7600 || openScoreNS%10 != 0 || closedScoreNS%10 != 0 {
		return 0, fmt.Errorf("invalid duplicate score")
	}
	difference := openScoreNS - closedScoreNS
	sign := 1
	if difference < 0 {
		difference = -difference
		sign = -1
	}
	thresholds := [...]int{20, 50, 90, 130, 170, 220, 270, 320, 370, 430, 500, 600, 750, 900, 1100, 1300, 1500, 1750, 2000, 2250, 2500, 3000, 3500, 4000}
	imps := 0
	for _, threshold := range thresholds {
		if difference < threshold {
			break
		}
		imps++
	}
	return sign * imps, nil
}
