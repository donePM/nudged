// Package update finds the newest nudged release on GitHub and installs it with Homebrew.
package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// LatestURL is the GitHub API page of the newest release. It has no drafts and no prereleases.
const LatestURL = "https://api.github.com/repos/donePM/nudged/releases/latest"

// Cask is the Homebrew cask that installs nudged.
const Cask = "donepm/tap/nudged"

type Release struct {
	Version string `json:"version"` // without the leading "v"
	URL     string `json:"url"`     // the release page
}

// Latest gets the newest release from url.
func Latest(ctx context.Context, c *http.Client, url string) (Release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := c.Do(req)
	if err != nil {
		return Release{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Release{}, fmt.Errorf("release check: %s", resp.Status)
	}
	var r struct {
		Tag string `json:"tag_name"`
		URL string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return Release{}, fmt.Errorf("release check: %w", err)
	}
	v := strings.TrimPrefix(r.Tag, "v")
	if _, ok := parse(v); !ok {
		return Release{}, fmt.Errorf("release check: tag %q is no version", r.Tag)
	}
	return Release{Version: v, URL: r.URL}, nil
}

// parse reads "X.Y.Z" with an optional leading "v". A suffix like "-rc1" is not a version here.
func parse(v string) ([3]int, bool) {
	var out [3]int
	parts := strings.Split(strings.TrimPrefix(v, "v"), ".")
	if len(parts) != 3 {
		return out, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return out, false
		}
		out[i] = n
	}
	return out, true
}

// Newer reports whether latest is a higher version than cur. A dev build (cur is no version) is never older.
func Newer(cur, latest string) bool {
	c, ok1 := parse(cur)
	l, ok2 := parse(latest)
	if !ok1 || !ok2 {
		return false
	}
	for i := range c {
		if l[i] != c[i] {
			return l[i] > c[i]
		}
	}
	return false
}

// brewPaths are the Homebrew binaries on Apple silicon and on Intel. A GUI app has no shell PATH.
var brewPaths = []string{"/opt/homebrew/bin/brew", "/usr/local/bin/brew"}

// ErrNoBrew means Homebrew is not installed.
var ErrNoBrew = errors.New("Homebrew not found")

// Brew returns the path of the brew binary.
func Brew() (string, error) {
	for _, p := range brewPaths {
		if fi, err := os.Stat(p); err == nil && fi.Mode()&0o111 != 0 {
			return p, nil
		}
	}
	return "", ErrNoBrew
}

// Install updates the tap and upgrades the cask with brew. On error, the message has the end of the brew output.
func Install(ctx context.Context, brew string) error {
	for _, args := range [][]string{{"update", "--quiet"}, {"upgrade", "--cask", Cask}} {
		cmd := exec.CommandContext(ctx, brew, args...)
		cmd.Env = append(os.Environ(), "HOMEBREW_NO_AUTO_UPDATE=1", "HOMEBREW_NO_ENV_HINTS=1",
			"PATH=/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin")
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("brew %s: %w: %s", strings.Join(args, " "), err, tail(string(out), 300))
		}
	}
	return nil
}

func tail(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		s = "…" + s[len(s)-n:]
	}
	return s
}
