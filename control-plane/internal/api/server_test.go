package api

// Handler tests that need no database: auth middleware, /xml guard and the
// validation paths that reject a request before any store call. Paths that
// reach PostgreSQL are covered by deploy/api-test.sh against the live stack.

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jamalshahverdiev/freeswitch-iac-platform/control-plane/internal/audit"
	"github.com/jamalshahverdiev/freeswitch-iac-platform/control-plane/internal/runtime"
)

func testServer(t *testing.T, opts Options) http.Handler {
	t.Helper()
	if opts.Token == "" {
		opts.Token = "test-token"
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	// nil store/pool: only routes that fail before touching the DB are tested.
	esl := runtime.New("", "", time.Second) // Enabled() == false
	return NewServer(nil, audit.New(nil), esl, opts, log).Router()
}

func do(t *testing.T, h http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, rdr)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestAuth(t *testing.T) {
	h := testServer(t, Options{})

	cases := []struct {
		name  string
		token string
		want  int
	}{
		{"no token", "", http.StatusUnauthorized},
		{"wrong token", "nope", http.StatusUnauthorized},
		// correct token passes auth; ESL disabled -> 503 from the handler,
		// which proves the middleware let the request through.
		{"valid token", "test-token", http.StatusServiceUnavailable},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := do(t, h, http.MethodGet, "/api/v1/runtime/health", c.token, "")
			if c.name == "valid token" {
				// runtime/health reports ESL state in the body with 200/503
				if rec.Code != http.StatusOK && rec.Code != http.StatusServiceUnavailable {
					t.Fatalf("got %d", rec.Code)
				}
				return
			}
			if rec.Code != c.want {
				t.Fatalf("got %d want %d", rec.Code, c.want)
			}
		})
	}
}

func TestHealthzIsPublic(t *testing.T) {
	h := testServer(t, Options{})
	if rec := do(t, h, http.MethodGet, "/healthz", "", ""); rec.Code != http.StatusOK {
		t.Fatalf("healthz must not require auth, got %d", rec.Code)
	}
}

func TestXMLGuardRequiresBasicAuth(t *testing.T) {
	h := testServer(t, Options{XMLUser: "fs", XMLPassword: "secret"})

	req := httptest.NewRequest(http.MethodPost, "/xml/directory", strings.NewReader("user=1&domain=d"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no basic auth: got %d want 401", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/xml/directory", strings.NewReader("user=1&domain=d"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("fs", "wrong")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong basic auth: got %d want 401", rec.Code)
	}
}

func TestValidationRejectsBeforeStore(t *testing.T) {
	h := testServer(t, Options{})
	tok := "test-token"

	cases := []struct {
		name, method, path, body string
	}{
		{"domain without name", http.MethodPost, "/api/v1/domains", `{"description":"x"}`},
		{"bad json", http.MethodPost, "/api/v1/domains", `{not-json`},
		{"cc queue without name", http.MethodPost, "/api/v1/callcenter/queues", `{}`},
		{"cc agent without contact", http.MethodPost, "/api/v1/callcenter/agents", `{"name":"a@d"}`},
		{"cc tier without agent", http.MethodPost, "/api/v1/callcenter/tiers", `{"queue":"q@d"}`},
		{"conf profile without name", http.MethodPost, "/api/v1/conference/profiles", `{"video_mode":"mux"}`},
		{"conf room missing profile", http.MethodPost, "/api/v1/conference/rooms", `{"name":"r","number":"1","domain":"d","context":"c"}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := do(t, h, c.method, c.path, tok, c.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("got %d want 400 (body: %s)", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestCDRParseBadPayload(t *testing.T) {
	// POST /cdr is behind xmlGuard; with no guard configured it is open in
	// tests, so we can exercise the parser. A non-cdr body must 400 (dropped,
	// not retried), and a payload with no uuid must 400.
	h := testServer(t, Options{})
	for _, body := range []string{`{"not":"a cdr"}`, `{"variables":{}}`, `not json`} {
		req := httptest.NewRequest(http.MethodPost, "/cdr", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("body %q: got %d want 400", body, rec.Code)
		}
	}
}

func TestCDRListBadPagination(t *testing.T) {
	h := testServer(t, Options{})
	rec := do(t, h, http.MethodGet, "/api/v1/cdr?limit=bad", "test-token", "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got %d want 400", rec.Code)
	}
}

func TestRuntimeRequiresESL(t *testing.T) {
	h := testServer(t, Options{})
	tok := "test-token"

	paths := []struct{ method, path, body string }{
		{http.MethodPost, "/api/v1/runtime/reloadxml", ""},
		{http.MethodPost, "/api/v1/runtime/callcenter/reload", ""},
		{http.MethodGet, "/api/v1/runtime/conference/standup", ""},
		{http.MethodPut, "/api/v1/runtime/callcenter/agents/a@d/status", `{"status":"Available"}`},
	}
	for _, p := range paths {
		t.Run(p.path, func(t *testing.T) {
			rec := do(t, h, p.method, p.path, tok, p.body)
			if rec.Code != http.StatusServiceUnavailable {
				t.Fatalf("got %d want 503 when ESL is not configured", rec.Code)
			}
		})
	}
}

func TestPaginationHelper(t *testing.T) {
	items := []int{0, 1, 2, 3, 4}
	cases := []struct {
		limit, offset int
		want          []int
		wantTotal     int
	}{
		{0, 0, []int{0, 1, 2, 3, 4}, 5}, // no limit = all (backward compat)
		{2, 0, []int{0, 1}, 5},
		{2, 3, []int{3, 4}, 5},
		{10, 0, []int{0, 1, 2, 3, 4}, 5}, // limit > len
		{2, 99, []int{}, 5},              // offset past end
	}
	for _, c := range cases {
		out, total := apply(items, page{limit: c.limit, offset: c.offset})
		if total != c.wantTotal {
			t.Errorf("limit=%d offset=%d total=%d want %d", c.limit, c.offset, total, c.wantTotal)
		}
		if len(out) != len(c.want) {
			t.Errorf("limit=%d offset=%d got %v want %v", c.limit, c.offset, out, c.want)
			continue
		}
		for i := range out {
			if out[i] != c.want[i] {
				t.Errorf("limit=%d offset=%d got %v want %v", c.limit, c.offset, out, c.want)
				break
			}
		}
	}
}

func TestPaginationBadParams(t *testing.T) {
	h := testServer(t, Options{})
	for _, q := range []string{"limit=abc", "limit=-1", "offset=-5", "offset=x"} {
		rec := do(t, h, http.MethodGet, "/api/v1/domains?"+q, "test-token", "")
		// nil store would panic only AFTER parsePage; a 400 proves we stopped first.
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: got %d want 400", q, rec.Code)
		}
	}
}

func TestAuditBadLimit(t *testing.T) {
	h := testServer(t, Options{})
	rec := do(t, h, http.MethodGet, "/api/v1/audit?limit=nope", "test-token", "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got %d want 400", rec.Code)
	}
}

func TestRecordings(t *testing.T) {
	t.Run("503 when not configured", func(t *testing.T) {
		h := testServer(t, Options{})
		rec := do(t, h, http.MethodGet, "/api/v1/recordings", "test-token", "")
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("got %d want 503", rec.Code)
		}
	})

	t.Run("bad date and traversal-looking names rejected", func(t *testing.T) {
		// Backend stub proves the request never reaches it.
		backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Errorf("backend must not be called, got %s", r.URL.Path)
		}))
		defer backend.Close()
		h := testServer(t, Options{RecURL: backend.URL, RecUser: "u", RecPassword: "p"})

		for _, path := range []string{
			"/api/v1/recordings?date=2026-13-99x",
			"/api/v1/recordings/2026-06-04/..%2f..%2fetc%2fpasswd",
			"/api/v1/recordings/2026-06-04/shell.sh",
		} {
			rec := do(t, h, http.MethodGet, path, "test-token", "")
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("%s: got %d want 400", path, rec.Code)
			}
		}
	})

	t.Run("proxies listing", func(t *testing.T) {
		backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if u, p, _ := r.BasicAuth(); u != "u" || p != "p" {
				t.Errorf("missing basic auth on proxied request")
			}
			w.Write([]byte(`[{"name":"a.wav","type":"file","mtime":"m","size":5}]`))
		}))
		defer backend.Close()
		h := testServer(t, Options{RecURL: backend.URL, RecUser: "u", RecPassword: "p"})

		rec := do(t, h, http.MethodGet, "/api/v1/recordings?date=2026-06-04", "test-token", "")
		if rec.Code != http.StatusOK {
			t.Fatalf("got %d: %s", rec.Code, rec.Body.String())
		}
		for _, want := range []string{`"a.wav"`, `"/api/v1/recordings/2026-06-04/a.wav"`} {
			if !strings.Contains(rec.Body.String(), want) {
				t.Errorf("listing missing %s: %s", want, rec.Body.String())
			}
		}
	})
}
