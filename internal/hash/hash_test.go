package hash

import (
	"testing"
)

func TestHashBasic(t *testing.T) {
	// Should return consistent results
	a := Hash([]byte("hello"), 0, 1024)
	b := Hash([]byte("hello"), 0, 1024)
	if a != b {
		t.Errorf("Hash not deterministic: %d != %d", a, b)
	}
	// Different i should (usually) give different index
	c := Hash([]byte("hello"), 1, 1024)
	if a == c {
		t.Log("Hash(hello, 0) == Hash(hello, 1) — unlikely but not impossible")
	}
}

func TestHashZeroM(t *testing.T) {
	idx := Hash([]byte("data"), 0, 0)
	if idx != 0 {
		t.Errorf("Hash with m=0 should return 0, got %d", idx)
	}
}

func TestMurmur3HashBasic(t *testing.T) {
	a := Murmur3Hash([]byte("test"), 0, 4096)
	b := Murmur3Hash([]byte("test"), 0, 4096)
	if a != b {
		t.Errorf("Murmur3Hash not deterministic: %d != %d", a, b)
	}
	if a >= 4096 {
		t.Errorf("Murmur3Hash(%q, 0, 4096) = %d, want < 4096", "test", a)
	}
}

func TestMurmur3HashZeroM(t *testing.T) {
	idx := Murmur3Hash([]byte("x"), 0, 0)
	if idx != 0 {
		t.Errorf("Murmur3Hash with m=0 = %d, want 0", idx)
	}
}

func TestEnhancedDoubleHash(t *testing.T) {
	m := uint(8192)
	for i := uint(0); i < 10; i++ {
		idx := EnhancedDoubleHash([]byte("item"), i, m)
		if idx >= m {
			t.Errorf("EnhancedDoubleHash(..., %d, %d) = %d, out of range", i, m, idx)
		}
	}
}

func TestTripleHash(t *testing.T) {
	m := uint(4096)
	idx := TripleHash([]byte("abc"), 3, m)
	if idx >= m {
		t.Errorf("TripleHash out of range: %d >= %d", idx, m)
	}
}

func TestHashWithStrategy(t *testing.T) {
	strategies := []Strategy{StrategyFNV, StrategyMurmur3, StrategyEnhancedDouble, StrategyTriple}
	for _, s := range strategies {
		idx := HashWithStrategy([]byte("data"), 2, 2048, s)
		if idx >= 2048 {
			t.Errorf("strategy %d: index %d out of range", s, idx)
		}
	}
}

func TestMultiHash(t *testing.T) {
	positions := MultiHash([]byte("multi"), 7, 4096, StrategyFNV)
	if len(positions) != 7 {
		t.Errorf("MultiHash returned %d positions, want 7", len(positions))
	}
	for i, p := range positions {
		if p >= 4096 {
			t.Errorf("positions[%d] = %d, out of range", i, p)
		}
	}
}

func TestIndependentHashes(t *testing.T) {
	positions := IndependentHashes([]byte("indep"), 5, 8192)
	if len(positions) != 5 {
		t.Errorf("IndependentHashes returned %d, want 5", len(positions))
	}
	for i, p := range positions {
		if p >= 8192 {
			t.Errorf("positions[%d] = %d, out of range", i, p)
		}
	}
}

func TestDispersion(t *testing.T) {
	score := Dispersion([]byte("test-dispersion"), 7, 8192, StrategyFNV)
	if score < 0 || score > 1 {
		t.Errorf("Dispersion score = %f, out of [0, 1]", score)
	}
}

func TestMurmur3Sum128Deterministic(t *testing.T) {
	h1a, h2a := Murmur3Sum128([]byte("deterministic"), 42)
	h1b, h2b := Murmur3Sum128([]byte("deterministic"), 42)
	if h1a != h1b || h2a != h2b {
		t.Error("Murmur3Sum128 not deterministic")
	}
	// Different seed should give different result
	h1c, _ := Murmur3Sum128([]byte("deterministic"), 99)
	if h1a == h1c {
		t.Log("Same hash with different seed — unlikely")
	}
}
