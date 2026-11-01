package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Stage downloads rel's zip, checks it, and unpacks it beside bundle,
// ready for Install. It returns the unpacked app. progress, when set, is
// told the bytes so far and the total (0 while unknown).
func Stage(ctx context.Context, rel *Release, bundle string, progress func(done, total int64)) (string, error) {
	dir := stageDir(bundle)
	os.RemoveAll(dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	zip := filepath.Join(dir, "update.zip")
	if err := download(ctx, rel.Zip, zip, progress); err != nil {
		os.RemoveAll(dir)
		return "", err
	}
	out := filepath.Join(dir, "app")
	if b, err := exec.CommandContext(ctx, "ditto", "-x", "-k", zip, out).CombinedOutput(); err != nil {
		os.RemoveAll(dir)
		return "", fmt.Errorf("unzip: %v: %s", err, b)
	}
	os.Remove(zip)
	app := filepath.Join(out, filepath.Base(bundle))
	if err := sameSigner(ctx, bundle, app); err != nil {
		os.RemoveAll(dir)
		return "", err
	}
	return app, nil
}

// download fetches a to path and checks its hash. A connection that drops
// part way gets two more tries.
func download(ctx context.Context, a Asset, path string, progress func(done, total int64)) error {
	if a.URL == "" {
		return errors.New("the release names no zip")
	}
	if a.SHA256 == "" {
		return errors.New("the release lists no checksum for its zip")
	}
	var err error
	for try := range 3 {
		if try > 0 {
			select {
			case <-ctx.Done():
				return err
			case <-time.After(time.Duration(try) * 2 * time.Second):
			}
		}
		if err = fetch(ctx, a, path, progress); err == nil || ctx.Err() != nil {
			break
		}
	}
	return err
}

func fetch(ctx context.Context, a Asset, path string, progress func(done, total int64)) error {
	req, err := http.NewRequestWithContext(ctx, "GET", a.URL, nil)
	if err != nil {
		return err
	}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: %s", filepath.Base(a.URL), res.Status)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	h := sha256.New()
	var body io.Reader = res.Body
	if progress != nil {
		total := res.ContentLength
		if total <= 0 {
			total = a.Size
		}
		body = &counter{r: res.Body, total: max(total, 0), report: progress}
		progress(0, max(total, 0))
	}
	_, err = io.Copy(io.MultiWriter(f, h), body)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil && !strings.EqualFold(hex.EncodeToString(h.Sum(nil)), a.SHA256) {
		err = fmt.Errorf("%s does not match its checksum", filepath.Base(a.URL))
	}
	if err != nil {
		os.Remove(path)
	}
	return err
}

// counter reports bytes as they are read.
type counter struct {
	r      io.Reader
	done   int64
	total  int64
	report func(done, total int64)
}

func (c *counter) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.done += int64(n)
	c.report(c.done, c.total)
	return n, err
}
