package app

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/donepm/nudged/internal/config"
	"github.com/donepm/nudged/internal/update"
)

func fakeUpdater(cur, latest string, installErr error) (*updater, *[]updateInfo) {
	var seen []updateInfo
	u := &updater{
		info:    updateInfo{Version: cur},
		latest:  func(context.Context) (update.Release, error) { return update.Release{Version: latest, URL: "u"}, nil },
		install: func(context.Context) error { return installErr },
		changed: func(i updateInfo) { seen = append(seen, i) },
	}
	return u, &seen
}

func TestUpdaterCheck(t *testing.T) {
	u, seen := fakeUpdater("0.1.4", "0.1.5", nil)
	i := u.check(context.Background())
	if !i.Update || i.Latest != "0.1.5" || i.Checking || i.Error != "" {
		t.Fatalf("newer: %+v", i)
	}
	if len(*seen) != 2 || !(*seen)[0].Checking {
		t.Fatalf("changes %+v", *seen)
	}
	u, _ = fakeUpdater("0.1.5", "0.1.5", nil)
	if i := u.check(context.Background()); i.Update {
		t.Fatalf("same: %+v", i)
	}
	u.latest = func(context.Context) (update.Release, error) { return update.Release{}, errors.New("offline") }
	if i := u.check(context.Background()); i.Error != "offline" || i.Checking {
		t.Fatalf("error: %+v", i)
	}
}

func TestUpdaterApply(t *testing.T) {
	u, _ := fakeUpdater("0.1.4", "0.1.5", nil)
	if u.apply(context.Background()) {
		t.Fatal("apply before check: true")
	}
	u.check(context.Background())
	if !u.apply(context.Background()) {
		t.Fatalf("apply: false, %+v", u.state())
	}
	u, _ = fakeUpdater("0.1.4", "0.1.5", errors.New("brew failed"))
	u.check(context.Background())
	if u.apply(context.Background()) || u.state().Error != "brew failed" || u.state().Checking {
		t.Fatalf("failed install: %+v", u.state())
	}
}

func TestRunUpdateChecks(t *testing.T) {
	old := Version
	t.Cleanup(func() { Version = old })
	var calls atomic.Int32
	u, _ := fakeUpdater("0.1.4", "0.1.5", nil)
	u.latest = func(context.Context) (update.Release, error) {
		calls.Add(1)
		return update.Release{Version: "0.1.5"}, nil
	}
	run := func(on bool) {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		load := func() (config.Config, error) { c := config.Default(); c.CheckUpdates = on; return c, nil }
		runUpdateChecks(ctx, u, load, time.Millisecond)
	}
	Version = "dev"
	run(true)
	Version = "0.1.4"
	run(false)
	if n := calls.Load(); n != 0 {
		t.Fatalf("dev build or setting off: %d checks", n)
	}
	run(true)
	if n := calls.Load(); n != 1 {
		t.Fatalf("on: %d checks", n)
	}
}

func TestAppBundle(t *testing.T) {
	for bin, want := range map[string]string{
		"/Applications/nudged.app/Contents/MacOS/nudged": "/Applications/nudged.app",
		"/Users/x/go/bin/nudged":                         "",
		"/tmp/Contents/MacOS/nudged":                     "",
	} {
		if got := appBundle(bin); got != want {
			t.Errorf("%s: %q", bin, got)
		}
	}
}

func TestAboutText(t *testing.T) {
	old := Version
	t.Cleanup(func() { Version = old })
	Version = "1.2.3"
	if got := aboutText(); got != "Version 1.2.3\nTray app and hooks for Claude Code.\nnudged is a service by donePM." {
		t.Fatalf("%q", got)
	}
	if got := updateLabel(updateInfo{Latest: "1.2.4"}); got != "Update to 1.2.4…" {
		t.Fatalf("%q", got)
	}
}
