//go:build darwin

package stats

import "testing"

func TestDarwinNetworkConstantsMatchHeaders(t *testing.T) {
	up, loopback, p2p, ether := darwinNetworkHeaderConstants()
	if iffUp != up {
		t.Fatalf("iffUp = %#x, want %#x", iffUp, up)
	}
	if iffLoopback != loopback {
		t.Fatalf("iffLoopback = %#x, want %#x", iffLoopback, loopback)
	}
	if iffPointToPoint != p2p {
		t.Fatalf("iffPointToPoint = %#x, want %#x", iffPointToPoint, p2p)
	}
	if iftEther != ether {
		t.Fatalf("iftEther = %#x, want %#x", iftEther, ether)
	}
}

func TestReadNetworkInterfacesAndContinuousNow(t *testing.T) {
	t1 := continuousNow()
	ifaces, ok := readNetworkInterfaces()
	t2 := continuousNow()

	if !ok {
		t.Fatal("readNetworkInterfaces failed")
	}
	if len(ifaces) == 0 {
		t.Fatal("readNetworkInterfaces returned no interfaces")
	}
	if t1 == 0 || t2 < t1 {
		t.Fatalf("continuousNow timestamps invalid: t1=%d, t2=%d", t1, t2)
	}

	var foundLoopback bool
	for _, iface := range ifaces {
		if iface.name == "lo0" {
			foundLoopback = true
			break
		}
	}
	if !foundLoopback {
		t.Fatalf("expected lo0 in interfaces, got %+v", ifaces)
	}
}
