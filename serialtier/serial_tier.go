// Package serialtier is the canonical home for the serial-tier vocabulary and
// bucket math used across services that price, filter, or stat moments by
// rarity band. The tier strings on the wire MUST match what consumers
// (FE, fandom-event-admin-service, claw-service, listing-service,
// sales-history-service, spinner-bff-service) expect — historically each
// service kept its own private copy; this package consolidates them so the
// bucket boundaries cannot drift between services.
//
// Callers hydrate an EventGroup value from their own data layer (the
// eventsGroup Mongo collection is the single source of truth for
// totalFractionCount and metaData.jerseyNumber) and call eg.Ranges / eg.TierFor.
// The package is pure math and has no opinion on persistence.
package serialtier

import (
	"fmt"
	"sort"
)

type Tier string

const (
	TierUltraPremium Tier = "ULTRA_PREMIUM" // {serial #1, #JerseyNumber, #Total}
	Tier2To10        Tier = "2_TO_10"       // serials #2..#10 (capped below #Total, jersey excluded)
	Tier10To15       Tier = "10_TO_15"      // top 15% (after #10), jersey/total excluded
	Tier15To40       Tier = "15_TO_40"      // 15% to 40%
	Tier40To75       Tier = "40_TO_75"      // 40% to 75%
	Tier75To100      Tier = "75_TO_100"     // 75% to <100% (jersey/total excluded)
)

// AllTiers returns every tier in ordered-by-rarity sequence (rarest first).
// Order matters: EventGroup.TierFor walks tiers in this order and returns the
// first match, so adjacent bands cannot leave a serial in two buckets.
// ULTRA_PREMIUM is listed first so the boundary serials (#1, #JerseyNumber,
// #Total) resolve to it rather than to the percentile band that would
// otherwise contain them.
func AllTiers() []Tier {
	return []Tier{
		TierUltraPremium,
		Tier2To10,
		Tier10To15,
		Tier15To40,
		Tier40To75,
		Tier75To100,
	}
}

// IsValid reports whether s is one of the known tier strings. Useful at
// trust-boundary points (HTTP request bodies, pubsub payloads) before casting
// to Tier — type Tier is a string alias so an unchecked cast would silently
// admit garbage.
func IsValid(s string) bool {
	switch Tier(s) {
	case TierUltraPremium, Tier2To10, Tier10To15, Tier15To40, Tier40To75, Tier75To100:
		return true
	}
	return false
}

// EventGroup carries the per-event-group inputs needed to bucket serials by
// rarity tier. Hydrate it from your service's data layer — the eventsGroup
// Mongo collection holds totalFractionCount and metaData.jerseyNumber.
//
// JerseyNumber == 0 means "no jersey for this event group" — the value type
// behaves as if only #1 and #Total are ULTRA_PREMIUM. Out-of-range jerseys
// (< 1 or > Total) are silently normalized to 0; callers don't have to
// pre-validate.
type EventGroup struct {
	Total        int
	JerseyNumber int
}

// Validate reports whether the event group is well-formed enough to bucket
// serials. Returns an error only for non-positive Total. Out-of-range
// JerseyNumber is normalized in Ranges/TierFor, not rejected here.
func (eg EventGroup) Validate() error {
	if eg.Total <= 0 {
		return fmt.Errorf("total must be positive, got %d", eg.Total)
	}
	return nil
}

// Range is [Min,Max] inclusive for one contiguous slice of one tier within an
// event group. ULTRA_PREMIUM produces 1-3 disjoint singleton ranges
// (#1, #JerseyNumber, #Total — deduped). Every percentile tier produces 1
// contiguous range when JerseyNumber is outside it, or up to 2 ranges when
// JerseyNumber falls inside the natural band (the band splits around the
// jersey serial).
type Range struct {
	Tier Tier
	Min  int
	Max  int
}

// Empty reports whether the range covers zero serials. Bands collapse to empty
// when the percentile math produces Min > Max for very small totals (e.g. a
// 9-serial event group has no 10_TO_15 band).
func (r Range) Empty() bool { return r.Min == 0 || r.Max == 0 || r.Min > r.Max }

// Ranges returns every [Min,Max] band for tier within this event group.
// ULTRA_PREMIUM owns the set {1, JerseyNumber, Total}, deduped and sorted;
// JerseyNumber is ignored when out of range (< 1 or > Total) or zero. Every
// percentile tier excludes any of {JerseyNumber, Total} that would otherwise
// fall inside its natural band — a percentile band split around an interior
// JerseyNumber returns two ranges. Empty bands are filtered out so callers
// always get a clean slice — len(out) == 0 means the tier has no serials at
// this Total. Returns an error for unknown tiers or non-positive Total.
func (eg EventGroup) Ranges(tier Tier) ([]Range, error) {
	if err := eg.Validate(); err != nil {
		return nil, err
	}
	jersey := normalizeJersey(eg.JerseyNumber, eg.Total)

	if tier == TierUltraPremium {
		return ultraPremiumRanges(eg.Total, jersey), nil
	}
	mn, mx, ok := percentileBounds(tier, eg.Total)
	if !ok {
		return nil, fmt.Errorf("unknown serial tier: %s", tier)
	}
	if mn > mx {
		return nil, nil
	}
	return splitAroundJersey(tier, mn, mx, jersey), nil
}

// TierFor returns the tier whose band contains serialNo within this event
// group. Returns ("", false) when no band matches (e.g. serialNo < 1 or >
// Total, or Total <= 0). Walks tiers in AllTiers() order so the rarer tier
// wins on the boundary case — #1, #JerseyNumber, and #Total resolve to
// ULTRA_PREMIUM, not to the percentile band that would otherwise contain them.
func (eg EventGroup) TierFor(serialNo int) (Tier, bool) {
	for _, t := range AllTiers() {
		ranges, err := eg.Ranges(t)
		if err != nil {
			continue
		}
		for _, r := range ranges {
			if serialNo >= r.Min && serialNo <= r.Max {
				return t, true
			}
		}
	}
	return "", false
}

// percentileBounds returns the [Min, Max] band for tier (jersey-blind) using
// the historical percentile cutoffs, or ok=false for non-percentile tiers.
// Each tier's lower bound is the previous tier's upper + 1 so bands never
// overlap, even at small totals where naive percentile floors would collide.
// Upper bounds for every non-ULTRA tier are clamped to total-1 so #total stays
// exclusive to ULTRA_PREMIUM.
func percentileBounds(tier Tier, total int) (int, int, bool) {
	upper2To10 := minInt(10, total-1)
	upper10To15 := maxInt(upper2To10, minInt(pctCeil(total, 0.15), total-1))
	upper15To40 := maxInt(upper10To15, minInt(pctCeil(total, 0.40), total-1))
	upper40To75 := maxInt(upper15To40, minInt(pctCeil(total, 0.75), total-1))
	upper75To100 := total - 1

	switch tier {
	case Tier2To10:
		return 2, upper2To10, true
	case Tier10To15:
		return upper2To10 + 1, upper10To15, true
	case Tier15To40:
		return upper10To15 + 1, upper15To40, true
	case Tier40To75:
		return upper15To40 + 1, upper40To75, true
	case Tier75To100:
		return upper40To75 + 1, upper75To100, true
	}
	return 0, 0, false
}

// normalizeJersey returns 0 when jerseyNumber is missing or out of range —
// callers downstream treat 0 as "no jersey, ULTRA_PREMIUM = {1, total} only."
func normalizeJersey(jerseyNumber, total int) int {
	if jerseyNumber < 1 || jerseyNumber > total {
		return 0
	}
	return jerseyNumber
}

// ultraPremiumRanges returns {1, jersey, total} as up to 3 sorted singleton
// ranges. jersey == 0 means "no valid jersey," dedupe handles total == 1 and
// jersey ∈ {1, total}.
func ultraPremiumRanges(total, jersey int) []Range {
	serials := []int{1, total}
	if jersey != 0 {
		serials = append(serials, jersey)
	}
	sort.Ints(serials)

	out := make([]Range, 0, len(serials))
	prev := 0
	for _, s := range serials {
		if s == prev {
			continue
		}
		out = append(out, Range{Tier: TierUltraPremium, Min: s, Max: s})
		prev = s
	}
	return out
}

// splitAroundJersey returns [mn, mx] with the jersey serial removed. When
// jersey is outside the band or zero, returns the single contiguous range.
// When jersey is at a boundary, returns the single trimmed range. When jersey
// is strictly interior, returns two ranges: [mn, jersey-1] and [jersey+1, mx].
func splitAroundJersey(tier Tier, mn, mx, jersey int) []Range {
	if jersey == 0 || jersey < mn || jersey > mx {
		return []Range{{Tier: tier, Min: mn, Max: mx}}
	}
	if jersey == mn && jersey == mx {
		return nil
	}
	if jersey == mn {
		return []Range{{Tier: tier, Min: mn + 1, Max: mx}}
	}
	if jersey == mx {
		return []Range{{Tier: tier, Min: mn, Max: mx - 1}}
	}
	return []Range{
		{Tier: tier, Min: mn, Max: jersey - 1},
		{Tier: tier, Min: jersey + 1, Max: mx},
	}
}

func pctCeil(total int, pct float64) int {
	v := float64(total) * pct
	n := int(v)
	if v > float64(n) {
		n++
	}
	return n
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
