package filter

import "errors"

// Builder errors.
var (
	ErrBuilderNoItems = errors.New("filter: builder has no target capacity set")
)

// Builder provides a fluent API for constructing BloomFilters with validated
// parameters. It computes optimal m and k from the desired capacity and FPR.
type Builder struct {
	expectedN uint
	maxFPR    float64
	m         uint
	k         uint
	useManual bool
	preload   [][]byte
}

// NewBuilder creates a new filter builder.
func NewBuilder() *Builder {
	return &Builder{
		maxFPR: 0.01, // default 1% FPR
	}
}

// WithCapacity sets the expected number of items and target false positive rate.
// Automatically computes optimal m and k.
func (b *Builder) WithCapacity(expectedN uint, maxFPR float64) *Builder {
	b.expectedN = expectedN
	b.maxFPR = maxFPR
	b.useManual = false
	return b
}

// WithParams sets explicit m and k, overriding automatic computation.
func (b *Builder) WithParams(m, k uint) *Builder {
	b.m = m
	b.k = k
	b.useManual = true
	return b
}

// WithPreload specifies items to add immediately after construction.
func (b *Builder) WithPreload(items [][]byte) *Builder {
	b.preload = items
	return b
}

// Build constructs the BloomFilter. Returns an error if parameters are invalid.
func (b *Builder) Build() (*BloomFilter, error) {
	var f *BloomFilter
	var err error

	if b.useManual {
		f, err = New(b.m, b.k)
	} else {
		if b.expectedN == 0 {
			return nil, ErrBuilderNoItems
		}
		if b.maxFPR <= 0 || b.maxFPR >= 1 {
			return nil, ErrInvalidParams
		}
		f, err = NewFromCapacity(b.expectedN, b.maxFPR)
	}
	if err != nil {
		return nil, err
	}

	for _, item := range b.preload {
		f.Add(item)
	}
	return f, nil
}

// EstimateParams returns the m and k that would be used without building.
func (b *Builder) EstimateParams() (m, k uint) {
	if b.useManual {
		return b.m, b.k
	}
	if b.expectedN == 0 || b.maxFPR <= 0 || b.maxFPR >= 1 {
		return 0, 0
	}
	m = RequiredBits(b.expectedN, b.maxFPR)
	k = OptimalK(m, b.expectedN)
	return m, k
}

// ScalableBloom implements a scalable Bloom filter that grows by adding new
// sub-filters when the current one approaches saturation. Each new sub-filter
// is sized larger (by growthFactor) with a tighter FPR (by fprScale).
type ScalableBloom struct {
	filters      []*BloomFilter
	targetFPR    float64
	fprScale     float64 // each new filter's FPR = prev * fprScale (< 1)
	growthFactor uint    // each new filter's capacity = prev * growthFactor
	initCap      uint
	currentN     int
	maxN         int // max items in current active filter
}

// NewScalableBloom creates a scalable Bloom filter.
// initCap: initial capacity of the first filter.
// targetFPR: overall target false positive rate.
// fprScale: how much tighter each new filter's FPR is (e.g. 0.8).
// growthFactor: capacity multiplier for each new filter (e.g. 2).
func NewScalableBloom(initCap uint, targetFPR, fprScale float64, growthFactor uint) (*ScalableBloom, error) {
	if initCap == 0 || targetFPR <= 0 || targetFPR >= 1 {
		return nil, ErrInvalidParams
	}
	if fprScale <= 0 || fprScale >= 1 {
		return nil, errors.New("filter: fprScale must be in (0, 1)")
	}
	if growthFactor < 1 {
		growthFactor = 2
	}

	first, err := NewFromCapacity(initCap, targetFPR)
	if err != nil {
		return nil, err
	}
	sb := &ScalableBloom{
		filters:      []*BloomFilter{first},
		targetFPR:    targetFPR,
		fprScale:     fprScale,
		growthFactor: growthFactor,
		initCap:      initCap,
		currentN:     0,
		maxN:         first.MaxInsertions(targetFPR),
	}
	return sb, nil
}

// Add inserts an item into the scalable filter. If the active filter is
// at capacity, a new larger filter is created.
func (sb *ScalableBloom) Add(data []byte) {
	if sb.currentN >= sb.maxN {
		sb.grow()
	}
	active := sb.filters[len(sb.filters)-1]
	active.Add(data)
	sb.currentN++
}

// Test checks all sub-filters for the item.
func (sb *ScalableBloom) Test(data []byte) bool {
	for _, f := range sb.filters {
		if f.Test(data) {
			return true
		}
	}
	return false
}

// Len returns the total number of items added.
func (sb *ScalableBloom) Len() int {
	total := 0
	// All previous filters are full
	for i := 0; i < len(sb.filters)-1; i++ {
		total += sb.filters[i].MaxInsertions(sb.targetFPR)
	}
	total += sb.currentN
	return total
}

// NumFilters returns the number of sub-filters.
func (sb *ScalableBloom) NumFilters() int {
	return len(sb.filters)
}

// Filters returns all sub-filters (read-only view).
func (sb *ScalableBloom) Filters() []*BloomFilter {
	out := make([]*BloomFilter, len(sb.filters))
	copy(out, sb.filters)
	return out
}

func (sb *ScalableBloom) grow() {
	// New filter with scaled capacity and tighter FPR
	cap := sb.initCap
	fpr := sb.targetFPR
	for i := 0; i < len(sb.filters); i++ {
		cap *= sb.growthFactor
		fpr *= sb.fprScale
	}
	if fpr < 1e-10 {
		fpr = 1e-10
	}
	nf, err := NewFromCapacity(cap, fpr)
	if err != nil {
		// Fallback: duplicate current size
		nf, _ = NewFromCapacity(sb.initCap, sb.targetFPR)
	}
	sb.filters = append(sb.filters, nf)
	sb.currentN = 0
	sb.maxN = nf.MaxInsertions(fpr)
	if sb.maxN <= 0 {
		sb.maxN = int(cap)
	}
}
