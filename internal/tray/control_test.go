//go:build !windows

package tray

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// fakeSkillman writes a shell script standing in for the skillman binary.
// It records its arguments to <dir>/calls and answers `service status --json`
// with statusJSON; `serve` writes the pid file and sleeps.
func fakeSkillman(t *testing.T, dataDir, statusJSON string) string {
	t.Helper()
	dir := t.TempDir()
	script := `#!/bin/sh
echo "$@" >> "` + dir + `/calls"
case "$*" in
  *"service status --json"*) printf '%s\n' '` + statusJSON + `' ;;
  *" serve "*|*" serve") echo $$ > "` + dataDir + `/serve.pid"; trap 'rm -f "` + dataDir + `/serve.pid"; exit 0' TERM; while :; do sleep 0.1; done ;;
esac
`
	p := filepath.Join(dir, "skillman")
	if err := os.WriteFile(p, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func calls(t *testing.T, bin string) []string {
	t.Helper()
	raw, _ := os.ReadFile(filepath.Join(filepath.Dir(bin), "calls"))
	return strings.Split(strings.TrimSpace(string(raw)), "\n")
}

func TestStatusLabel(t *testing.T) {
	cases := []struct {
		st   Status
		want string
	}{
		{Status{Healthy: true, Port: 3010, Version: "0.3.0"}, "Running on :3010 · v0.3.0"},
		{Status{Healthy: true, Port: 3011, Version: "v1.0.0"}, "Running on :3011 · v1.0.0"},
		{Status{Healthy: true, Port: 3010}, "Running on :3010"},
		{Status{Installed: true}, "Stopped"},
		{Status{}, "Service not installed"},
	}
	for _, c := range cases {
		if got := c.st.Label(); got != c.want {
			t.Errorf("%+v: got %q want %q", c.st, got, c.want)
		}
	}
}

func TestStatusReadsCLIAndHealth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/health" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(`{"status":"ok","version":"0.3.0","pid":42}`))
	}))
	defer srv.Close()
	port, _ := strconv.Atoi(strings.TrimPrefix(srv.URL, "http://127.0.0.1:"))
	data := t.TempDir()
	bin := fakeSkillman(t, data, `{"supported":true,"installed":true,"running":true,"port":`+strconv.Itoa(port)+`,"healthy":true,"serverPid":42}`)
	c := NewController(Config{DataDir: data, Skillman: bin})
	st, err := c.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !st.Installed || !st.Healthy || st.Port != port || st.Version != "0.3.0" {
		t.Fatalf("unexpected status %+v", st)
	}
	if got := calls(t, bin)[0]; got != "--data-dir "+data+" service status --json" {
		t.Errorf("call: %q", got)
	}
}

func TestServiceActionsDelegateToCLI(t *testing.T) {
	data := t.TempDir()
	bin := fakeSkillman(t, data, `{}`)
	c := NewController(Config{DataDir: data, Skillman: bin})
	ctx := context.Background()
	installed := Status{Installed: true, Healthy: true}
	for _, step := range []func() error{
		func() error { return c.Start(ctx, installed) },
		func() error { return c.Stop(ctx, installed) },
		func() error { return c.Restart(ctx, installed) },
		func() error { return c.Install(ctx) },
		func() error { return c.Uninstall(ctx) },
		func() error { return c.SetPort(ctx, installed, 4010) },
	} {
		if err := step(); err != nil {
			t.Fatal(err)
		}
	}
	want := []string{"service start", "service stop", "service restart", "service install", "service uninstall", "config set port 4010", "service restart"}
	got := calls(t, bin)
	if len(got) != len(want) {
		t.Fatalf("calls: %v", got)
	}
	for i := range want {
		if !strings.HasSuffix(got[i], want[i]) {
			t.Errorf("call %d: %q want suffix %q", i, got[i], want[i])
		}
	}
}

func TestDetachedServeStartStop(t *testing.T) {
	data := t.TempDir()
	bin := fakeSkillman(t, data, `{}`)
	c := NewController(Config{DataDir: data, Skillman: bin})
	ctx := context.Background()
	if err := c.Start(ctx, Status{}); err != nil {
		t.Fatal(err)
	}
	var pid int
	for i := 0; i < 50 && pid == 0; i++ {
		time.Sleep(50 * time.Millisecond)
		pid = readPIDFile(c.PIDFile())
	}
	if pid == 0 {
		t.Fatal("detached serve did not write the pid file")
	}
	if _, err := os.Stat(c.LogFile()); err != nil {
		t.Fatalf("log file: %v", err)
	}
	if err := c.Stop(ctx, Status{}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		if readPIDFile(c.PIDFile()) == 0 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("pid file still present after Stop")
}

func TestStopWithoutServerIsNoop(t *testing.T) {
	c := NewController(Config{DataDir: t.TempDir(), Skillman: "/nonexistent"})
	if err := c.Stop(context.Background(), Status{}); err != nil {
		t.Fatal(err)
	}
}
