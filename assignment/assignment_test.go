package assignment

import (
	"math"
	"testing"
	"time"
)

var epoch = time.Unix(0, 0).UTC()

func at(d time.Duration) time.Time { return epoch.Add(d) }

func mustRotation(t *testing.T, cfg Config) *Rotation {
	t.Helper()

	r, err := MakeRotation(cfg)
	if err != nil {
		t.Fatalf("MakeRotation: %v", err)
	}

	return r
}

func TestMakeRotation_Rejects(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
	}{
		{"no clusters", Config{Current: "el", Period: time.Minute}},
		{"empty name", Config{Clusters: []string{"el", ""}, Current: "el", Period: time.Minute}},
		{"surrounding space", Config{Clusters: []string{"el", " xc"}, Current: "el", Period: time.Minute}},
		{"duplicate", Config{Clusters: []string{"el", "el"}, Current: "el", Period: time.Minute}},
		{"unknown current", Config{Clusters: []string{"el", "xc"}, Current: "dm", Period: time.Minute}},
		{"case matters", Config{Clusters: []string{"el", "xc"}, Current: "EL", Period: time.Minute}},
		{"zero period", Config{Clusters: []string{"el"}, Current: "el"}},
		{"negative period", Config{Clusters: []string{"el"}, Current: "el", Period: -time.Minute}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := MakeRotation(tc.cfg); err == nil {
				t.Error("accepted an invalid config")
			}
		})
	}
}

// The order given must not matter, and the caller must be free to reuse its
// slice afterwards.
func TestMakeRotation_SortsAndCopies(t *testing.T) {
	clusters := []string{"xc", "el", "dm"}

	rotation := mustRotation(t, Config{Clusters: clusters, Current: "el", Period: time.Minute})

	clusters[0] = "zz"

	decision, err := rotation.Decide(Invocation{ScheduledFor: at(0)})
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if decision.Owner != "dm" {
		t.Errorf("slot 0 owner = %q, want dm (sorted first)", decision.Owner)
	}
	if rotation.ClusterCount() != 3 {
		t.Errorf("ClusterCount() = %d, want 3", rotation.ClusterCount())
	}
}

// Every permutation of the same set must produce the same owner, or clusters
// would disagree with each other.
func TestDecide_PermutationsAgree(t *testing.T) {
	permutations := [][]string{
		{"el", "xc", "dm"},
		{"dm", "el", "xc"},
		{"xc", "dm", "el"},
		{"el", "dm", "xc"},
	}

	for slot := range int64(9) {
		var want string

		for _, clusters := range permutations {
			rotation := mustRotation(t, Config{Clusters: clusters, Current: "el", Period: 4 * time.Hour})

			decision, err := rotation.Decide(Invocation{ScheduledFor: at(time.Duration(slot) * 4 * time.Hour)})
			if err != nil {
				t.Fatalf("Decide: %v", err)
			}
			if decision.Slot != slot {
				t.Fatalf("Slot = %d, want %d", decision.Slot, slot)
			}

			if len(want) == 0 {
				want = decision.Owner
				continue
			}
			if decision.Owner != want {
				t.Fatalf("slot %d: %v gave owner %q, want %q", slot, clusters, decision.Owner, want)
			}
		}
	}
}

func TestDecide_RotatesInSortedOrder(t *testing.T) {
	rotation := mustRotation(t, Config{
		Clusters: []string{"el", "xc", "dm"},
		Current:  "el",
		Period:   4 * time.Hour,
	})

	// Sorted: dm, el, xc.
	want := []string{"dm", "el", "xc", "dm", "el", "xc"}

	for slot, owner := range want {
		decision, err := rotation.Decide(Invocation{
			ScheduledFor: at(time.Duration(slot) * 4 * time.Hour),
		})
		if err != nil {
			t.Fatalf("Decide: %v", err)
		}

		if decision.Owner != owner {
			t.Errorf("slot %d owner = %q, want %q", slot, decision.Owner, owner)
		}
		if got := decision.Execute; got != (owner == "el") {
			t.Errorf("slot %d Execute = %v for owner %q", slot, got, owner)
		}
	}
}

// One cluster owns every point; the policy is then a no-op that still validates.
func TestDecide_SingleCluster(t *testing.T) {
	rotation := mustRotation(t, Config{Clusters: []string{"el"}, Current: "el", Period: time.Minute})

	for slot := range int64(5) {
		decision, err := rotation.Decide(Invocation{ScheduledFor: at(time.Duration(slot) * time.Minute)})
		if err != nil {
			t.Fatalf("Decide: %v", err)
		}
		if !decision.Execute || decision.Owner != "el" {
			t.Errorf("slot %d = %+v, want el executing", slot, decision)
		}
	}
}

func TestDecide_Rejects(t *testing.T) {
	rotation := mustRotation(t, Config{
		Clusters: []string{"el", "xc", "dm"},
		Current:  "el",
		Period:   5 * time.Minute,
	})

	tests := []struct {
		name string
		when time.Time
	}{
		{"zero time", time.Time{}},
		{"off the grid", at(5*time.Minute + time.Second)},
		{"off the grid by a nanosecond", at(5*time.Minute + 1)},
		{"before the epoch", epoch.Add(-5 * time.Minute)},
		{"beyond the representable range", time.Unix(math.MaxInt64/1_000_000_000+1, 0)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := rotation.Decide(Invocation{ScheduledFor: tc.when}); err == nil {
				t.Error("accepted an invalid invocation")
			}
		})
	}
}

// A late attempt and its retry decide as the original point did.
func TestDecide_OwnerFollowsThePointNotTheClock(t *testing.T) {
	rotation := mustRotation(t, Config{
		Clusters: []string{"el", "xc", "dm"},
		Current:  "el",
		Period:   4 * time.Hour,
	})

	point := Invocation{ScheduledFor: at(4 * time.Hour)} // slot 1, owner el

	first, err := rotation.Decide(point)
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}

	// The same point decided again - hours later, on a retry - is unchanged.
	second, err := rotation.Decide(point)
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}

	if first != second {
		t.Errorf("the same point gave %+v then %+v", first, second)
	}
	if !first.Execute {
		t.Errorf("slot 1 should belong to el, got %q", first.Owner)
	}
}

// The timezone of the point is not part of its identity: the grid is absolute.
func TestDecide_LocationDoesNotChangeTheOwner(t *testing.T) {
	rotation := mustRotation(t, Config{
		Clusters: []string{"el", "xc", "dm"},
		Current:  "el",
		Period:   4 * time.Hour,
	})

	utc := at(8 * time.Hour)

	elsewhere, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip(err)
	}

	a, err := rotation.Decide(Invocation{ScheduledFor: utc})
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	b, err := rotation.Decide(Invocation{ScheduledFor: utc.In(elsewhere)})
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}

	if a.Owner != b.Owner || a.Slot != b.Slot {
		t.Errorf("same instant gave %+v and %+v", a, b)
	}
}

func TestOwnedBetween(t *testing.T) {
	// Sorted: dm, el, xc - so el owns slots 1, 4, 7, ...
	rotation := mustRotation(t, Config{
		Clusters: []string{"el", "xc", "dm"},
		Current:  "el",
		Period:   time.Minute,
	})

	tests := []struct {
		name     string
		from, to int64
		want     int
	}{
		{"empty range", 3, 3, 0},
		{"reversed range", 5, 2, 0},
		{"one foreign slot", 2, 3, 0},
		{"one owned slot", 3, 4, 1},
		{"a full turn is one of mine", 1, 4, 1},
		{"two turns", 1, 7, 2},
		{"across the start", 0, 10, 4}, // slots 1, 4, 7, 10
		{"a day of one-minute slots", 0, 1440, 480},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := rotation.OwnedBetween(tc.from, tc.to); got != tc.want {
				t.Errorf("OwnedBetween(%d, %d) = %d, want %d", tc.from, tc.to, got, tc.want)
			}
		})
	}
}

// The arithmetic must agree with counting the slots one by one.
func TestOwnedBetween_MatchesAWalk(t *testing.T) {
	rotation := mustRotation(t, Config{
		Clusters: []string{"el", "xc", "dm"},
		Current:  "el",
		Period:   time.Minute,
	})

	for from := range int64(12) {
		for to := from; to < 24; to++ {
			want := 0
			for slot := from + 1; slot <= to; slot++ {
				decision, err := rotation.Decide(Invocation{
					ScheduledFor: at(time.Duration(slot) * time.Minute),
				})
				if err != nil {
					t.Fatalf("Decide: %v", err)
				}
				if decision.Execute {
					want++
				}
			}

			if got := rotation.OwnedBetween(from, to); got != want {
				t.Fatalf("OwnedBetween(%d, %d) = %d, walk says %d", from, to, got, want)
			}
		}
	}
}
