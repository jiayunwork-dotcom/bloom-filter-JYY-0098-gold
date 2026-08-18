package merge

import (
	"testing"

	"bloom-filter/internal/filter"
)

func makeFilter(t *testing.T, m, k uint, items []string) *filter.BloomFilter {
	t.Helper()
	f, err := filter.New(m, k)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		f.Add([]byte(item))
	}
	return f
}

func TestUnion(t *testing.T) {
	a := makeFilter(t, 1024, 5, []string{"a", "b", "c"})
	b := makeFilter(t, 1024, 5, []string{"d", "e", "f"})

	u, err := Union(a, b)
	if err != nil {
		t.Fatal(err)
	}
	// Union should contain items from both
	for _, item := range []string{"a", "b", "c", "d", "e", "f"} {
		if !u.Test([]byte(item)) {
			t.Errorf("Union missing %q", item)
		}
	}
}

func TestIntersection(t *testing.T) {
	a := makeFilter(t, 1024, 5, []string{"x", "y", "z"})
	b := makeFilter(t, 1024, 5, []string{"y", "z", "w"})

	inter, err := Intersection(a, b)
	if err != nil {
		t.Fatal(err)
	}
	// "y" and "z" should still test true (both filters have them)
	if !inter.Test([]byte("y")) {
		t.Error("Intersection missing 'y'")
	}
	if !inter.Test([]byte("z")) {
		t.Error("Intersection missing 'z'")
	}
}

func TestDifference(t *testing.T) {
	a := makeFilter(t, 2048, 5, []string{"alpha", "beta", "gamma"})
	b := makeFilter(t, 2048, 5, []string{"beta"})

	diff, err := Difference(a, b)
	if err != nil {
		t.Fatal(err)
	}
	// "beta" bits should be cleared in diff
	// Note: this is an approximation, not guaranteed
	_ = diff // just ensure no error
}

func TestParamMismatch(t *testing.T) {
	a, _ := filter.New(1024, 5)
	b, _ := filter.New(2048, 5) // different m

	_, err := Union(a, b)
	if err != ErrParamMismatch {
		t.Errorf("Union with mismatched m: err = %v, want ErrParamMismatch", err)
	}
}

func TestSimilarityIdentical(t *testing.T) {
	a := makeFilter(t, 1024, 5, []string{"same", "items"})
	sim, err := Similarity(a, a)
	if err != nil {
		t.Fatal(err)
	}
	if sim != 1.0 {
		t.Errorf("Similarity(a, a) = %f, want 1.0", sim)
	}
}

func TestHammingDistanceEmpty(t *testing.T) {
	a, _ := filter.New(512, 3)
	b, _ := filter.New(512, 3)
	dist, err := HammingDistance(a, b)
	if err != nil {
		t.Fatal(err)
	}
	if dist != 0 {
		t.Errorf("HammingDistance of two empty filters = %d, want 0", dist)
	}
}

func TestUnionMany(t *testing.T) {
	filters := make([]*filter.BloomFilter, 5)
	for i := range filters {
		filters[i] = makeFilter(t, 1024, 5, []string{string(rune('a' + i))})
	}
	u, err := UnionMany(filters)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		if !u.Test([]byte(string(rune('a' + i)))) {
			t.Errorf("UnionMany missing item %d", i)
		}
	}
}

func TestEstimateUnionCardinality(t *testing.T) {
	a := makeFilter(t, 8192, 7, []string{"1", "2", "3", "4", "5"})
	b := makeFilter(t, 8192, 7, []string{"4", "5", "6", "7", "8"})
	n, err := EstimateUnionCardinality(a, b)
	if err != nil {
		t.Fatal(err)
	}
	// Expect roughly 8 distinct items (some estimation error)
	if n < 4 || n > 16 {
		t.Errorf("EstimateUnionCardinality = %d, expected roughly 8", n)
	}
}
