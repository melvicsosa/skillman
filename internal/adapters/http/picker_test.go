package http

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/melvicsosa/skillman/internal/app"
	"github.com/melvicsosa/skillman/internal/domain"
)

type fakePicker struct {
	path  string
	err   error
	calls int
}

func (f *fakePicker) PickFolder(context.Context, string) (string, error) {
	f.calls++
	return f.path, f.err
}

func newPickerServer(t *testing.T, p domain.FolderPicker) *httptest.Server {
	t.Helper()
	h, err := NewHandler(app.New(app.Deps{Picker: p}))
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}

func postPick(t *testing.T, srv *httptest.Server, contentType string) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest("POST", srv.URL+"/api/fs/pick-folder", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b
}

func TestPickFolderReturnsPath(t *testing.T) {
	fp := &fakePicker{path: "/Users/me/code/app"}
	status, body := postPick(t, newPickerServer(t, fp), "application/json; charset=utf-8")
	var got map[string]string
	if status != 200 || json.Unmarshal(body, &got) != nil || got["path"] != "/Users/me/code/app" {
		t.Fatalf("status %d body %s", status, body)
	}
}

func TestPickFolderCancelIsNoContent(t *testing.T) {
	fp := &fakePicker{err: domain.ErrCanceled}
	status, body := postPick(t, newPickerServer(t, fp), "application/json")
	if status != 204 || len(body) != 0 {
		t.Fatalf("status %d body %s", status, body)
	}
}

func TestPickFolderRejectsNonJSONContentType(t *testing.T) {
	for _, ct := range []string{"", "text/plain", "application/x-www-form-urlencoded", "multipart/form-data; boundary=x"} {
		fp := &fakePicker{path: "/x"}
		status, body := postPick(t, newPickerServer(t, fp), ct)
		var e map[string]string
		if status != 415 || json.Unmarshal(body, &e) != nil || e["error"] == "" {
			t.Fatalf("%q: status %d body %s", ct, status, body)
		}
		if fp.calls != 0 {
			t.Fatalf("%q: picker must not run", ct)
		}
	}
}

func TestPickFolderUnsupported(t *testing.T) {
	for name, p := range map[string]domain.FolderPicker{
		"adapter": &fakePicker{err: domain.ErrUnsupported},
		"nil":     nil,
	} {
		status, body := postPick(t, newPickerServer(t, p), "application/json")
		var e map[string]string
		if status != 501 || json.Unmarshal(body, &e) != nil || e["error"] == "" {
			t.Fatalf("%s: status %d body %s", name, status, body)
		}
	}
}
