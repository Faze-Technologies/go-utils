package serialtier

import (
	"reflect"
	"testing"
)

func TestIsValid(t *testing.T) {
	for _, tt := range []struct {
		in   string
		want bool
	}{
		{"ULTRA_PREMIUM", true},
		{"2_TO_10", true},
		{"10_TO_15", true},
		{"15_TO_40", true},
		{"40_TO_75", true},
		{"75_TO_100", true},
		{"", false},
		{"SPECIFIC", false},
		{"ultra_premium", false},
		{"GARBAGE", false},
	} {
		if got := IsValid(tt.in); got != tt.want {
			t.Errorf("IsValid(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestEventGroup_Validate(t *testing.T) {
	for _, tt := range []struct {
		eg      EventGroup
		wantErr bool
	}{
		{EventGroup{Total: 1}, false},
		{EventGroup{Total: 100, JerseyNumber: 50}, false},
		{EventGroup{Total: 100, JerseyNumber: 0}, false},
		{EventGroup{Total: 100, JerseyNumber: 999}, false}, // out-of-range jersey is normalized later, not rejected
		{EventGroup{Total: 0}, true},
		{EventGroup{Total: -1}, true},
	} {
		err := tt.eg.Validate()
		gotErr := err != nil
		if gotErr != tt.wantErr {
			t.Errorf("EventGroup%+v.Validate() err=%v, wantErr=%v", tt.eg, err, tt.wantErr)
		}
	}
}

func TestRanges_NonPositiveTotal(t *testing.T) {
	for _, total := range []int{0, -1, -100} {
		eg := EventGroup{Total: total}
		if _, err := eg.Ranges(TierUltraPremium); err == nil {
			t.Errorf("total=%d: want error, got nil", total)
		}
	}
}

func TestRanges_UnknownTier(t *testing.T) {
	eg := EventGroup{Total: 100}
	if _, err := eg.Ranges(Tier("BOGUS")); err == nil {
		t.Error("unknown tier: want error, got nil")
	}
}

// TestUltraPremium covers the deduped {1, jersey, total} set across edge cases.
func TestUltraPremium(t *testing.T) {
	for _, tt := range []struct {
		name string
		eg   EventGroup
		want []Range
	}{
		{
			name: "total=1 dedupes to single #1", eg: EventGroup{Total: 1},
			want: []Range{{Tier: TierUltraPremium, Min: 1, Max: 1}},
		},
		{
			name: "total=2 no jersey", eg: EventGroup{Total: 2},
			want: []Range{
				{Tier: TierUltraPremium, Min: 1, Max: 1},
				{Tier: TierUltraPremium, Min: 2, Max: 2},
			},
		},
		{
			name: "no jersey at total=100", eg: EventGroup{Total: 100},
			want: []Range{
				{Tier: TierUltraPremium, Min: 1, Max: 1},
				{Tier: TierUltraPremium, Min: 100, Max: 100},
			},
		},
		{
			name: "jersey=50 at total=100 emits 3 ranges", eg: EventGroup{Total: 100, JerseyNumber: 50},
			want: []Range{
				{Tier: TierUltraPremium, Min: 1, Max: 1},
				{Tier: TierUltraPremium, Min: 50, Max: 50},
				{Tier: TierUltraPremium, Min: 100, Max: 100},
			},
		},
		{
			name: "jersey=1 dedupes with #1", eg: EventGroup{Total: 100, JerseyNumber: 1},
			want: []Range{
				{Tier: TierUltraPremium, Min: 1, Max: 1},
				{Tier: TierUltraPremium, Min: 100, Max: 100},
			},
		},
		{
			name: "jersey=total dedupes with #total", eg: EventGroup{Total: 100, JerseyNumber: 100},
			want: []Range{
				{Tier: TierUltraPremium, Min: 1, Max: 1},
				{Tier: TierUltraPremium, Min: 100, Max: 100},
			},
		},
		{
			name: "jersey<1 ignored", eg: EventGroup{Total: 100, JerseyNumber: -5},
			want: []Range{
				{Tier: TierUltraPremium, Min: 1, Max: 1},
				{Tier: TierUltraPremium, Min: 100, Max: 100},
			},
		},
		{
			name: "jersey>total ignored", eg: EventGroup{Total: 50, JerseyNumber: 99},
			want: []Range{
				{Tier: TierUltraPremium, Min: 1, Max: 1},
				{Tier: TierUltraPremium, Min: 50, Max: 50},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.eg.Ranges(TierUltraPremium)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

// TestPercentileBandSplitsAroundJersey: a jersey that falls inside a
// percentile band's natural interval splits it into two ranges.
func TestPercentileBandSplitsAroundJersey(t *testing.T) {
	eg := EventGroup{Total: 100, JerseyNumber: 50}
	got, err := eg.Ranges(Tier40To75)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []Range{
		{Tier: Tier40To75, Min: 41, Max: 49},
		{Tier: Tier40To75, Min: 51, Max: 75},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestPercentileBandJerseyAtBoundary(t *testing.T) {
	// Tier40To75 at total=100 is [41, 75]. jersey=41 → [42, 75]; jersey=75 → [41, 74].
	got, err := EventGroup{Total: 100, JerseyNumber: 41}.Ranges(Tier40To75)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(got, []Range{{Tier: Tier40To75, Min: 42, Max: 75}}) {
		t.Errorf("jersey=lower-edge: got %+v", got)
	}

	got, err = EventGroup{Total: 100, JerseyNumber: 75}.Ranges(Tier40To75)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(got, []Range{{Tier: Tier40To75, Min: 41, Max: 74}}) {
		t.Errorf("jersey=upper-edge: got %+v", got)
	}
}

func TestPercentileBandJerseyOutside(t *testing.T) {
	// Tier40To75 at total=100 is [41, 75]. jersey=20 (in Tier15To40) leaves it alone.
	got, err := EventGroup{Total: 100, JerseyNumber: 20}.Ranges(Tier40To75)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(got, []Range{{Tier: Tier40To75, Min: 41, Max: 75}}) {
		t.Errorf("got %+v", got)
	}
}

// TestTopSerialExclusive: serial #total never appears in any non-ULTRA tier.
func TestTopSerialExclusive(t *testing.T) {
	totals := []int{2, 5, 9, 10, 11, 100, 1000}
	jerseys := []int{0, 1, 5, 50, 99, 100}
	for _, total := range totals {
		for _, j := range jerseys {
			eg := EventGroup{Total: total, JerseyNumber: j}
			for _, tier := range []Tier{Tier2To10, Tier10To15, Tier15To40, Tier40To75, Tier75To100} {
				ranges, err := eg.Ranges(tier)
				if err != nil {
					t.Fatalf("eg=%+v tier=%s: %v", eg, tier, err)
				}
				for _, r := range ranges {
					if r.Max >= total {
						t.Errorf("eg=%+v tier=%s: range %+v claims #total", eg, tier, r)
					}
				}
			}
		}
	}
}

// TestJerseyExclusive: serial #jersey never appears in any non-ULTRA tier
// (when jersey is in valid range).
func TestJerseyExclusive(t *testing.T) {
	for _, total := range []int{10, 11, 100, 1000} {
		for j := 1; j <= total; j++ {
			eg := EventGroup{Total: total, JerseyNumber: j}
			for _, tier := range []Tier{Tier2To10, Tier10To15, Tier15To40, Tier40To75, Tier75To100} {
				ranges, err := eg.Ranges(tier)
				if err != nil {
					t.Fatalf("eg=%+v tier=%s: %v", eg, tier, err)
				}
				for _, r := range ranges {
					if j >= r.Min && j <= r.Max {
						t.Errorf("eg=%+v tier=%s: range %+v claims #jersey", eg, tier, r)
					}
				}
			}
		}
	}
}

func TestRanges_2To10(t *testing.T) {
	for _, tt := range []struct {
		name string
		eg   EventGroup
		want []Range
	}{
		{"total=1 empty", EventGroup{Total: 1}, nil},
		{"total=2 empty", EventGroup{Total: 2}, nil},
		{"total=3 single serial #2", EventGroup{Total: 3}, []Range{{Tier: Tier2To10, Min: 2, Max: 2}}},
		{"total=10", EventGroup{Total: 10}, []Range{{Tier: Tier2To10, Min: 2, Max: 9}}},
		{"total=100 no jersey", EventGroup{Total: 100}, []Range{{Tier: Tier2To10, Min: 2, Max: 10}}},
		{"total=100 jersey=5 splits", EventGroup{Total: 100, JerseyNumber: 5}, []Range{
			{Tier: Tier2To10, Min: 2, Max: 4},
			{Tier: Tier2To10, Min: 6, Max: 10},
		}},
		{"total=100 jersey=2 trims lower", EventGroup{Total: 100, JerseyNumber: 2}, []Range{{Tier: Tier2To10, Min: 3, Max: 10}}},
		{"total=100 jersey=10 trims upper", EventGroup{Total: 100, JerseyNumber: 10}, []Range{{Tier: Tier2To10, Min: 2, Max: 9}}},
		{"total=100 jersey=50 outside", EventGroup{Total: 100, JerseyNumber: 50}, []Range{{Tier: Tier2To10, Min: 2, Max: 10}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.eg.Ranges(Tier2To10)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestRanges_75To100(t *testing.T) {
	for _, tt := range []struct {
		name string
		eg   EventGroup
		want []Range
	}{
		{"total=100", EventGroup{Total: 100}, []Range{{Tier: Tier75To100, Min: 76, Max: 99}}},
		{"total=1000", EventGroup{Total: 1000}, []Range{{Tier: Tier75To100, Min: 751, Max: 999}}},
		{"jersey=85 splits", EventGroup{Total: 100, JerseyNumber: 85}, []Range{
			{Tier: Tier75To100, Min: 76, Max: 84},
			{Tier: Tier75To100, Min: 86, Max: 99},
		}},
		{"jersey=76 trims lower", EventGroup{Total: 100, JerseyNumber: 76}, []Range{{Tier: Tier75To100, Min: 77, Max: 99}}},
		{"jersey=99 trims upper", EventGroup{Total: 100, JerseyNumber: 99}, []Range{{Tier: Tier75To100, Min: 76, Max: 98}}},
		{"jersey=100 dedupes in ULTRA, band unchanged", EventGroup{Total: 100, JerseyNumber: 100}, []Range{{Tier: Tier75To100, Min: 76, Max: 99}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.eg.Ranges(Tier75To100)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

// TestFullPartition: every serial 1..total is owned by exactly one tier across
// representative (total, jersey) pairs. Catches off-by-one errors anywhere
// in the band-boundary math.
func TestFullPartition(t *testing.T) {
	cases := []EventGroup{
		{Total: 100},
		{Total: 100, JerseyNumber: 50},
		{Total: 100, JerseyNumber: 1},
		{Total: 100, JerseyNumber: 100},
		{Total: 100, JerseyNumber: 5},  // inside Tier2To10
		{Total: 100, JerseyNumber: 11}, // inside Tier10To15
		{Total: 100, JerseyNumber: 99}, // inside Tier75To100
		{Total: 1000},
		{Total: 1000, JerseyNumber: 500},
		{Total: 1000, JerseyNumber: 250},
		{Total: 11, JerseyNumber: 5},
		{Total: 10, JerseyNumber: 5},
	}
	for _, eg := range cases {
		t.Run("", func(t *testing.T) {
			owners := map[int]Tier{}
			for _, tier := range AllTiers() {
				ranges, err := eg.Ranges(tier)
				if err != nil {
					t.Fatalf("eg=%+v tier=%s: %v", eg, tier, err)
				}
				for _, r := range ranges {
					for s := r.Min; s <= r.Max; s++ {
						if prev, ok := owners[s]; ok {
							t.Errorf("eg=%+v serial=%d claimed by both %s and %s", eg, s, prev, tier)
						}
						owners[s] = tier
					}
				}
			}
			for s := 1; s <= eg.Total; s++ {
				if _, ok := owners[s]; !ok {
					t.Errorf("eg=%+v serial=%d not claimed by any tier", eg, s)
				}
			}
			if owners[1] != TierUltraPremium {
				t.Errorf("eg=%+v: serial #1 owned by %s, want ULTRA_PREMIUM", eg, owners[1])
			}
			if owners[eg.Total] != TierUltraPremium {
				t.Errorf("eg=%+v: serial #%d owned by %s, want ULTRA_PREMIUM", eg, eg.Total, owners[eg.Total])
			}
			if eg.JerseyNumber >= 1 && eg.JerseyNumber <= eg.Total && owners[eg.JerseyNumber] != TierUltraPremium {
				t.Errorf("eg=%+v: serial #%d owned by %s, want ULTRA_PREMIUM", eg, eg.JerseyNumber, owners[eg.JerseyNumber])
			}
		})
	}
}

func TestTierFor(t *testing.T) {
	for _, tt := range []struct {
		name     string
		eg       EventGroup
		serialNo int
		wantTier Tier
		wantOK   bool
	}{
		{"#1 always ULTRA", EventGroup{Total: 100}, 1, TierUltraPremium, true},
		{"#total always ULTRA", EventGroup{Total: 100}, 100, TierUltraPremium, true},
		{"#jersey ULTRA", EventGroup{Total: 100, JerseyNumber: 50}, 50, TierUltraPremium, true},
		{"#99 last non-ULTRA", EventGroup{Total: 100}, 99, Tier75To100, true},
		{"#2", EventGroup{Total: 100}, 2, Tier2To10, true},
		{"#10", EventGroup{Total: 100}, 10, Tier2To10, true},
		{"#11", EventGroup{Total: 100}, 11, Tier10To15, true},
		{"#50 percentile no jersey", EventGroup{Total: 100}, 50, Tier40To75, true},
		{"#51 percentile when jersey=50", EventGroup{Total: 100, JerseyNumber: 50}, 51, Tier40To75, true},
		{"#0 invalid", EventGroup{Total: 100}, 0, "", false},
		{"#101 invalid", EventGroup{Total: 100}, 101, "", false},
		{"total=0 invalid", EventGroup{Total: 0}, 1, "", false},
		{"total=1 single serial ULTRA", EventGroup{Total: 1}, 1, TierUltraPremium, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := tt.eg.TierFor(tt.serialNo)
			if ok != tt.wantOK {
				t.Fatalf("ok=%v, want %v", ok, tt.wantOK)
			}
			if ok && got != tt.wantTier {
				t.Errorf("got %s, want %s", got, tt.wantTier)
			}
		})
	}
}

// TestTierFor_RoundTripFullPartition: for every (eg, serial) pair, TierFor
// agrees with Ranges — the tier TierFor returns must contain serial in one of
// its ranges.
func TestTierFor_RoundTripFullPartition(t *testing.T) {
	cases := []EventGroup{
		{Total: 2}, {Total: 5}, {Total: 5, JerseyNumber: 3},
		{Total: 10}, {Total: 10, JerseyNumber: 5},
		{Total: 11}, {Total: 11, JerseyNumber: 7},
		{Total: 100}, {Total: 100, JerseyNumber: 50},
		{Total: 100, JerseyNumber: 1}, {Total: 100, JerseyNumber: 100},
		{Total: 1000}, {Total: 1000, JerseyNumber: 500},
	}
	for _, eg := range cases {
		for s := 1; s <= eg.Total; s++ {
			tier, ok := eg.TierFor(s)
			if !ok {
				t.Errorf("eg=%+v serial=%d: !ok", eg, s)
				continue
			}
			ranges, err := eg.Ranges(tier)
			if err != nil {
				t.Fatalf("eg=%+v tier=%s: %v", eg, tier, err)
			}
			hit := false
			for _, r := range ranges {
				if s >= r.Min && s <= r.Max {
					hit = true
					break
				}
			}
			if !hit {
				t.Errorf("eg=%+v serial=%d: TierFor returned %s but ranges %+v don't contain serial",
					eg, s, tier, ranges)
			}
		}
	}
}

func TestRange_Empty(t *testing.T) {
	for _, tt := range []struct {
		r    Range
		want bool
	}{
		{Range{Min: 1, Max: 1}, false},
		{Range{Min: 0, Max: 0}, true},
		{Range{Min: 5, Max: 4}, true},
		{Range{Min: 0, Max: 10}, true},
	} {
		if got := tt.r.Empty(); got != tt.want {
			t.Errorf("Range%+v.Empty() = %v, want %v", tt.r, got, tt.want)
		}
	}
}
