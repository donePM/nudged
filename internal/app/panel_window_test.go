package app

import (
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/rokde/nudged/internal/config"
	"github.com/rokde/nudged/internal/proto"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// loop runs runPanel in a goroutine and returns its channels and the show results.
func loop(t *testing.T, configs chan config.Config, cfg config.Config) (chan []proto.PendingRequest, chan *panelShow) {
	t.Helper()
	changes, shows := make(chan []proto.PendingRequest), make(chan *panelShow, 1)
	done := make(chan struct{})
	go func() { runPanel(changes, configs, cfg, func(s *panelShow) { shows <- s }); close(done) }()
	t.Cleanup(func() { close(changes); <-done })
	return changes, shows
}

func next(t *testing.T, shows chan *panelShow) *panelShow {
	t.Helper()
	select {
	case s := <-shows:
		return s
	case <-time.After(2 * time.Second):
		t.Fatal("no show")
	}
	return nil
}

func TestRunPanelSounds(t *testing.T) {
	var mu sync.Mutex
	var got []string
	old := play
	play = func(name string) error { mu.Lock(); got = append(got, name); mu.Unlock(); return nil }
	t.Cleanup(func() { play = old })
	played := func() []string {
		time.Sleep(50 * time.Millisecond)
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(got)
	}

	cfg := config.Default()
	cfg.Events.Permission.Sound = true
	cfg.Events.Question = config.Event{Show: false, Sound: true}
	cfg.Events.Done.Sound = false
	cfg.Events.Waits.Sound = true
	changes, shows := loop(t, nil, cfg)

	changes <- []proto.PendingRequest{
		{ID: "1", Kind: proto.Permission},
		{ID: "2", Kind: proto.Question}, // show off
		{ID: "3", Kind: proto.Done},     // sound off
		{ID: "4", Kind: "other"},        // no config entry
	}
	next(t, shows)
	if p := played(); !slices.Equal(p, []string{"permission"}) {
		t.Fatalf("first snapshot: %v", p)
	}
	changes <- []proto.PendingRequest{{ID: "1", Kind: proto.Permission}, {ID: "5", Kind: proto.Waits}}
	next(t, shows)
	if p := played(); !slices.Equal(p, []string{"permission", "notice"}) {
		t.Fatalf("second snapshot: %v", p)
	}
}

func TestRunPanelMode(t *testing.T) {
	cfg := config.Default()
	cfg.Mode = "notify"
	changes, shows := loop(t, nil, cfg)
	changes <- []proto.PendingRequest{{ID: "1", Kind: proto.Permission}}
	if s := next(t, shows); s == nil || s.Mode != "notify" || s.Request.ID != "1" {
		t.Fatalf("show %+v", s)
	}
}

func TestFocusRequest(t *testing.T) {
	var calls [][2]string
	old := jump
	jump = func(tty, term string) { calls = append(calls, [2]string{tty, term}) }
	t.Cleanup(func() { jump = old })

	req := proto.PendingRequest{ID: "1", TTY: "/dev/ttys003", Term: "iTerm.app"}
	focusRequest(&panelShow{Request: req, Mode: "answer"}, "1")
	focusRequest(&panelShow{Request: req, Mode: "notify"}, "2")
	focusRequest(nil, "1")
	if len(calls) != 0 {
		t.Fatalf("jump called: %v", calls)
	}
	focusRequest(&panelShow{Request: req, Mode: "notify"}, "1")
	if want := [][2]string{{"/dev/ttys003", "iTerm.app"}}; !slices.Equal(calls, want) {
		t.Fatalf("calls %v, want %v", calls, want)
	}
}

func drainConfigs() {
	for len(panelConfigs) > 0 {
		<-panelConfigs
	}
}

func TestSaveSettingsReachesPanel(t *testing.T) {
	srv := testServer(t)
	drainConfigs()
	changes, shows := loop(t, panelConfigs, config.Default())
	changes <- []proto.PendingRequest{{ID: "1", Kind: proto.Permission}, {ID: "2", Kind: proto.Question}}
	if s := next(t, shows); s == nil || s.Request.ID != "1" || s.Mode != "answer" || s.Others != 1 {
		t.Fatalf("first show %+v", s)
	}

	cfg := config.Default()
	cfg.Mode = "notify"
	cfg.Events.Permission.Show = false
	if err := saveSettings(srv, cfg); err != nil {
		t.Fatal(err)
	}
	if s := next(t, shows); s == nil || s.Request.ID != "2" || s.Mode != "notify" || s.Others != 0 {
		t.Fatalf("show after save %+v", s)
	}

	cfg.Events.Question.Show = false
	if err := saveSettings(srv, cfg); err != nil {
		t.Fatal(err)
	}
	if s := next(t, shows); s != nil {
		t.Fatalf("all hidden: show %+v", s)
	}

	cfg.Mode = "loud"
	if err := saveSettings(srv, cfg); err == nil {
		t.Fatal("no error")
	}
	select {
	case s := <-shows:
		t.Fatalf("failed save reached the panel: %+v", s)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestSaveSettingsNoReader(t *testing.T) {
	srv := testServer(t)
	drainConfigs()
	t.Cleanup(drainConfigs)
	done := make(chan struct{})
	cfg := config.Default()
	go func() {
		saveSettings(srv, cfg)
		cfg.Mode = "notify"
		saveSettings(srv, cfg)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("save blocks")
	}
	if got := <-panelConfigs; got.Mode != "notify" || len(panelConfigs) != 0 {
		t.Fatalf("got %+v, %d more", got, len(panelConfigs))
	}
}

func TestPanelOptions(t *testing.T) {
	o := panelOptions("Generic")
	if o.Mac.WindowClass != application.MacWindowClassPanel {
		t.Fatalf("window class %v", o.Mac.WindowClass)
	}
	if p := o.Mac.PanelPreferences; !p.NonActivating || !p.FloatingPanel || !p.BecomesKeyOnlyIfNeeded {
		t.Fatalf("panel preferences %+v", p)
	}
	if o.Name != panelName || o.URL != "/#/Generic" || o.Width != 380 || o.Height != 220 ||
		!o.Frameless || !o.AlwaysOnTop || !o.DisableResize || !o.Hidden {
		t.Fatalf("options %+v", o)
	}
}
