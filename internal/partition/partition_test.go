package partition

import (
	"testing"
)

func TestNewPartitionFilter(t *testing.T) {
	pf, err := New(4, 1024, 5)
	if err != nil {
		t.Fatalf("New(4, 1024, 5) error: %v", err)
	}
	if pf.PartitionCount() != 4 {
		t.Errorf("PartitionCount = %d, want 4", pf.PartitionCount())
	}
	if pf.BitsPerPartition() != 1024 {
		t.Errorf("BitsPerPartition = %d, want 1024", pf.BitsPerPartition())
	}
	if pf.TotalBits() != 4096 {
		t.Errorf("TotalBits = %d, want 4096", pf.TotalBits())
	}
}

func TestPartitionAddAndTest(t *testing.T) {
	pf, err := New(4, 8192, 7)
	if err != nil {
		t.Fatal(err)
	}
	items := []string{"apple", "banana", "cherry", "date", "elderberry"}
	for _, item := range items {
		pf.Add([]byte(item))
	}
	// Routed test: each item should be in its routed partition
	for _, item := range items {
		if !pf.Test([]byte(item)) {
			t.Errorf("Test(%q) = false after Add, want true", item)
		}
	}
	// TestAll should also find them
	for _, item := range items {
		if !pf.TestAll([]byte(item)) {
			t.Errorf("TestAll(%q) = false after Add, want true", item)
		}
	}
	// Absent item
	if pf.Test([]byte("zzz_never_added")) {
		// Could be false positive but very unlikely with 8192*4 bits
		t.Log("unexpected positive for absent item (possible false positive)")
	}
}

func TestPartitionReset(t *testing.T) {
	pf, _ := New(3, 4096, 5)
	pf.Add([]byte("item1"))
	pf.Add([]byte("item2"))
	pf.Reset()
	if pf.TestAll([]byte("item1")) {
		t.Error("TestAll after Reset should be false")
	}
}

func TestPartitionResetSingle(t *testing.T) {
	pf, _ := New(4, 4096, 5)
	pf.Add([]byte("hello"))
	// Reset partition 0 only
	err := pf.ResetPartition(0)
	if err != nil {
		t.Fatal(err)
	}
	// Invalid index
	err = pf.ResetPartition(10)
	if err != ErrPartitionIndex {
		t.Errorf("ResetPartition(10) error = %v, want ErrPartitionIndex", err)
	}
}

func TestPartitionSaturation(t *testing.T) {
	pf, _ := New(2, 64, 3)
	// Empty filter has zero saturation
	sat, err := pf.Saturation(0)
	if err != nil {
		t.Fatal(err)
	}
	if sat != 0 {
		t.Errorf("empty saturation = %f, want 0", sat)
	}
	// Add items to drive up saturation
	for i := 0; i < 20; i++ {
		pf.Add([]byte{byte(i)})
	}
	sat, _ = pf.Saturation(0)
	if sat < 0 || sat > 1 {
		t.Errorf("saturation out of range: %f", sat)
	}
}

func TestPartitionInvalidParams(t *testing.T) {
	_, err := New(0, 1024, 5)
	if err != ErrInvalidPartitions {
		t.Errorf("New(0,...) error = %v, want ErrInvalidPartitions", err)
	}
	_, err = New(4, 0, 5)
	if err != ErrInvalidParams {
		t.Errorf("New(4,0,5) error = %v, want ErrInvalidParams", err)
	}
}

func TestRotatingFilterBasic(t *testing.T) {
	cfg := DefaultRotationConfig()
	rf, err := NewRotatingFilter(3, 4096, 5, cfg)
	if err != nil {
		t.Fatal(err)
	}
	rf.Add([]byte("test-item"))
	if !rf.Test([]byte("test-item")) {
		t.Error("Test after Add should be true")
	}
	// Manual rotate
	if err := rf.Rotate(); err != nil {
		t.Fatal(err)
	}
	// Item might still be in non-rotated partition
	history := rf.History()
	if len(history) != 1 {
		t.Errorf("History len = %d, want 1", len(history))
	}
}
