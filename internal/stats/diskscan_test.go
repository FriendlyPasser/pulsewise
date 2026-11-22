package stats

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeRealFile(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	data := bytes.Repeat([]byte{'a'}, size)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func waitForTerminalScan(t *testing.T, scanner *DiskScanner) DiskScanStatus {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		st := scanner.Status()
		if st.State != DiskScanScanning {
			return st
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("scan did not finish in time; last status: %+v", scanner.Status())
	return DiskScanStatus{}
}

func TestDiskScannerTopNRankingAndTieBreaking(t *testing.T) {
	root := t.TempDir()
	// Use distinct multiples of 16 KiB so APFS 4 KiB blocks differ reliably.
	writeRealFile(t, filepath.Join(root, "a.bin"), 16*1024)
	writeRealFile(t, filepath.Join(root, "sub", "b.bin"), 64*1024)
	writeRealFile(t, filepath.Join(root, "sub", "c.bin"), 32*1024)
	writeRealFile(t, filepath.Join(root, "sub", "z-tie.bin"), 48*1024)
	writeRealFile(t, filepath.Join(root, "a-tie.bin"), 48*1024)
	writeRealFile(t, filepath.Join(root, "empty.bin"), 0)

	scanner := newDiskScanner(context.Background(), 3, root)
	started, err := scanner.Start(root)
	if err != nil {
		t.Fatal(err)
	}
	if started.State != DiskScanScanning || started.ID != 1 {
		t.Fatalf("unexpected start status: %+v", started)
	}

	got := waitForTerminalScan(t, scanner)
	if got.State != DiskScanCompleted {
		t.Fatalf("state = %s, err = %s", got.State, got.Error)
	}
	if got.Root != root {
		t.Fatalf("root = %q, want %q", got.Root, root)
	}
	if got.Incomplete {
		t.Fatalf("expected complete scan, got %+v", got)
	}
	if !got.Truncated {
		t.Fatalf("expected Truncated=true for 5 nonzero files with limit 3")
	}
	if got.ScannedFiles != 6 {
		t.Fatalf("ScannedFiles = %d, want 6", got.ScannedFiles)
	}
	if len(got.Files) != 3 {
		t.Fatalf("len(Files) = %d, want 3: %+v", len(got.Files), got.Files)
	}
	wantNames := []string{"b.bin", "a-tie.bin", "z-tie.bin"}
	for i, want := range wantNames {
		if got.Files[i].Name != want {
			t.Fatalf("Files[%d].Name = %q, want %q (%+v)", i, got.Files[i].Name, want, got.Files)
		}
		if got.Files[i].Size == 0 || got.Files[i].Dir == "" || got.Files[i].Path == "" {
			t.Fatalf("Files[%d] missing metadata: %+v", i, got.Files[i])
		}
	}
	if got.Files[0].Dir != filepath.Join(root, "sub") {
		t.Fatalf("Files[0].Dir = %q, want %q", got.Files[0].Dir, filepath.Join(root, "sub"))
	}
}

func TestDiskScannerSkipsSymlinksAndDedupsHardLinks(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "real.bin")
	writeRealFile(t, target, 32*1024)

	hardlink := filepath.Join(root, "hardlink.bin")
	if err := os.Link(target, hardlink); err != nil {
		t.Fatal(err)
	}
	fileSymlink := filepath.Join(root, "symfile.bin")
	if err := os.Symlink(target, fileSymlink); err != nil {
		t.Fatal(err)
	}
	dirSymlink := filepath.Join(root, "loopdir")
	if err := os.Symlink(root, dirSymlink); err != nil {
		t.Fatal(err)
	}

	scanner := newDiskScanner(context.Background(), 10, root)
	if _, err := scanner.Start(root); err != nil {
		t.Fatal(err)
	}
	got := waitForTerminalScan(t, scanner)
	if got.State != DiskScanCompleted {
		t.Fatalf("state = %s, err = %s", got.State, got.Error)
	}
	if got.ScannedFiles != 2 {
		t.Fatalf("ScannedFiles = %d, want 2 (regular + hardlink, skipping symlinks)", got.ScannedFiles)
	}
	if len(got.Files) != 1 {
		t.Fatalf("len(Files) = %d, want 1 after hard-link dedup: %+v", len(got.Files), got.Files)
	}
	if got.ScannedBytes != got.Files[0].Size {
		t.Fatalf("ScannedBytes = %d, want single inode size %d", got.ScannedBytes, got.Files[0].Size)
	}
}

func TestDiskScannerSkippedDirsAndUnreadableRoot(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("chmod 000 does not restrict root")
	}

	root := t.TempDir()
	locked := filepath.Join(root, "locked")
	writeRealFile(t, filepath.Join(locked, "secret.bin"), 64*1024)
	writeRealFile(t, filepath.Join(root, "visible.bin"), 16*1024)

	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o700) })

	scanner := newDiskScanner(context.Background(), 10, root)
	if _, err := scanner.Start(root); err != nil {
		t.Fatal(err)
	}
	got := waitForTerminalScan(t, scanner)
	if got.State != DiskScanCompleted {
		t.Fatalf("state = %s, err = %s", got.State, got.Error)
	}
	if !got.Incomplete || got.SkippedDirs != 1 {
		t.Fatalf("expected Incomplete=true and SkippedDirs=1, got %+v", got)
	}
	if len(got.SkippedSamples) != 1 || got.SkippedSamples[0] != locked {
		t.Fatalf("SkippedSamples = %v, want [%s]", got.SkippedSamples, locked)
	}
	if len(got.Files) != 1 || got.Files[0].Name != "visible.bin" {
		t.Fatalf("Files = %+v, want [visible.bin]", got.Files)
	}

	// Unreadable root must transition to error state.
	badRoot := t.TempDir()
	if err := os.Chmod(badRoot, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(badRoot, 0o700) })

	if _, err := scanner.Start(badRoot); err != nil {
		t.Fatal(err)
	}
	errStatus := waitForTerminalScan(t, scanner)
	if errStatus.State != DiskScanError || !errStatus.Incomplete || errStatus.Error == "" {
		t.Fatalf("expected error status, got %+v", errStatus)
	}
}

func TestDiskScannerCancelAndSupersede(t *testing.T) {
	root := t.TempDir()
	for i := range 60 {
		writeRealFile(t, filepath.Join(root, filepath.FromSlash("d/"+string(rune('a'+i%26))), "f-"+string(rune('0'+i%10))+".bin"), 8*1024)
	}

	appCtx, appCancel := context.WithCancel(context.Background())
	defer appCancel()
	scanner := newDiskScanner(appCtx, 10, root)

	if _, err := scanner.Start(root); err != nil {
		t.Fatal(err)
	}
	cancelled := scanner.Cancel()
	if cancelled.State != DiskScanCancelled && cancelled.State != DiskScanCompleted {
		t.Fatalf("unexpected state after Cancel: %+v", cancelled)
	}
	if cancelled.State == DiskScanCancelled && !cancelled.Incomplete {
		t.Fatalf("cancelled scan should be marked incomplete: %+v", cancelled)
	}

	// Pre-cancelled context guarantees deterministic cancellation path.
	deadCtx, deadCancel := context.WithCancel(context.Background())
	deadCancel()
	deadScanner := newDiskScanner(deadCtx, 10, root)
	if _, err := deadScanner.Start(root); err != nil {
		t.Fatal(err)
	}
	deadStatus := waitForTerminalScan(t, deadScanner)
	if deadStatus.State != DiskScanCancelled || !deadStatus.Incomplete {
		t.Fatalf("expected cancelled status from cancelled app context, got %+v", deadStatus)
	}

	// Superseding a scan bumps ID cleanly and finishes with the latest root.
	otherRoot := t.TempDir()
	writeRealFile(t, filepath.Join(otherRoot, "winner.bin"), 32*1024)
	if _, err := scanner.Start(root); err != nil {
		t.Fatal(err)
	}
	second, err := scanner.Start(otherRoot)
	if err != nil {
		t.Fatal(err)
	}
	final := waitForTerminalScan(t, scanner)
	if final.ID != second.ID || final.Root != otherRoot || final.State != DiskScanCompleted {
		t.Fatalf("final = %+v, want ID=%d Root=%q completed", final, second.ID, otherRoot)
	}
	if len(final.Files) != 1 || final.Files[0].Name != "winner.bin" {
		t.Fatalf("final.Files = %+v, want [winner.bin]", final.Files)
	}
}

func TestResolveScanRootAndDefaultDiskScopes(t *testing.T) {
	home := t.TempDir()
	downloads := filepath.Join(home, "Downloads")
	documents := filepath.Join(home, "Documents")
	filePath := filepath.Join(home, "note.txt")
	if err := os.MkdirAll(downloads, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(documents, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filePath, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	defRoot, presets := DefaultDiskScopes(home)
	if defRoot != downloads {
		t.Fatalf("defaultRoot = %q, want %q", defRoot, downloads)
	}
	if len(presets) != 3 || presets[0].ID != "downloads" || presets[1].ID != "documents" || presets[2].ID != "home" {
		t.Fatalf("presets = %+v", presets)
	}

	// Fallback when Downloads is absent.
	homeNoDownloads := t.TempDir()
	if err := os.MkdirAll(filepath.Join(homeNoDownloads, "Desktop"), 0o755); err != nil {
		t.Fatal(err)
	}
	defFallback, _ := DefaultDiskScopes(homeNoDownloads)
	if defFallback != filepath.Join(homeNoDownloads, "Desktop") {
		t.Fatalf("fallback defaultRoot = %q", defFallback)
	}

	if got, err := ResolveScanRoot("~/Documents", downloads, home); err != nil || got != documents {
		t.Fatalf("ResolveScanRoot(~) = %q, %v", got, err)
	}
	if got, err := ResolveScanRoot("", downloads, home); err != nil || got != downloads {
		t.Fatalf("ResolveScanRoot(empty) = %q, %v", got, err)
	}

	for _, tc := range []struct {
		name string
		raw  string
		want error
	}{
		{"relative", "Downloads", ErrInvalidScanRoot},
		{"nul byte", home + "\x00bad", ErrInvalidScanRoot},
		{"missing", filepath.Join(home, "missing"), ErrScanRootNotFound},
		{"file not dir", filePath, ErrScanRootNotDir},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ResolveScanRoot(tc.raw, downloads, home); !errors.Is(err, tc.want) {
				t.Fatalf("ResolveScanRoot(%q) err = %v, want %v", tc.raw, err, tc.want)
			}
		})
	}
}
