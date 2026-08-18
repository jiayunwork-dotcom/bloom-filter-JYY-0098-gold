package counting

import (
	"testing"
)

func TestCountingAddAndTest(t *testing.T) {
	cf, err := New(1024, 5, 4)
	if err != nil {
		t.Fatal(err)
	}
	items := []string{"foo", "bar", "baz"}
	for _, item := range items {
		if err := cf.Add([]byte(item)); err != nil {
			t.Fatalf("Add(%q): %v", item, err)
		}
	}
	for _, item := range items {
		if !cf.Test([]byte(item)) {
			t.Errorf("Test(%q) = false after Add", item)
		}
	}
	if cf.Count() != 3 {
		t.Errorf("Count = %d, want 3", cf.Count())
	}
}

func TestCountingRemove(t *testing.T) {
	cf, _ := New(2048, 5, 4)
	_ = cf.Add([]byte("hello"))
	_ = cf.Add([]byte("world"))

	if err := cf.Remove([]byte("hello")); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if cf.Test([]byte("hello")) {
		t.Error("Test('hello') = true after Remove, want false")
	}
	if !cf.Test([]byte("world")) {
		t.Error("Test('world') = false, should still be present")
	}
}

func TestCountingUnderflow(t *testing.T) {
	cf, _ := New(512, 3, 4)
	err := cf.Remove([]byte("never_added"))
	if err != ErrUnderflow {
		t.Errorf("Remove never-added item: err = %v, want ErrUnderflow", err)
	}
}

func TestCountingOverflow(t *testing.T) {
	// 1-bit counter: max count = 1
	cf, _ := New(512, 3, 1)
	_ = cf.Add([]byte("item"))
	err := cf.Add([]byte("item"))
	if err != ErrOverflow {
		t.Errorf("Double Add with 1-bit counter: err = %v, want ErrOverflow", err)
	}
}

func TestCountingSnapshot(t *testing.T) {
	cf, _ := New(1024, 5, 4)
	_ = cf.Add([]byte("snap1"))
	_ = cf.Add([]byte("snap2"))

	data, err := MarshalSnapshot(cf)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := UnmarshalSnapshot(data)
	if err != nil {
		t.Fatal(err)
	}
	if !restored.Test([]byte("snap1")) {
		t.Error("restored filter missing 'snap1'")
	}
	if !restored.Test([]byte("snap2")) {
		t.Error("restored filter missing 'snap2'")
	}
	if restored.M() != cf.M() || restored.K() != cf.K() {
		t.Error("restored parameters don't match")
	}
}

func TestCountingSaturation(t *testing.T) {
	cf, _ := New(64, 3, 4)
	sat := cf.Saturation()
	if sat != 0 {
		t.Errorf("empty saturation = %f, want 0", sat)
	}
	for i := 0; i < 20; i++ {
		_ = cf.Add([]byte{byte(i)})
	}
	sat = cf.Saturation()
	if sat <= 0 || sat > 1 {
		t.Errorf("saturation after adds = %f, out of range", sat)
	}
}

func TestCountingClone(t *testing.T) {
	cf, _ := New(512, 5, 4)
	_ = cf.Add([]byte("clone-me"))
	clone := cf.Clone()
	if !clone.Test([]byte("clone-me")) {
		t.Error("clone missing 'clone-me'")
	}
	// Modifying original shouldn't affect clone
	_ = cf.Remove([]byte("clone-me"))
	if !clone.Test([]byte("clone-me")) {
		t.Error("clone affected by original Remove")
	}
}

func TestCountingMerge(t *testing.T) {
	a, _ := New(512, 5, 4)
	b, _ := New(512, 5, 4)
	_ = a.Add([]byte("a-item"))
	_ = b.Add([]byte("b-item"))

	err := a.Merge(b)
	if err != nil {
		t.Fatal(err)
	}
	if !a.Test([]byte("b-item")) {
		t.Error("merged filter missing 'b-item'")
	}
}
