package spin

import "math/rand/v2"

// Rand is the one random source the picker needs. Production uses the
// standard library's generator; tests pass a seeded one.
type Rand interface {
	// IntN returns a number in [0, n). n is always greater than zero.
	IntN(n int) int
}

type systemRand struct{}

func (systemRand) IntN(n int) int { return rand.IntN(n) }

// NewSystemRand returns the production random source. It is safe for use
// from many goroutines.
func NewSystemRand() Rand { return systemRand{} }

// Pick returns the index of one weight, chosen with probability equal to its
// share of the total. Weights that are not positive count as zero. It returns
// -1 when there is nothing to pick (no entries, or no positive weight).
func Pick(weights []int, r Rand) int {
	total := 0
	for _, w := range weights {
		if w > 0 {
			total += w
		}
	}
	if total <= 0 {
		return -1
	}
	n := r.IntN(total)
	for i, w := range weights {
		if w <= 0 {
			continue
		}
		if n < w {
			return i
		}
		n -= w
	}
	return -1 // not reachable: n < total
}

// PickOther picks an entry by weight from every entry except the one whose id
// is currentID, and returns its index in entries. When no other entry exists
// it returns ErrNoOther; it never panics.
func PickOther(entries []WheelEntry, currentID string, r Rand) (int, error) {
	indexes := make([]int, 0, len(entries))
	weights := make([]int, 0, len(entries))
	for i, e := range entries {
		if e.ID == currentID || e.Weight <= 0 {
			continue
		}
		indexes = append(indexes, i)
		weights = append(weights, e.Weight)
	}
	picked := Pick(weights, r)
	if picked < 0 {
		return -1, ErrNoOther
	}
	return indexes[picked], nil
}
