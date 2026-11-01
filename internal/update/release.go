package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// feedURL is where the newest release is described. release.sh can bake
// another in with -ldflags "-X …/update.feedURL=…"; PULSEWISE_UPDATE_FEED
// points it elsewhere at run time, for trying an update against a local
// server.
var feedURL = "https://github.com/FriendlyPasser/pulsewise/releases/latest/download/latest.json"

// Feed is the feed's URL.
func Feed() string {
	if f := os.Getenv("PULSEWISE_UPDATE_FEED"); f != "" {
		return f
	}
	return feedURL
}

// Release is one published version, as latest.json describes it.
type Release struct {
	Version string `json:"version"` // "0.2.0", no v
	Notes   string `json:"notes"`
	URL     string `json:"url"` // the release page
	Zip     Asset  `json:"zip"`
}

// Asset is a downloadable file of a release.
type Asset struct {
	URL    string `json:"url"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

var client = &http.Client{Timeout: 10 * time.Minute}

// Latest asks the feed for the newest release.
func Latest(ctx context.Context) (*Release, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", Feed(), nil)
	if err != nil {
		return nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("update feed: %s", res.Status)
	}
	var r Release
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&r); err != nil {
		return nil, fmt.Errorf("update feed: %w", err)
	}
	if parse(r.Version) == nil {
		return nil, fmt.Errorf("update feed: bad version %q", r.Version)
	}
	return &r, nil
}

// Newer reports whether version a comes after b. Versions are release.sh's:
// one to three numbers, a leading v allowed.
func Newer(a, b string) bool {
	x, y := parse(a), parse(b)
	if x == nil || y == nil {
		return false
	}
	for i := range x {
		if x[i] != y[i] {
			return x[i] > y[i]
		}
	}
	return false
}

func parse(v string) []int {
	parts := strings.Split(strings.TrimPrefix(strings.TrimSpace(v), "v"), ".")
	if len(parts) == 0 || len(parts) > 3 {
		return nil
	}
	n := make([]int, 3)
	for i, p := range parts {
		d, err := strconv.Atoi(p)
		if err != nil || d < 0 {
			return nil
		}
		n[i] = d
	}
	return n
}
