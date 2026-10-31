package app

import (
	"encoding/json"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/donepm/nudged/internal/config"
	"github.com/donepm/nudged/internal/proto"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// loop runs runPanel in a goroutine and returns its channels and the show results.
func loop(t *testing.T, configs chan config.Config, cfg config.Config) (chan []proto.PendingRequest, chan *panelShow) {
	t.Helper()
	changes, shows := make(chan []proto.PendingRequest), make(chan *panelShow, 1)
	done := make(chan struct{})
	go func() {
		runPanel(changes, configs, cfg, func(string) {}, func(s *panelShow, _ int) { shows <- s })
		close(done)
	}()
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

	var dismissed []string
	dismiss := func(id string) { dismissed = append(dismissed, id) }

	req := proto.PendingRequest{ID: "1", Kind: proto.Permission, TTY: "/dev/ttys003", Term: "iTerm.app"}
	focusRequest(&panelShow{Request: req, Mode: "answer"}, "1", dismiss)
	focusRequest(&panelShow{Request: req, Mode: "notify"}, "2", dismiss)
	focusRequest(nil, "1", dismiss)
	if len(calls) != 0 {
		t.Fatalf("jump called: %v", calls)
	}
	focusRequest(&panelShow{Request: req, Mode: "notify"}, "1", dismiss)
	if want := [][2]string{{"/dev/ttys003", "iTerm.app"}}; !slices.Equal(calls, want) || len(dismissed) != 0 {
		t.Fatalf("calls %v, want %v, dismissed %v", calls, want, dismissed)
	}

	req.Kind = proto.Done
	focusRequest(&panelShow{Request: req, Mode: "answer"}, "1", dismiss)
	if len(calls) != 2 || !slices.Equal(dismissed, []string{"1"}) {
		t.Fatalf("notice click: calls %v, dismissed %v", calls, dismissed)
	}
}

func TestRunPanelHide(t *testing.T) {
	old := hideUnit
	hideUnit = time.Millisecond
	t.Cleanup(func() { hideUnit = old })
	calls := make(chan string, 10)
	cfg := config.Default()
	cfg.Events.Done.HideSeconds = 20
	cfg.Events.Waits.HideSeconds = 80
	changes := make(chan []proto.PendingRequest)
	go runPanel(changes, nil, cfg, func(id string) { calls <- id }, func(*panelShow, int) {})
	t.Cleanup(func() { close(changes) })
	none := func(d time.Duration) {
		t.Helper()
		select {
		case id := <-calls:
			t.Fatalf("dismiss %q", id)
		case <-time.After(d):
		}
	}
	after := func(id string, min time.Duration) {
		t.Helper()
		start := time.Now()
		select {
		case got := <-calls:
			if got != id || time.Since(start) < min {
				t.Fatalf("dismiss %q after %v, want %q after %v", got, time.Since(start), id, min)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("no dismiss of %q", id)
		}
	}

	perm, done, waits := proto.PendingRequest{ID: "1", Kind: proto.Permission}, proto.PendingRequest{ID: "2", Kind: proto.Done}, proto.PendingRequest{ID: "3", Kind: proto.Waits}
	changes <- []proto.PendingRequest{perm, done, waits}
	none(100 * time.Millisecond) // behind the permission panel
	changes <- []proto.PendingRequest{done, waits}
	changes <- []proto.PendingRequest{done, waits} // same top: no second timer
	after("2", 15*time.Millisecond)
	none(60 * time.Millisecond)
	changes <- []proto.PendingRequest{waits}
	after("3", 70*time.Millisecond)
	none(100 * time.Millisecond)
}

func TestRunPanelHideConfigChange(t *testing.T) {
	old := hideUnit
	hideUnit = time.Millisecond
	t.Cleanup(func() { hideUnit = old })
	for _, c := range []struct {
		name          string
		first, second int
		want          bool
	}{{"long to short", 100000, 20, true}, {"short to long", 100, 100000, false}, {"to zero", 100, 0, false}} {
		t.Run(c.name, func(t *testing.T) {
			calls := make(chan string, 10)
			cfg := config.Default()
			cfg.Events.Done.HideSeconds = c.first
			changes := make(chan []proto.PendingRequest)
			configs := make(chan config.Config)
			go runPanel(changes, configs, cfg, func(id string) { calls <- id }, func(*panelShow, int) {})
			t.Cleanup(func() { close(changes) })
			changes <- []proto.PendingRequest{{ID: "1", Kind: proto.Done}}
			cfg.Events.Done.HideSeconds = c.second
			configs <- cfg
			select {
			case id := <-calls:
				if !c.want {
					t.Fatalf("dismiss %q", id)
				}
			case <-time.After(300 * time.Millisecond):
				if c.want {
					t.Fatal("no dismiss")
				}
			}
		})
	}
}

func TestRunPanelHideNotShown(t *testing.T) {
	old := hideUnit
	hideUnit = time.Millisecond
	t.Cleanup(func() { hideUnit = old })
	calls := make(chan string, 10)
	cfg := config.Default()
	cfg.Events.Done = config.Event{Show: false, HideSeconds: 1}
	changes := make(chan []proto.PendingRequest)
	go runPanel(changes, nil, cfg, func(id string) { calls <- id }, func(*panelShow, int) {})
	t.Cleanup(func() { close(changes) })
	changes <- []proto.PendingRequest{{ID: "1", Kind: proto.Done}}
	select {
	case id := <-calls:
		t.Fatalf("dismiss %q", id)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestRunPanelHideZero(t *testing.T) {
	old := hideUnit
	hideUnit = time.Millisecond
	t.Cleanup(func() { hideUnit = old })
	calls := make(chan string, 10)
	cfg := config.Default()
	cfg.Events.Done.HideSeconds, cfg.Events.Waits.HideSeconds = 0, 0
	changes := make(chan []proto.PendingRequest)
	go runPanel(changes, nil, cfg, func(id string) { calls <- id }, func(*panelShow, int) {})
	t.Cleanup(func() { close(changes) })
	changes <- []proto.PendingRequest{{ID: "1", Kind: proto.Done}, {ID: "2", Kind: proto.Waits}}
	select {
	case id := <-calls:
		t.Fatalf("dismiss %q", id)
	case <-time.After(100 * time.Millisecond):
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

func TestRunPanelLiveTimeout(t *testing.T) {
	changes, configs, timeouts := make(chan []proto.PendingRequest), make(chan config.Config), make(chan int, 1)
	go runPanel(changes, configs, config.Default(), func(string) {}, func(_ *panelShow, m int) { timeouts <- m })
	t.Cleanup(func() { close(changes) })
	changes <- []proto.PendingRequest{{ID: "1", Kind: proto.Permission}}
	if m := <-timeouts; m != 30 {
		t.Fatalf("timeout %d, want 30", m)
	}
	cfg := config.Default()
	cfg.TimeoutMinutes = 5
	configs <- cfg
	if m := <-timeouts; m != 5 {
		t.Fatalf("timeout %d, want 5", m)
	}
}

func TestAnswerPanel(t *testing.T) {
	payload := json.RawMessage(`{"permission_suggestions":[{"type":"addRules"}]}`)
	pending := []proto.PendingRequest{{ID: "p", Kind: proto.Permission, Payload: payload}}
	allow, _ := decision("allow")
	deny, _ := decision("deny")
	always, _ := permissionDecision("always", 0, payload)
	for _, c := range []struct {
		name string
		a    panelAnswer
		want json.RawMessage
	}{
		{"allow", panelAnswer{ID: "p", Behavior: "allow"}, allow},
		{"deny", panelAnswer{ID: "p", Behavior: "deny"}, deny},
		{"always", panelAnswer{ID: "p", Behavior: "always", Suggestion: 0}, always},
		{"index out of range", panelAnswer{ID: "p", Behavior: "always", Suggestion: 1}, nil},
		{"unknown id", panelAnswer{ID: "x", Behavior: "allow"}, nil},
	} {
		var got []json.RawMessage
		err := answerPanel(pending, c.a, func(_ string, out json.RawMessage) error { got = append(got, out); return nil })
		if c.want == nil {
			if err == nil || len(got) != 0 {
				t.Errorf("%s: err %v, answers %d", c.name, err, len(got))
			}
			continue
		}
		if err != nil || len(got) != 1 || string(got[0]) != string(c.want) {
			t.Errorf("%s: err %v, got %s", c.name, err, got)
		}
	}
}
