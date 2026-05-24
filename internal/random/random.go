// Package random provides a cryptographically-secure source compatible with
// math/rand.Source64. It also exposes a small RandRange helper.
package random

import (
	crand "crypto/rand"
	"math/big"
	"math/rand"
)

type secureRandomSource struct{}

// Source is a process-global source backed by crypto/rand. It satisfies
// math/rand.Source64, so it can be passed to rand.New(...).
var Source secureRandomSource

var (
	int63Limit = big.NewInt(0).Lsh(big.NewInt(1), 63)
	int64Limit = big.NewInt(0).Lsh(big.NewInt(1), 64)
)

func (secureRandomSource) Seed(_ int64) {}

func (secureRandomSource) Int63() int64 {
	n, err := crand.Int(crand.Reader, int63Limit)
	if err != nil {
		panic(err)
	}
	return n.Int64()
}

func (secureRandomSource) Uint64() uint64 {
	n, err := crand.Int(crand.Reader, int64Limit)
	if err != nil {
		panic(err)
	}
	return n.Uint64()
}

// RandRange returns a uniformly distributed random integer in [low, hi].
// Panics if low >= hi.
func RandRange(low, hi int64) int64 {
	if low >= hi {
		panic("random.RandRange: low boundary is greater or equal to high boundary")
	}
	delta := hi - low
	return low + rand.New(Source).Int63n(delta+1)
}
