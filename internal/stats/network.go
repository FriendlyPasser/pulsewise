package stats

import (
	"strings"
	"time"
)

const defaultMaxNetworkGap = 5 * time.Second

// Flag and interface type constants matching Darwin <net/if.h> and <net/if_types.h>.
const (
	iffUp           uint32 = 0x1
	iffLoopback     uint32 = 0x8
	iffPointToPoint uint32 = 0x10
	iftEther        uint8  = 0x6
)

type networkInterface struct {
	name   string
	ifType uint8
	flags  uint32
	rx     uint64
	tx     uint64
}

type netCounters struct {
	rx uint64
	tx uint64
}

func isPhysicalInterface(iface networkInterface) bool {
	if iface.flags&iffUp == 0 || iface.flags&(iffLoopback|iffPointToPoint) != 0 {
		return false
	}
	return iface.ifType == iftEther && hasDigitSuffix(iface.name, "en")
}

func hasDigitSuffix(name, prefix string) bool {
	rest, ok := strings.CutPrefix(name, prefix)
	if !ok || rest == "" {
		return false
	}
	for i := 0; i < len(rest); i++ {
		if rest[i] < '0' || rest[i] > '9' {
			return false
		}
	}
	return true
}

type networkTracker struct {
	maxGap  time.Duration
	hasPrev bool
	prevAt  uint64
	prev    map[string]netCounters
}

func newNetworkTracker(maxGap time.Duration) *networkTracker {
	return &networkTracker{maxGap: maxGap}
}

func (t *networkTracker) sample(ifaces []networkInterface, atNs uint64) Network {
	current := make(map[string]netCounters, len(ifaces))
	for _, iface := range ifaces {
		if !isPhysicalInterface(iface) {
			continue
		}
		prev := current[iface.name]
		current[iface.name] = netCounters{
			rx: prev.rx + iface.rx,
			tx: prev.tx + iface.tx,
		}
	}

	if !t.hasPrev || atNs <= t.prevAt || (t.maxGap > 0 && time.Duration(atNs-t.prevAt) > t.maxGap) {
		t.prev = current
		t.prevAt = atNs
		t.hasPrev = true
		return Network{}
	}
	dt := time.Duration(atNs - t.prevAt)

	var dRx, dTx uint64
	for name, cur := range current {
		prev, ok := t.prev[name]
		if !ok {
			continue
		}
		if cur.rx >= prev.rx {
			dRx += cur.rx - prev.rx
		}
		if cur.tx >= prev.tx {
			dTx += cur.tx - prev.tx
		}
	}

	t.prev = current
	t.prevAt = atNs

	secs := dt.Seconds()
	return Network{
		Down: float64(dRx) / secs,
		Up:   float64(dTx) / secs,
	}
}
