/*
Package assignment picks one cluster to own a scheduled point when the same
deployment runs in several of them.

It is a pure policy: it reads no clock, opens no connection and keeps no state
between decisions. Every process computes the same answer for the same point
from the same configuration, so nothing has to be coordinated at run time.

	rotation, err := assignment.MakeRotation(assignment.Config{
		Clusters: []string{"el", "xc", "dm"},
		Current:  clusterName,
		Period:   5 * time.Minute,
	})

	decision, err := rotation.Decide(assignment.Invocation{ScheduledFor: point})

Ownership rotates over the clusters in sorted order, one point each:

	slot    = (ScheduledFor - Unix epoch) / Period
	owner   = sorted(Clusters)[slot % len(Clusters)]
	execute = owner == Current

# What it does and does not promise

It decides who should try, not that anyone did. If the owner of a point is down,
that point is simply not served - no other cluster takes it over, because that
would need shared state this package deliberately does not have. The work must
therefore survive a skipped attempt: keep it in a queue and let the next
attempt pick it up.

It also does not prevent overlap. A delayed attempt and its retry keep the owner
of their original point, so an owner still working on one point can overlap the
next owner's work.

With N clusters a point comes round to each of them every N periods. Three
clusters on a five-minute period means each one gets an attempt every fifteen
minutes, and the whole deployment attempts every five.

# Changing the deployment

A cluster added, removed or renamed - or a different period - changes who owns
which point, and every process has to be deciding from the same configuration
for the rotation to divide anything. Nothing here checks that they are, so a
rolling update, with the old and the new configuration deciding side by side,
produces duplicates and gaps at the same time.

Change the topology by restarting every pod in every cluster, and accept the
pause in the schedule while that happens. There is no hot reconfiguration and no
agreement protocol, by choice: either one would need exactly the shared state
this package exists without.

# What it needs from the caller

The identity of the scheduled point, not the current time. A pod that starts
late, or a retry of the same scheduled Job, must decide as the original point
did - so the caller passes that point in rather than reading a clock. A
long-lived scheduler takes it from its own grid, before any per-replica offset:
an offset staggers replicas, it must not move a point to another cluster.
*/
package assignment

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"
)

// Config describes the deployment this process belongs to.
type Config struct {
	// Clusters is every cluster the deployment runs in. The list is copied and
	// sorted, so the order given does not matter, but the set must be the same
	// in every cluster or they will not agree on an owner.
	Clusters []string

	// Current is the cluster this process runs in. It must be one of Clusters:
	// an identity the list does not know is a configuration error, never a
	// reason to run everywhere.
	Current string

	// Period is the spacing of the scheduled points, which must match the
	// schedule that produces them.
	Period time.Duration
}

// Invocation is the scheduled point a decision is about.
type Invocation struct {
	// ScheduledFor is the nominal point of the grid, before any per-replica
	// offset. It is an identity, not a measurement: a late start or a retry
	// carries the point it was meant to serve.
	ScheduledFor time.Time
}

// Decision is what the policy concluded about one point.
type Decision struct {
	Invocation Invocation
	Slot       int64
	Owner      string
	Execute    bool
}

// Rotation answers who owns a scheduled point. Build it with MakeRotation; it
// is immutable and safe for concurrent use.
type Rotation struct {
	clusters []string
	current  string
	period   time.Duration

	// ownRemainder is the remainder every slot of this process leaves: the point
	// is ours exactly when slot % len(clusters) equals it.
	ownRemainder int64
}

// MakeRotation validates cfg and returns the policy it describes. The cluster
// list is copied and sorted, so callers may reuse their slice.
//
// Identifiers are case-sensitive and must be non-empty and free of surrounding
// space; an empty list, a duplicate, a Current the list does not contain, or a
// non-positive Period are errors. Nothing is corrected silently: a typo that
// quietly turned into "run everywhere" would be worse than a failed start.
func MakeRotation(cfg Config) (*Rotation, error) {
	if len(cfg.Clusters) == 0 {
		return nil, errors.New("clusters are required")
	}

	if cfg.Period <= 0 {
		return nil, fmt.Errorf("period %v must be > 0", cfg.Period)
	}

	clusters := slices.Clone(cfg.Clusters)
	for _, name := range clusters {
		if len(name) == 0 {
			return nil, errors.New("cluster names must not be empty")
		}
		if name != strings.TrimSpace(name) {
			return nil, fmt.Errorf("cluster name %q has surrounding space", name)
		}
	}

	slices.Sort(clusters)
	if i := firstDuplicate(clusters); i >= 0 {
		return nil, fmt.Errorf("cluster %q is listed twice", clusters[i])
	}

	if !slices.Contains(clusters, cfg.Current) {
		return nil, fmt.Errorf("current cluster %q is not one of %v", cfg.Current, clusters)
	}

	return &Rotation{
		clusters:     clusters,
		current:      cfg.Current,
		period:       cfg.Period,
		ownRemainder: int64(slices.Index(clusters, cfg.Current)),
	}, nil
}

// ClusterCount reports how many clusters share the rotation. A scheduler uses
// it to describe its own cadence: a point comes round to one cluster every
// ClusterCount periods.
func (r *Rotation) ClusterCount() int { return len(r.clusters) }

// Period reports the spacing the policy was built for, so a scheduler can check
// that it matches its own.
func (r *Rotation) Period() time.Duration { return r.period }

// Current reports the cluster this process runs in.
func (r *Rotation) Current() string { return r.current }

// Decide reports who owns the given point and whether this process should run
// it. An invocation that is not a point of the configured grid is an error, not
// a silent yes or no.
func (r *Rotation) Decide(in Invocation) (Decision, error) {
	if in.ScheduledFor.IsZero() {
		return Decision{}, errors.New("scheduled point is required")
	}

	elapsed, ok := nanosSinceEpoch(in.ScheduledFor)
	if !ok {
		return Decision{}, fmt.Errorf("scheduled point %v is outside the representable range", in.ScheduledFor)
	}

	period := int64(r.period)
	if elapsed%period != 0 {
		return Decision{}, fmt.Errorf("scheduled point %v is not on the %v grid", in.ScheduledFor, r.period)
	}

	slot := elapsed / period
	owner := r.clusters[slot%int64(len(r.clusters))]

	return Decision{
		Invocation: in,
		Slot:       slot,
		Owner:      owner,
		Execute:    owner == r.current,
	}, nil
}

// OwnedBetween reports how many of the slots in (from, to] belong to this
// process. It is the arithmetic a scheduler needs to say how many of its own
// scheduled points went by unserved, without walking each one: a clock that
// jumped a day must not cost a day of iterations.
//
// An empty or reversed range reports zero.
func (r *Rotation) OwnedBetween(from, to int64) int {
	if to <= from {
		return 0
	}

	count := len(r.clusters)

	return int(floorDiv(to-r.ownRemainder, int64(count)) - floorDiv(from-r.ownRemainder, int64(count)))
}

// floorDiv divides rounding towards negative infinity, which Go's / does not do
// for negative numerators - and the numerator here is negative whenever the
// range starts before this process's first owned slot.
func floorDiv(a, b int64) int64 {
	q := a / b
	if a%b != 0 && (a < 0) != (b < 0) {
		q--
	}

	return q
}

// nanosSinceEpoch converts t to nanoseconds since the Unix epoch without the
// silent wrap of UnixNano or the saturation of Sub. Points before the epoch and
// beyond the int64 range of nanoseconds are refused rather than folded onto
// some other slot.
func nanosSinceEpoch(t time.Time) (int64, bool) {
	const nsPerSec = int64(time.Second)

	secs := t.Unix()
	if secs < 0 || secs > math.MaxInt64/nsPerSec {
		return 0, false
	}

	whole := secs * nsPerSec

	fraction := int64(t.Nanosecond())
	if whole > math.MaxInt64-fraction {
		return 0, false
	}

	return whole + fraction, true
}

// firstDuplicate reports the index of the first repeat in a sorted slice, or -1.
func firstDuplicate(sorted []string) int {
	for i := 1; i < len(sorted); i++ {
		if sorted[i] == sorted[i-1] {
			return i
		}
	}

	return -1
}
