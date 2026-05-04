// Package serialtier is the canonical home for the serial-tier vocabulary and
// bucket math used across services that price, filter, or stat moments by
// rarity band. The tier strings on the wire MUST match what consumers
// (FE, fandom-event-admin-service, claw-service, listing-service,
// sales-history-service, spinner-bff-service) expect — historically each
// service kept its own private copy; this package consolidates them so the
// bucket boundaries cannot drift between services.
package serialtier

import "fmt"

// Tier identifies a serial-rarity band.
type Tier string

const (
	TierUltraPremium Tier = "ULTRA_PREMIUM" // serial #1
	Tier2To10        Tier = "2_TO_10"       // serials #2..#10
	Tier10To15       Tier = "10_TO_15"      // top 15% (after #10)
	Tier15To40       Tier = "15_TO_40"      // 15% to 40%
	Tier40To75       Tier = "40_TO_75"      // 40% to 75%
	Tier75To100      Tier = "75_TO_100"     // 75% to 100%
)

// AllTiers returns every tier in ordered-by-rarity sequence (rarest first).
// Order matters: TierForSerial walks tiers in this order and returns the first
// match, so adjacent bands cannot leave a serial in two buckets.
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

// Range is [Min,Max] inclusive for one tier given the event group's total
// fraction count.
type Range struct {
	Tier Tier
	Min  int
	Max  int
}

// Empty reports whether the range covers zero serials. Bands collapse to empty
// when the percentile math produces Min > Max for very small totals (e.g. a
// 9-serial event group has no 10_TO_15 band).
func (r Range) Empty() bool { return r.Min == 0 || r.Max == 0 || r.Min > r.Max }

// ResolveTierRange returns the [Min,Max] band for tier given the event group's
// total fraction count. Returns an error for unknown tiers or non-positive
// totals; an "empty" Range (Min > Max) is a valid return for tiers whose
// percentile cutoff falls below the previous band's upper bound.
func ResolveTierRange(tier Tier, total int) (Range, error) {
	if total <= 0 {
		return Range{Tier: tier}, fmt.Errorf("total must be positive, got %d", total)
	}
	switch tier {
	case TierUltraPremium:
		return Range{Tier: tier, Min: 1, Max: 1}, nil
	case Tier2To10:
		upper := minInt(10, total)
		if upper < 2 {
			return Range{Tier: tier, Min: 2, Max: 1}, nil
		}
		return Range{Tier: tier, Min: 2, Max: upper}, nil
	case Tier10To15:
		return bandFromPct(tier, total, 10, 0.15), nil
	case Tier15To40:
		return bandFromPct(tier, total, pctFloor(total, 0.15), 0.40), nil
	case Tier40To75:
		return bandFromPct(tier, total, pctFloor(total, 0.40), 0.75), nil
	case Tier75To100:
		mn := maxInt(pctFloor(total, 0.75)+1, 1)
		return Range{Tier: tier, Min: mn, Max: total}, nil
	default:
		return Range{Tier: tier}, fmt.Errorf("unknown serial tier: %s", tier)
	}
}

// TierForSerial returns the tier whose [Min,Max] band contains serialNo.
// Returns ("", false) when no band matches (e.g. serialNo < 1 or > total, or
// total <= 0). Walks tiers in AllTiers() order so the rarer tier wins on the
// boundary case.
func TierForSerial(total, serialNo int) (Tier, bool) {
	for _, t := range AllTiers() {
		r, err := ResolveTierRange(t, total)
		if err != nil || r.Empty() {
			continue
		}
		if serialNo >= r.Min && serialNo <= r.Max {
			return t, true
		}
	}
	return "", false
}

func bandFromPct(tier Tier, total int, prevUpper int, pct float64) Range {
	upper := minInt(pctCeil(total, pct), total)
	mn := prevUpper + 1
	if mn > total {
		return Range{Tier: tier, Min: mn, Max: mn - 1}
	}
	if upper < mn {
		upper = mn - 1
	}
	return Range{Tier: tier, Min: mn, Max: upper}
}

func pctCeil(total int, pct float64) int {
	v := float64(total) * pct
	n := int(v)
	if v > float64(n) {
		n++
	}
	return n
}

func pctFloor(total int, pct float64) int { return int(float64(total) * pct) }

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
