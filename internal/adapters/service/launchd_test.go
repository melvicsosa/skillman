package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/melvicsosa/skillman/internal/domain"
)

// fakeLaunchctl records every invocation and answers from a script.
type fakeLaunchctl struct {
	calls  []string
	loaded bool
	pid    int
	fail   map[string]bool // verb -> fail
}

func (f *fakeLaunchctl) run(_ context.Context, args ...string) (string, error) {
	f.calls = append(f.calls, strings.Join(args, " "))
	verb := args[0]
	if f.fail[verb] {
		return "", errors.New("launchctl " + verb + ": failed")
	}
	switch verb {
	case "print":
		if !f.loaded {
			return "", errors.New("Could not find service")
		}
		if f.pid > 0 {
			return "com.melvicsosa.skillman = {\n\tactive count = 1\n\tpath = /x\n\tstate = running\n\n\tpid = " + itoa(f.pid) + "\n}\n", nil
		}
		return "com.melvicsosa.skillman = {\n\tstate = waiting\n}\n", nil
	case "bootstrap", "load":
		f.loaded = true
		f.pid = 4242
	case "bootout", "unload":
		f.loaded = false
		f.pid = 0
	case "kickstart":
		f.pid++
	}
	return "", nil
}

func itoa(i int) string { return strconv.Itoa(i) }

func newTestLaunchd(t *testing.T) (*Launchd, *fakeLaunchctl) {
	t.Helper()
	f := &fakeLaunchctl{fail: map[string]bool{}}
	l := &Launchd{Label: domain.ServiceLabel, PlistDir: filepath.Join(t.TempDir(), "LaunchAgents"), DataDir: filepath.Join(t.TempDir(), "data"), UID: 501, Run: f.run}
	return l, f
}

func TestLaunchdInstallWritesPlistAndBootstraps(t *testing.T) {
	l, f := newTestLaunchd(t)
	ctx := context.Background()
	spec := domain.ServiceSpec{Executable: "/usr/local/bin/skillman", Args: []string{"serve", "--no-open"}, Env: map[string]string{"PATH": "/usr/bin:/bin", "A&B": "<x>"}}
	if err := l.Install(ctx, spec); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(l.PlistPath())
	if err != nil {
		t.Fatal(err)
	}
	plist := string(raw)
	for _, want := range []string{
		"<string>com.melvicsosa.skillman</string>",
		"<string>/usr/local/bin/skillman</string>",
		"<string>serve</string>", "<string>--no-open</string>",
		"<key>RunAtLoad</key>\n\t<true/>",
		"<key>SuccessfulExit</key>\n\t\t<false/>",
		"<string>" + filepath.Join(l.DataDir, "logs", "skillman.log") + "</string>",
		"<key>A&amp;B</key>\n\t\t<string>&lt;x&gt;</string>",
	} {
		if !strings.Contains(plist, want) {
			t.Errorf("plist missing %q:\n%s", want, plist)
		}
	}
	if _, err := os.Stat(l.LogDir()); err != nil {
		t.Fatal("log dir not created")
	}
	last := f.calls[len(f.calls)-1]
	if last != "bootstrap gui/501 "+l.PlistPath() {
		t.Fatalf("last call = %q", last)
	}

	st, err := l.Status(ctx)
	if err != nil || !st.Installed || !st.Running || st.PID != 4242 || !st.Supported {
		t.Fatalf("status after install = %+v, %v", st, err)
	}

	// Reinstall boots out first so the new plist is picked up.
	f.calls = nil
	if err := l.Install(ctx, spec); err != nil {
		t.Fatal(err)
	}
	if !contains(f.calls, "bootout gui/501/com.melvicsosa.skillman") {
		t.Fatalf("reinstall did not bootout: %v", f.calls)
	}
}

func TestLaunchdInstallFallsBackToLoad(t *testing.T) {
	l, f := newTestLaunchd(t)
	f.fail["bootstrap"] = true
	if err := l.Install(context.Background(), domain.ServiceSpec{Executable: "/bin/skillman"}); err != nil {
		t.Fatal(err)
	}
	if !contains(f.calls, "load -w "+l.PlistPath()) {
		t.Fatalf("no load fallback: %v", f.calls)
	}
}

func TestLaunchdStopStartRestartUninstall(t *testing.T) {
	l, f := newTestLaunchd(t)
	ctx := context.Background()

	if err := l.Start(ctx); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("start before install: %v", err)
	}
	if err := l.Restart(ctx); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("restart before install: %v", err)
	}
	if err := l.Install(ctx, domain.ServiceSpec{Executable: "/bin/skillman"}); err != nil {
		t.Fatal(err)
	}
	if err := l.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	if st, _ := l.Status(ctx); st.Running || !st.Installed {
		t.Fatalf("after stop = %+v", st)
	}
	if err := l.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if st, _ := l.Status(ctx); !st.Running {
		t.Fatalf("after start = %+v", st)
	}
	f.calls = nil
	if err := l.Restart(ctx); err != nil {
		t.Fatal(err)
	}
	if !contains(f.calls, "kickstart -k gui/501/com.melvicsosa.skillman") {
		t.Fatalf("restart calls = %v", f.calls)
	}
	if err := l.Uninstall(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(l.PlistPath()); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("plist still present after uninstall")
	}
	if st, _ := l.Status(ctx); st.Installed || st.Running {
		t.Fatalf("after uninstall = %+v", st)
	}
	// Uninstalling twice is a no-op.
	if err := l.Uninstall(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestParsePrint(t *testing.T) {
	if r, pid := parsePrint("x = {\n\tstate = running\n\tpid = 77\n}"); !r || pid != 77 {
		t.Fatalf("got %v %d", r, pid)
	}
	if r, pid := parsePrint("x = {\n\tstate = waiting\n}"); r || pid != 0 {
		t.Fatalf("got %v %d", r, pid)
	}
}

func TestUnsupported(t *testing.T) {
	u := Unsupported{Platform: "plan9"}
	if err := u.Install(context.Background(), domain.ServiceSpec{}); !errors.Is(err, domain.ErrUnsupported) {
		t.Fatal(err)
	}
	st, err := u.Status(context.Background())
	if err != nil || st.Supported || st.Platform != "plan9" {
		t.Fatalf("%+v %v", st, err)
	}
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
