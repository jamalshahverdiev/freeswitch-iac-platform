package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	HTTPAddr    string
	DatabaseURL string
	APIToken    string
	ESLAddr     string
	ESLPassword string
	ESLTimeout  time.Duration

	// XML endpoint protection (/xml/* consumed by mod_xml_curl).
	XMLUser       string
	XMLPassword   string   // if set, /xml/* requires HTTP Basic auth
	XMLAllowCIDRs []string // if non-empty, /xml/* is restricted to these source CIDRs

	// TLS. If TLSCertFile+TLSKeyFile are set the server speaks HTTPS.
	// If XMLClientCAFile is set, /xml/* additionally requires a client cert
	// signed by that CA (mTLS).
	TLSCertFile     string
	TLSKeyFile      string
	XMLClientCAFile string

	// CCOdbcDSN ("dsn:user:pass") is emitted into the rendered callcenter.conf
	// so mod_callcenter keeps its runtime tables in PostgreSQL, not sqlite.
	CCOdbcDSN string
	// Recordings file server on the FS host (nginx) proxied by /api/v1/recordings.
	RecURL      string
	RecUser     string
	RecPassword string
}

func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:      env("HTTP_ADDR", ":8080"),
		DatabaseURL:   env("DATABASE_URL", ""),
		APIToken:      env("API_TOKEN", "dev-token"),
		ESLAddr:       env("ESL_ADDR", ""),
		ESLPassword:   env("ESL_PASSWORD", "ClueCon"),
		ESLTimeout:    5 * time.Second,
		XMLUser:         env("XML_USER", "freeswitch"),
		XMLPassword:     env("XML_PASSWORD", ""),
		XMLAllowCIDRs:   splitCSV(env("XML_ALLOW_CIDRS", "")),
		TLSCertFile:     env("TLS_CERT_FILE", ""),
		TLSKeyFile:      env("TLS_KEY_FILE", ""),
		XMLClientCAFile: env("XML_CLIENT_CA_FILE", ""),
		CCOdbcDSN:       env("CC_ODBC_DSN", ""),
		RecURL:          env("REC_URL", ""),
		RecUser:         env("REC_USER", "recordings"),
		RecPassword:     env("REC_PASSWORD", ""),
	}
	if cfg.DatabaseURL == "" {
		return cfg, fmt.Errorf("DATABASE_URL is required")
	}
	return cfg, nil
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
