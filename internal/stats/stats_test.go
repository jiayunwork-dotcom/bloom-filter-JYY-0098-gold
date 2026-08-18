package stats

import (
	"testing"
	"time"
)

func TestCollectorRecordAdd(t *testing.T) {
	c := NewCollector(100)
	c.RecordAdd()
	c.RecordAdd()
	c.RecordAdd()
	m := c.Metrics()
	if m.Adds != 3 {
		t.Errorf("Adds = %d, want 3", m.Adds)
	}
}

func TestCollectorRecordTest(t *testing.T) {
	c := NewCollector(100)
	c.RecordTest(true)
	c.RecordTest(true)
	c.RecordTest(false)
	m := c.Metrics()
	if m.Tests != 3 {
		t.Errorf("Tests = %d, want 3", m.Tests)
	}
	if m.Hits != 2 {
		t.Errorf("Hits = %d, want 2", m.Hits)
	}
	if m.Misses != 1 {
		t.Errorf("Misses = %d, want 1", m.Misses)
	}
	hr := m.HitRate()
	if hr < 0.66 || hr > 0.67 {
		t.Errorf("HitRate = %f, want ~0.667", hr)
	}
}

func TestCollectorSnapshot(t *testing.T) {
	c := NewCollector(10)
	c.RecordAdd()
	snap := c.TakeSnapshot(0.5)
	if snap.Saturation != 0.5 {
		t.Errorf("snapshot Saturation = %f, want 0.5", snap.Saturation)
	}
	if snap.Metrics.Adds != 1 {
		t.Errorf("snapshot Adds = %d, want 1", snap.Metrics.Adds)
	}
	history := c.History()
	if len(history) != 1 {
		t.Errorf("History len = %d, want 1", len(history))
	}
}

func TestCollectorReset(t *testing.T) {
	c := NewCollector(100)
	c.RecordAdd()
	c.RecordTest(true)
	c.TakeSnapshot(0.1)
	c.Reset()
	m := c.Metrics()
	if m.Adds != 0 || m.Tests != 0 {
		t.Error("counters not zero after Reset")
	}
	if len(c.History()) != 0 {
		t.Error("history not cleared after Reset")
	}
}

func TestRateTrackerBasic(t *testing.T) {
	rt := NewRateTracker(10*time.Second, time.Second)
	rt.Record(5)
	rt.Record(3)
	total := rt.Total()
	if total != 8 {
		t.Errorf("Total = %d, want 8", total)
	}
	rate := rt.Rate()
	if rate <= 0 {
		t.Errorf("Rate = %f, want > 0", rate)
	}
}

func TestRateTrackerReset(t *testing.T) {
	rt := NewRateTracker(5*time.Second, time.Second)
	rt.Record(10)
	rt.Reset()
	if rt.Total() != 0 {
		t.Errorf("Total after Reset = %d, want 0", rt.Total())
	}
}

func TestRateTrackerSummary(t *testing.T) {
	rt := NewRateTracker(30*time.Second, time.Second)
	rt.Record(100)
	s := rt.Summary()
	if s.NumBuckets != 30 {
		t.Errorf("NumBuckets = %d, want 30", s.NumBuckets)
	}
	if s.ActiveTotal != 100 {
		t.Errorf("ActiveTotal = %d, want 100", s.ActiveTotal)
	}
}
