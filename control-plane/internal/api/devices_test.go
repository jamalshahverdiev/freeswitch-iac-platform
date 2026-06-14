package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jamalshahverdiev/freeswitch-iac-platform/control-plane/internal/models"
)

func TestMacFromFile(t *testing.T) {
	cases := map[string]string{
		"805ec0112233.cfg":     "805ec0112233", // Yealink
		"cfg805ec0112233.xml":  "805ec0112233", // Grandstream
		"80:5E:C0:11:22:33.cfg": "805ec0112233", // separators + uppercase
		"805ec0112233.xml":     "805ec0112233", // generic
	}
	for in, want := range cases {
		if got := macFromFile(in); got != want {
			t.Errorf("macFromFile(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRenderProvision(t *testing.T) {
	acct := &models.DeviceAccount{
		Device:   models.Device{Number: "1001", DisplayName: "Reception"},
		Password: "s3cret",
	}

	t.Run("yealink", func(t *testing.T) {
		acct.Vendor = "yealink"
		body, ct := renderProvision(acct, "192.168.48.143", "5060")
		if !strings.HasPrefix(ct, "text/plain") {
			t.Errorf("content-type = %q", ct)
		}
		for _, want := range []string{
			"account.1.auth_name = 1001",
			"account.1.password = s3cret",
			"account.1.sip_server.1.address = 192.168.48.143",
			"account.1.sip_server.1.port = 5060",
			"account.1.display_name = Reception",
		} {
			if !strings.Contains(body, want) {
				t.Errorf("yealink config missing %q:\n%s", want, body)
			}
		}
	})

	t.Run("grandstream", func(t *testing.T) {
		acct.Vendor = "grandstream"
		body, ct := renderProvision(acct, "192.168.48.143", "5060")
		if !strings.HasPrefix(ct, "text/xml") {
			t.Errorf("content-type = %q", ct)
		}
		for _, want := range []string{
			"<P47>192.168.48.143</P47>",
			"<P35>1001</P35>",
			"<P34>s3cret</P34>",
		} {
			if !strings.Contains(body, want) {
				t.Errorf("grandstream config missing %q:\n%s", want, body)
			}
		}
	})

	t.Run("display name falls back to number", func(t *testing.T) {
		a := &models.DeviceAccount{Device: models.Device{Vendor: "yealink", Number: "1002"}, Password: "p"}
		body, _ := renderProvision(a, "h", "5060")
		if !strings.Contains(body, "account.1.display_name = 1002") {
			t.Errorf("expected number as display name:\n%s", body)
		}
	})

	t.Run("xml escaping", func(t *testing.T) {
		a := &models.DeviceAccount{Device: models.Device{Vendor: "grandstream", Number: "1", DisplayName: `A&B<C>`}, Password: "p"}
		body, _ := renderProvision(a, "h", "5060")
		if strings.Contains(body, "A&B<C>") {
			t.Errorf("display name not escaped:\n%s", body)
		}
		if !strings.Contains(body, "A&amp;B&lt;C&gt;") {
			t.Errorf("expected escaped display name:\n%s", body)
		}
	})
}

func TestProvisionGuardRequiresBasicAuth(t *testing.T) {
	h := testServer(t, Options{ProvisionUser: "provision", ProvisionPassword: "secret"})

	// no credentials -> 401 (never reaches the nil store)
	rec := do(t, h, http.MethodGet, "/provision/805ec0112233.cfg", "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no auth: got %d want 401", rec.Code)
	}

	// wrong credentials -> 401
	req := httptest.NewRequest(http.MethodGet, "/provision/805ec0112233.cfg", nil)
	req.SetBasicAuth("provision", "wrong")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong auth: got %d want 401", rec.Code)
	}
}

func TestDeviceCreateValidation(t *testing.T) {
	h := testServer(t, Options{})
	tok := "test-token"
	cases := []struct{ name, body string }{
		{"bad json", `{not json`},
		{"missing mac", `{"number":"1","domain":"d"}`},
		{"missing number", `{"mac":"aa","domain":"d"}`},
		{"missing domain", `{"mac":"aa","number":"1"}`},
		{"bad vendor", `{"mac":"aa","number":"1","domain":"d","vendor":"cisco"}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := do(t, h, http.MethodPost, "/api/v1/devices", tok, c.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("got %d want 400 (body: %s)", rec.Code, rec.Body.String())
			}
		})
	}
}
