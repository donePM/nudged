package loginitem

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

type doc struct {
	Dict struct {
		Keys    []string  `xml:"key"`
		Strings []string  `xml:"string"`
		Args    []string  `xml:"array>string"`
		True    *struct{} `xml:"true"`
	} `xml:"dict"`
}

func readPlist(t *testing.T) doc {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(os.Getenv("HOME"), "Library", "LaunchAgents", "de.donepm.nudged.plist"))
	if err != nil {
		t.Fatal(err)
	}
	var d doc
	if err := xml.Unmarshal(data, &d); err != nil {
		t.Fatalf("%v: %s", err, data)
	}
	return d
}

func TestEnable(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if Enabled() {
		t.Fatal("enabled before Enable")
	}
	if err := Enable("/x/nudged"); err != nil {
		t.Fatal(err)
	}
	d := readPlist(t)
	if want := []string{"Label", "ProgramArguments", "RunAtLoad"}; !slices.Equal(d.Dict.Keys, want) {
		t.Errorf("keys %v", d.Dict.Keys)
	}
	if !slices.Equal(d.Dict.Strings, []string{"de.donepm.nudged"}) {
		t.Errorf("label %v", d.Dict.Strings)
	}
	if !slices.Equal(d.Dict.Args, []string{"/x/nudged"}) {
		t.Errorf("args %v", d.Dict.Args)
	}
	if d.Dict.True == nil {
		t.Error("RunAtLoad is not true")
	}
	if !Enabled() {
		t.Fatal("not enabled after Enable")
	}
}

func TestEnableEscapes(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	bin := "/a&b/<c>/nudged"
	if err := Enable(bin); err != nil {
		t.Fatal(err)
	}
	if d := readPlist(t); !slices.Equal(d.Dict.Args, []string{bin}) {
		t.Fatalf("args %v", d.Dict.Args)
	}
}

func TestDisable(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := Enable("/x/nudged"); err != nil {
		t.Fatal(err)
	}
	if err := Disable(); err != nil {
		t.Fatal(err)
	}
	if Enabled() {
		t.Fatal("enabled after Disable")
	}
	if err := Disable(); err != nil {
		t.Fatalf("second Disable: %v", err)
	}
}
