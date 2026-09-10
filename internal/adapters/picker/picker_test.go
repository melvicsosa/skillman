package picker

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/melvicsosa/skillman/internal/domain"
)

type call struct {
	name string
	args []string
}

func fakeRunner(out string, err error, got *call) Runner {
	return func(_ context.Context, name string, args ...string) ([]byte, error) {
		*got = call{name: name, args: args}
		return []byte(out), err
	}
}

func TestPickSuccessTrimsPath(t *testing.T) {
	cases := map[string]string{
		"/Users/me/Projects/app/\n": "/Users/me/Projects/app",
		"/Volumes/Data\n":           "/Volumes/Data",
		"/\n":                       "/",
	}
	for out, want := range cases {
		var got call
		path, err := pickWithOsascript(context.Background(), fakeRunner(out, nil, &got), `Pick "one" \ now`)
		if err != nil || path != want {
			t.Fatalf("output %q: got %q, %v; want %q", out, path, err, want)
		}
		wantArgs := []string{
			"-e", "activate",
			"-e", `set f to choose folder with prompt "Pick \"one\" \\ now"`,
			"-e", "POSIX path of f",
		}
		if got.name != "osascript" || !reflect.DeepEqual(got.args, wantArgs) {
			t.Fatalf("command %s %q", got.name, got.args)
		}
	}
}

func TestPickCancelMapsToErrCanceled(t *testing.T) {
	var got call
	runErr := errors.New("osascript: 0:52: execution error: User canceled. (-128): exit status 1")
	_, err := pickWithOsascript(context.Background(), fakeRunner("", runErr, &got), "p")
	if !errors.Is(err, domain.ErrCanceled) {
		t.Fatalf("err = %v, want ErrCanceled", err)
	}
}

func TestPickOtherErrorIsWrapped(t *testing.T) {
	var got call
	runErr := errors.New("osascript: syntax error: exit status 1")
	_, err := pickWithOsascript(context.Background(), fakeRunner("", runErr, &got), "p")
	if err == nil || errors.Is(err, domain.ErrCanceled) || !errors.Is(err, runErr) || !strings.Contains(err.Error(), "choose folder") {
		t.Fatalf("err = %v", err)
	}
}

func TestPickEmptyOutputIsError(t *testing.T) {
	var got call
	if _, err := pickWithOsascript(context.Background(), fakeRunner("\n", nil, &got), "p"); err == nil {
		t.Fatal("expected error for empty output")
	}
}
