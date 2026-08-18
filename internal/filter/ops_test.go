package filter

import (
	"testing"
)

func TestFilterOpsReset(t *testing.T) {
	f, _ := New(512, 5)
	f.Add([]byte("item"))
	f.Reset()
	if !f.IsEmpty() {
		t.Error("filter not empty after Reset")
	}
	if f.Test([]byte("item")) {
		t.Error("Test returns true after Reset")
	}
}

func TestFilterOpsClone(t *testing.T) {
	f, _ := New(1024, 5)
	f.Add([]byte("clone"))
	c := f.Clone()
	if !c.Test([]byte("clone")) {
		t.Error("Clone doesn't contain item")
	}
	// Modify original
	f.Reset()
	if !c.Test([]byte("clone")) {
		t.Error("Clone affected by Reset on original")
	}
}

func TestFilterOpsUnion(t *testing.T) {
	a, _ := New(1024, 5)
	b, _ := New(1024, 5)
	a.Add([]byte("A"))
	b.Add([]byte("B"))

	err := a.Union(b)
	if err != nil {
		t.Fatal(err)
	}
	if !a.Test([]byte("A")) || !a.Test([]byte("B")) {
		t.Error("Union missing items")
	}
}

func TestFilterOpsIntersection(t *testing.T) {
	a, _ := New(2048, 5)
	b, _ := New(2048, 5)
	a.Add([]byte("shared"))
	a.Add([]byte("only-a"))
	b.Add([]byte("shared"))
	b.Add([]byte("only-b"))

	err := a.Intersection(b)
	if err != nil {
		t.Fatal(err)
	}
	if !a.Test([]byte("shared")) {
		t.Error("Intersection missing 'shared'")
	}
}

func TestFilterOpsEquals(t *testing.T) {
	a, _ := New(512, 3)
	b := a.Clone()
	if !a.Equals(b) {
		t.Error("Clone should Equal original")
	}
	b.Add([]byte("extra"))
	if a.Equals(b) {
		t.Error("Modified clone should not Equal original")
	}
}

func TestFilterOpsPopCount(t *testing.T) {
	f, _ := New(128, 3)
	if f.PopCount() != 0 {
		t.Error("empty filter PopCount != 0")
	}
	f.Add([]byte("x"))
	pc := f.PopCount()
	// Should have at least k bits set (3)
	if pc < 3 {
		t.Errorf("PopCount after one Add = %d, want >= 3", pc)
	}
}

func TestFilterOpsSaturation(t *testing.T) {
	f, _ := New(256, 5)
	if f.Saturation() != 0 {
		t.Error("empty Saturation != 0")
	}
	for i := 0; i < 50; i++ {
		f.Add([]byte{byte(i)})
	}
	sat := f.Saturation()
	if sat <= 0 || sat > 1 {
		t.Errorf("Saturation = %f, out of range", sat)
	}
}

func TestFilterOpsSetGetClearBit(t *testing.T) {
	f, _ := New(256, 3)
	f.SetBit(42)
	if !f.GetBit(42) {
		t.Error("GetBit(42) = false after SetBit")
	}
	f.ClearBit(42)
	if f.GetBit(42) {
		t.Error("GetBit(42) = true after ClearBit")
	}
}

func TestFilterOpsXOR(t *testing.T) {
	a, _ := New(512, 3)
	b, _ := New(512, 3)
	a.Add([]byte("one"))
	b.Add([]byte("two"))
	err := a.XOR(b)
	if err != nil {
		t.Fatal(err)
	}
	// XOR result is defined; just verify no error
}

func TestScalableBloomBasic(t *testing.T) {
	sb, err := NewScalableBloom(100, 0.01, 0.8, 2)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 250; i++ {
		sb.Add([]byte{byte(i), byte(i >> 8)})
	}
	// Should have grown
	if sb.NumFilters() < 2 {
		t.Errorf("NumFilters = %d after 250 items with initCap 100, expected growth", sb.NumFilters())
	}
	// Items should test positive
	for i := 0; i < 250; i++ {
		if !sb.Test([]byte{byte(i), byte(i >> 8)}) {
			t.Errorf("ScalableBloom missing item %d", i)
			break
		}
	}
}

func TestBuilderWithCapacity(t *testing.T) {
	f, err := NewBuilder().WithCapacity(1000, 0.01).Build()
	if err != nil {
		t.Fatal(err)
	}
	if f.M() == 0 || f.K() == 0 {
		t.Error("Builder produced filter with zero params")
	}
}

func TestBuilderWithPreload(t *testing.T) {
	items := [][]byte{[]byte("a"), []byte("b"), []byte("c")}
	f, err := NewBuilder().WithCapacity(100, 0.01).WithPreload(items).Build()
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		if !f.Test(item) {
			t.Errorf("preloaded item %q not found", item)
		}
	}
}
