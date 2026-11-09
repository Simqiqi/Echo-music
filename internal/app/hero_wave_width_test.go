package app

import "testing"

// A lead record too narrow for both gives the wave less room, and none at
// all before the words would have to share their line with it: the words
// once stood up in a column of one letter, pressed against the bars.
func TestHeroWaveWidthLeavesTheWordsTheirLine(t *testing.T) {
	// What the card owes the words and its own edges, before the wave.
	owed := float32(heroWordsLeft + heroWordsMin + heroWaveGap + heroWaveEdge)
	cases := []struct {
		name  string
		cardW float32
		want  float32
	}{
		{"nothing to spare", owed, 0},
		{"a sliver is no wave", owed + heroWaveBarsMin - 1, 0},
		{"bars alone", owed + heroWaveBarsMin, heroWaveBarsMin},
		{"all of it", owed + heroWaveW + 300, heroWaveW},
	}
	for _, c := range cases {
		if got := heroWaveWidth(c.cardW); got != c.want {
			t.Errorf("%s: a card %v wide gives the wave %v, want %v", c.name, c.cardW, got, c.want)
		}
	}
	// Whatever the card, the wave and the words never overlap.
	for w := float32(200); w < 1600; w++ {
		wave := heroWaveWidth(w)
		if wave == 0 {
			continue
		}
		if line := w - (owed - heroWordsMin) - wave; line < heroWordsMin {
			t.Fatalf("a card %v wide leaves the words %v for a wave of %v", w, line, wave)
		}
	}
}
