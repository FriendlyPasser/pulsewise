package stats

import (
	"os"
	"reflect"
	"testing"
	"time"
)

func TestOutermostAppName(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{
			path: "/Applications/WeChat.app/Contents/MacOS/WeChat",
			want: "WeChat",
		},
		{
			path: "/Applications/Visual Studio Code.app/Contents/Frameworks/Code Helper (Plugin).app/Contents/MacOS/Code Helper (Plugin)",
			want: "Visual Studio Code",
		},
		{
			path: "/private/var/folders/x/Google Chrome.app.bundle/Contents/MacOS/Google Chrome",
			want: "Google Chrome",
		},
		{
			path: "/Applications/Google Chrome.app/Contents/Frameworks/Google Chrome Framework.framework/Versions/157/Helpers/Google Chrome Helper (Renderer).app/Contents/MacOS/Google Chrome Helper (Renderer)",
			want: "Google Chrome",
		},
		{
			path: "/Applications/Foo.app.bundle/Contents/Frameworks/Bar.app/Contents/MacOS/Bar",
			want: "Foo",
		},
		{
			path: "/opt/homebrew/bin/node",
			want: "",
		},
		{
			path: "/.app/Contents/MacOS/Bad",
			want: "",
		},
	}
	for _, tc := range cases {
		if got := outermostAppName(tc.path); got != tc.want {
			t.Errorf("outermostAppName(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}

func TestGroupProcessesAttributionAndSorting(t *testing.T) {
	procs := []rawProcess{
		{pid: 0, ppid: 1, name: "kernel", memory: 100},
		{pid: 99, ppid: 1, name: "zombie", memory: 0},
		{
			pid:    10,
			ppid:   1,
			rpid:   10,
			name:   "Code",
			path:   "/Applications/Visual Studio Code.app/Contents/MacOS/Electron",
			memory: 200,
		},
		{
			pid:    11,
			ppid:   10,
			rpid:   10,
			name:   "Code Helper (Plugin",
			path:   "/Applications/Visual Studio Code.app/Contents/Frameworks/Code Helper (Plugin).app/Contents/MacOS/Code Helper (Plugin)",
			memory: 500,
		},
		{
			pid:    12,
			ppid:   10,
			rpid:   10,
			name:   "zsh",
			path:   "/bin/zsh",
			memory: 20,
		},
		{
			pid:    13,
			ppid:   12,
			rpid:   10,
			name:   "gopls",
			path:   "/Users/dev/go/bin/gopls",
			memory: 300,
		},
		{
			pid:    20,
			ppid:   1,
			rpid:   20,
			name:   "Google Chrome",
			path:   "/private/var/folders/x/Google Chrome.app.bundle/Contents/MacOS/Google Chrome",
			memory: 600,
		},
		{
			pid:    21,
			ppid:   20,
			rpid:   20,
			name:   "Google Chrome Helper (Aperitif",
			path:   "/Applications/Google Chrome.app/Contents/Frameworks/Google Chrome Framework.framework/Helpers/Google Chrome Helper (Renderer).app/Contents/MacOS/Google Chrome Helper (Renderer)",
			memory: 700,
		},
		{
			pid:    30,
			ppid:   1,
			rpid:   30,
			name:   "Outlook",
			path:   "/Applications/Microsoft Outlook.app/Contents/MacOS/Microsoft Outlook",
			memory: 150,
		},
		{
			pid:    31,
			ppid:   1,
			rpid:   30,
			name:   "com.apple.WebKi",
			path:   "/System/Library/Frameworks/WebKit.framework/XPCServices/com.apple.WebKit.WebContent.xpc/Contents/MacOS/com.apple.WebKit.WebContent",
			memory: 250,
		},
		{
			pid:    40,
			ppid:   41,
			rpid:   41,
			name:   "cycle-a",
			path:   "/usr/local/bin/cycle-a",
			memory: 80,
		},
		{
			pid:    41,
			ppid:   40,
			rpid:   40,
			name:   "cycle-b",
			path:   "",
			memory: 80,
		},
		{
			pid:    50,
			ppid:   1,
			rpid:   50,
			name:   "sharingd",
			path:   "/usr/libexec/sharingd",
			memory: 120,
		},
	}

	got := groupProcesses(procs)
	want := MemoryDetail{
		Apps: []AppMemory{
			{
				Name:   "Google Chrome",
				Memory: 1300,
				Processes: []ProcessMemory{
					{PID: 21, Name: "Google Chrome Helper (Renderer)", Memory: 700},
					{PID: 20, Name: "Google Chrome", Memory: 600},
				},
			},
			{
				Name:   "Visual Studio Code",
				Memory: 1020,
				Processes: []ProcessMemory{
					{PID: 11, Name: "Code Helper (Plugin)", Memory: 500},
					{PID: 13, Name: "gopls", Memory: 300},
					{PID: 10, Name: "Electron", Memory: 200},
					{PID: 12, Name: "zsh", Memory: 20},
				},
			},
			{
				Name:   "Microsoft Outlook",
				Memory: 400,
				Processes: []ProcessMemory{
					{PID: 31, Name: "com.apple.WebKit.WebContent", Memory: 250},
					{PID: 30, Name: "Microsoft Outlook", Memory: 150},
				},
			},
		},
		Unattributed: []ProcessMemory{
			{PID: 50, Name: "sharingd", Memory: 120},
			{PID: 40, Name: "cycle-a", Memory: 80},
			{PID: 41, Name: "cycle-b", Memory: 80},
		},
		UnattributedMemory: 280,
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("groupProcesses mismatch:\ngot:  %+v\nwant: %+v", got, want)
	}
}

func TestGroupProcessesEmptyInitializesSlices(t *testing.T) {
	got := groupProcesses(nil)
	if got.Apps == nil || len(got.Apps) != 0 {
		t.Fatalf("Apps = %#v, want empty non-nil slice", got.Apps)
	}
	if got.Unattributed == nil || len(got.Unattributed) != 0 {
		t.Fatalf("Unattributed = %#v, want empty non-nil slice", got.Unattributed)
	}
	if got.UnattributedMemory != 0 {
		t.Fatalf("UnattributedMemory = %d, want 0", got.UnattributedMemory)
	}
}

func TestGroupProcessesFollowsResponsibilityChainBeyondDirectParent(t *testing.T) {
	procs := []rawProcess{
		{
			pid:    100,
			ppid:   1,
			rpid:   100,
			name:   "App",
			path:   "/Applications/ChatGPT.app/Contents/MacOS/ChatGPT",
			memory: 200,
		},
		{
			pid:    101,
			ppid:   100,
			rpid:   100,
			name:   "codex",
			path:   "/usr/local/bin/codex",
			memory: 50,
		},
		{
			pid:    102,
			ppid:   1,
			rpid:   101,
			name:   "xpc-worker",
			path:   "/System/Library/PrivateFrameworks/XPCService",
			memory: 90,
		},
	}

	got := groupProcesses(procs)
	if len(got.Apps) != 1 || got.Apps[0].Name != "ChatGPT" || got.Apps[0].Memory != 340 {
		t.Fatalf("Apps = %+v, want ChatGPT with 340 bytes", got.Apps)
	}
	if len(got.Unattributed) != 0 {
		t.Fatalf("Unattributed = %+v, want empty", got.Unattributed)
	}
}

func TestReadMemoryDetailLiveDarwin(t *testing.T) {
	detail := NewSampler(time.Hour).MemoryDetail()
	if detail.At == 0 || detail.Memory.Total == 0 {
		t.Fatalf("expected live memory total and timestamp, got %+v", detail)
	}
	selfPID := os.Getpid()
	foundSelf := false
	for _, app := range detail.Apps {
		var sum uint64
		for _, p := range app.Processes {
			sum += p.Memory
			if p.PID == selfPID && p.Memory > 0 {
				foundSelf = true
			}
		}
		if sum != app.Memory {
			t.Fatalf("app %q sum mismatch: sum=%d, memory=%d", app.Name, sum, app.Memory)
		}
	}
	var unattrSum uint64
	for _, p := range detail.Unattributed {
		unattrSum += p.Memory
		if p.PID == selfPID && p.Memory > 0 {
			foundSelf = true
		}
	}
	if unattrSum != detail.UnattributedMemory {
		t.Fatalf("unattributed sum mismatch: sum=%d, total=%d", unattrSum, detail.UnattributedMemory)
	}
	if !foundSelf {
		t.Fatalf("expected current process pid %d in detail output", selfPID)
	}
}
