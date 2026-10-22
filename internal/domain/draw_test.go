package domain

import (
	"bytes"
	"testing"
)

func TestSelectWinnersUsesTicketsButSelectsEachUserOnce(t *testing.T) {
	winners, err := SelectWinners(bytes.NewReader(make([]byte, 16)), []int64{1, 1, 1, 2, 3}, 5)
	if err != nil || len(winners) != 3 || winners[0] != 1 || winners[1] != 2 || winners[2] != 3 {
		t.Fatalf("winners=%v, err=%v", winners, err)
	}
}
