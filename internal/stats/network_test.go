package stats

import (
	"testing"
	"time"
)

func TestIsPhysicalInterface(t *testing.T) {
	upEther := iffUp
	tests := []struct {
		name  string
		iface networkInterface
		want  bool
	}{
		{"en0", networkInterface{name: "en0", ifType: iftEther, flags: upEther}, true},
		{"en5", networkInterface{name: "en5", ifType: iftEther, flags: upEther}, true},
		{"en12", networkInterface{name: "en12", ifType: iftEther, flags: upEther}, true},
		{"lo0", networkInterface{name: "lo0", ifType: iftEther, flags: upEther}, false},
		{"utun0", networkInterface{name: "utun0", ifType: iftEther, flags: upEther}, false},
		{"utun37", networkInterface{name: "utun37", ifType: iftEther, flags: upEther}, false},
		{"awdl0", networkInterface{name: "awdl0", ifType: iftEther, flags: upEther}, false},
		{"llw0", networkInterface{name: "llw0", ifType: iftEther, flags: upEther}, false},
		{"ap1", networkInterface{name: "ap1", ifType: iftEther, flags: upEther}, false},
		{"anpi0", networkInterface{name: "anpi0", ifType: iftEther, flags: upEther}, false},
		{"bridge0", networkInterface{name: "bridge0", ifType: iftEther, flags: upEther}, false},
		{"gif0", networkInterface{name: "gif0", ifType: iftEther, flags: upEther}, false},
		{"stf0", networkInterface{name: "stf0", ifType: iftEther, flags: upEther}, false},
		{"vmenet0", networkInterface{name: "vmenet0", ifType: iftEther, flags: upEther}, false},
		{"vmnet0", networkInterface{name: "vmnet0", ifType: iftEther, flags: upEther}, false},
		{"feth0", networkInterface{name: "feth0", ifType: iftEther, flags: upEther}, false},
		{"en_no_digits", networkInterface{name: "en", ifType: iftEther, flags: upEther}, false},
		{"en0a_non_digit", networkInterface{name: "en0a", ifType: iftEther, flags: upEther}, false},
		{"down_en0", networkInterface{name: "en0", ifType: iftEther, flags: 0}, false},
		{"p2p_en0", networkInterface{name: "en0", ifType: iftEther, flags: upEther | iffPointToPoint}, false},
		{"loopback_en0", networkInterface{name: "en0", ifType: iftEther, flags: upEther | iffLoopback}, false},
		{"non_ether_en0", networkInterface{name: "en0", ifType: 0x18, flags: upEther}, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := isPhysicalInterface(tc.iface); got != tc.want {
				t.Fatalf("isPhysicalInterface(%+v) = %v, want %v", tc.iface, got, tc.want)
			}
		})
	}
}

func TestNetworkTrackerRateMathAndAggregation(t *testing.T) {
	tr := newNetworkTracker(defaultMaxNetworkGap)
	t0 := uint64(10 * time.Second)

	first := tr.sample([]networkInterface{
		{name: "en0", ifType: iftEther, flags: iffUp, rx: 1000, tx: 500},
		{name: "en5", ifType: iftEther, flags: iffUp, rx: 2000, tx: 1000},
		{name: "utun37", ifType: iftEther, flags: iffUp, rx: 50000, tx: 50000},
		{name: "awdl0", ifType: iftEther, flags: iffUp, rx: 9000, tx: 9000},
		{name: "lo0", ifType: iftEther, flags: iffUp, rx: 8000, tx: 8000},
	}, t0)
	if first.Down != 0 || first.Up != 0 {
		t.Fatalf("first sample = %+v, want zero rates", first)
	}

	t1 := t0 + uint64(time.Second)
	second := tr.sample([]networkInterface{
		{name: "en0", ifType: iftEther, flags: iffUp, rx: 2500, tx: 900},
		{name: "en5", ifType: iftEther, flags: iffUp, rx: 2500, tx: 1600},
		{name: "utun37", ifType: iftEther, flags: iffUp, rx: 999999, tx: 999999},
		{name: "awdl0", ifType: iftEther, flags: iffUp, rx: 999999, tx: 999999},
		{name: "lo0", ifType: iftEther, flags: iffUp, rx: 999999, tx: 999999},
	}, t1)
	if second.Down != 2000 || second.Up != 1000 {
		t.Fatalf("1s aggregate sample = %+v, want Down=2000 Up=1000", second)
	}

	t2 := t1 + uint64(2500*time.Millisecond)
	third := tr.sample([]networkInterface{
		{name: "en0", ifType: iftEther, flags: iffUp, rx: 7500, tx: 3400},
		{name: "en5", ifType: iftEther, flags: iffUp, rx: 2500, tx: 1600},
	}, t2)
	if third.Down != 2000 || third.Up != 1000 {
		t.Fatalf("2.5s sample = %+v, want Down=2000 Up=1000", third)
	}
}

func TestNetworkTrackerCounterResetClampsAffectedDirection(t *testing.T) {
	tr := newNetworkTracker(defaultMaxNetworkGap)
	t0 := uint64(time.Second)

	tr.sample([]networkInterface{
		{name: "en0", ifType: iftEther, flags: iffUp, rx: 5000, tx: 1000},
		{name: "en5", ifType: iftEther, flags: iffUp, rx: 2000, tx: 3000},
	}, t0)

	got := tr.sample([]networkInterface{
		{name: "en0", ifType: iftEther, flags: iffUp, rx: 100, tx: 1400},
		{name: "en5", ifType: iftEther, flags: iffUp, rx: 2500, tx: 50},
	}, t0+uint64(time.Second))
	if got.Down != 500 || got.Up != 400 {
		t.Fatalf("counter reset sample = %+v, want Down=500 Up=400", got)
	}
}

func TestNetworkTrackerHotPlugAndRemoval(t *testing.T) {
	tr := newNetworkTracker(defaultMaxNetworkGap)
	t0 := uint64(time.Second)

	tr.sample([]networkInterface{
		{name: "en0", ifType: iftEther, flags: iffUp, rx: 1000, tx: 500},
	}, t0)

	t1 := t0 + uint64(time.Second)
	hot := tr.sample([]networkInterface{
		{name: "en0", ifType: iftEther, flags: iffUp, rx: 1500, tx: 750},
		{name: "en5", ifType: iftEther, flags: iffUp, rx: 1_000_000, tx: 500_000},
	}, t1)
	if hot.Down != 500 || hot.Up != 250 {
		t.Fatalf("hot-plug first tick = %+v, want Down=500 Up=250", hot)
	}

	t2 := t1 + uint64(time.Second)
	both := tr.sample([]networkInterface{
		{name: "en0", ifType: iftEther, flags: iffUp, rx: 1600, tx: 850},
		{name: "en5", ifType: iftEther, flags: iffUp, rx: 1_000_900, tx: 500_400},
	}, t2)
	if both.Down != 1000 || both.Up != 500 {
		t.Fatalf("hot-plug second tick = %+v, want Down=1000 Up=500", both)
	}

	t3 := t2 + uint64(time.Second)
	removed := tr.sample([]networkInterface{
		{name: "en0", ifType: iftEther, flags: iffUp, rx: 2000, tx: 1050},
	}, t3)
	if removed.Down != 400 || removed.Up != 200 {
		t.Fatalf("after removal = %+v, want Down=400 Up=200", removed)
	}
}

func TestNetworkTrackerSleepWakeAndClockEdgeCases(t *testing.T) {
	for _, gap := range []time.Duration{6 * time.Second, time.Hour} {
		t.Run(gap.String(), func(t *testing.T) {
			tr := newNetworkTracker(defaultMaxNetworkGap)
			t0 := uint64(10 * time.Second)
			tr.sample([]networkInterface{
				{name: "en0", ifType: iftEther, flags: iffUp, rx: 1000, tx: 1000},
			}, t0)

			tWake := t0 + uint64(gap)
			wake := tr.sample([]networkInterface{
				{name: "en0", ifType: iftEther, flags: iffUp, rx: 500_000, tx: 250_000},
			}, tWake)
			if wake.Down != 0 || wake.Up != 0 {
				t.Fatalf("wake sample = %+v, want zero rates", wake)
			}

			after := tr.sample([]networkInterface{
				{name: "en0", ifType: iftEther, flags: iffUp, rx: 501_200, tx: 250_300},
			}, tWake+uint64(time.Second))
			if after.Down != 1200 || after.Up != 300 {
				t.Fatalf("post-wake sample = %+v, want Down=1200 Up=300", after)
			}
		})
	}

	t.Run("non-monotonic clock", func(t *testing.T) {
		tr := newNetworkTracker(defaultMaxNetworkGap)
		t0 := uint64(10 * time.Second)
		tr.sample([]networkInterface{
			{name: "en0", ifType: iftEther, flags: iffUp, rx: 1000, tx: 1000},
		}, t0)

		for _, ts := range []uint64{t0, t0 - uint64(time.Second)} {
			got := tr.sample([]networkInterface{
				{name: "en0", ifType: iftEther, flags: iffUp, rx: 5000, tx: 5000},
			}, ts)
			if got.Down != 0 || got.Up != 0 {
				t.Fatalf("non-increasing timestamp sample = %+v, want zero rates", got)
			}
		}

		after := tr.sample([]networkInterface{
			{name: "en0", ifType: iftEther, flags: iffUp, rx: 5400, tx: 5200},
		}, t0)
		if after.Down != 400 || after.Up != 200 {
			t.Fatalf("recovery sample = %+v, want Down=400 Up=200", after)
		}
	})
}
