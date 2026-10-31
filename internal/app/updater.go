package app

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/donepm/nudged/internal/config"
	"github.com/donepm/nudged/internal/update"
)

// Version is the app version. main sets it from the build flags.
var Version = "dev"

// updateInfo is the update state the settings view shows.
type updateInfo struct {
	Version  string `json:"version"`
	Latest   string `json:"latest,omitempty"`
	URL      string `json:"url,omitempty"`
	Update   bool   `json:"update"`   // Latest is newer than Version
	Checking bool   `json:"checking"` // a check or an install runs
	Error    string `json:"error,omitempty"`
}

// updater checks for releases and installs them. Its methods are safe for concurrent use.
type updater struct {
	mu      sync.Mutex
	info    updateInfo
	latest  func(context.Context) (update.Release, error)
	install func(context.Context) error
	changed func(updateInfo) // called after each state change, outside the lock
}

func newUpdater(changed func(updateInfo)) *updater {
	client := &http.Client{Timeout: 15 * time.Second}
	return &updater{
		info:   updateInfo{Version: Version},
		latest: func(ctx context.Context) (update.Release, error) { return update.Latest(ctx, client, update.LatestURL) },
		install: func(ctx context.Context) error {
			brew, err := update.Brew()
			if err != nil {
				return err
			}
			return update.Install(ctx, brew)
		},
		changed: changed,
	}
}

func (u *updater) state() updateInfo {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.info
}

func (u *updater) set(f func(*updateInfo)) updateInfo {
	u.mu.Lock()
	f(&u.info)
	info := u.info
	u.mu.Unlock()
	if u.changed != nil {
		u.changed(info)
	}
	return info
}

// check asks GitHub for the newest release. It does nothing while another check or an install runs.
func (u *updater) check(ctx context.Context) updateInfo {
	busy := false
	u.set(func(i *updateInfo) { busy = i.Checking; i.Checking = true })
	if busy {
		return u.state()
	}
	r, err := u.latest(ctx)
	return u.set(func(i *updateInfo) {
		i.Checking, i.Error = false, ""
		if err != nil {
			i.Error = err.Error()
			return
		}
		i.Latest, i.URL, i.Update = r.Version, r.URL, update.Newer(i.Version, r.Version)
	})
}

// apply installs the newest release. It returns false when there is no update or the install failed.
func (u *updater) apply(ctx context.Context) bool {
	ok := false
	u.set(func(i *updateInfo) {
		ok = i.Update && !i.Checking
		if ok {
			i.Checking, i.Error = true, ""
		}
	})
	if !ok {
		return false
	}
	err := u.install(ctx)
	u.set(func(i *updateInfo) {
		i.Checking = false
		if err != nil {
			i.Error = err.Error()
		}
	})
	return err == nil
}

// updateEvery is the time between two automatic checks.
const updateEvery = 24 * time.Hour

// runUpdateChecks checks after a short wait and then once per updateEvery, while the config has CheckUpdates on.
// A dev build never checks by itself.
func runUpdateChecks(ctx context.Context, u *updater, load func() (config.Config, error), first time.Duration) {
	if !update.Newer("0.0.0", Version) {
		return
	}
	t := time.NewTimer(first)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		if cfg, _ := load(); cfg.CheckUpdates {
			u.check(ctx)
		}
		t.Reset(updateEvery)
	}
}

// appBundle returns the .app folder of the binary bin, or "" when bin is not in a bundle.
func appBundle(bin string) string {
	dir := filepath.Dir(bin) // .../nudged.app/Contents/MacOS
	if filepath.Base(dir) != "MacOS" || filepath.Base(filepath.Dir(dir)) != "Contents" {
		return ""
	}
	app := filepath.Dir(filepath.Dir(dir))
	if !strings.HasSuffix(app, ".app") {
		return ""
	}
	return app
}

// relaunch starts the app bundle again after this process quit. The socket allows only one running app.
var relaunch = func() error {
	bin, err := binPath()
	if err != nil {
		return err
	}
	app := appBundle(bin)
	if app == "" {
		return fmt.Errorf("%s is not in an app bundle", bin)
	}
	cmd := exec.Command("/bin/sh", "-c", `sleep 2; /usr/bin/open "$0"`, app)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return cmd.Start()
}
