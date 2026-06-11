package runtime

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

// Client is a minimal FreeSWITCH event socket (ESL) inbound client.
// It connects per command, which is sufficient for low-frequency runtime
// operations like reloadxml and status queries.
type Client struct {
	Addr     string
	Password string
	Timeout  time.Duration
}

func New(addr, password string, timeout time.Duration) *Client {
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	return &Client{Addr: addr, Password: password, Timeout: timeout}
}

// Enabled reports whether an ESL address was configured.
func (c *Client) Enabled() bool { return c.Addr != "" }

// API runs an `api <command>` against FreeSWITCH and returns the response body.
func (c *Client) API(command string) (string, error) {
	if !c.Enabled() {
		return "", fmt.Errorf("esl not configured")
	}
	conn, err := net.DialTimeout("tcp", c.Addr, c.Timeout)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(c.Timeout))

	r := bufio.NewReader(conn)

	// Expect auth/request.
	if _, err := readHeaders(r); err != nil {
		return "", fmt.Errorf("read auth request: %w", err)
	}
	if _, err := fmt.Fprintf(conn, "auth %s\n\n", c.Password); err != nil {
		return "", err
	}
	reply, err := readHeaders(r)
	if err != nil {
		return "", fmt.Errorf("read auth reply: %w", err)
	}
	if !strings.Contains(reply["Reply-Text"], "+OK") {
		return "", fmt.Errorf("esl auth failed: %s", reply["Reply-Text"])
	}

	if _, err := fmt.Fprintf(conn, "api %s\n\n", command); err != nil {
		return "", err
	}
	headers, err := readHeaders(r)
	if err != nil {
		return "", fmt.Errorf("read api response: %w", err)
	}
	body := ""
	if cl := headers["Content-Length"]; cl != "" {
		n, err := strconv.Atoi(cl)
		if err != nil {
			return "", err
		}
		buf := make([]byte, n)
		if _, err := io.ReadFull(r, buf); err != nil {
			return "", err
		}
		body = string(buf)
	}
	return strings.TrimSpace(body), nil
}

// ReloadXML triggers `reloadxml` on FreeSWITCH.
func (c *Client) ReloadXML() (string, error) {
	return c.API("reloadxml")
}

// Ping checks ESL connectivity using the `status` command.
func (c *Client) Ping() error {
	_, err := c.API("status")
	return err
}

// GatewayStatus returns the parsed key/value output of
// `sofia status gateway <name>`.
func (c *Client) GatewayStatus(name string) (map[string]string, error) {
	out, err := c.API("sofia status gateway " + name)
	if err != nil {
		return nil, err
	}
	if strings.Contains(out, "Invalid Gateway") || strings.TrimSpace(out) == "" {
		return nil, nil
	}
	result := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "=") {
			continue
		}
		// lines look like "State \t NOREG" (key, then whitespace, then value)
		fields := strings.SplitN(line, "\t", 2)
		if len(fields) != 2 {
			// fall back to splitting on runs of spaces
			idx := strings.IndexAny(line, " \t")
			if idx <= 0 {
				continue
			}
			fields = []string{line[:idx], strings.TrimSpace(line[idx:])}
		}
		key := strings.TrimSpace(fields[0])
		val := strings.TrimSpace(fields[1])
		if key != "" {
			result[key] = val
		}
	}
	return result, nil
}

// Registration looks up a single SIP registration by user and realm/domain via
// `show registrations as json`. Returns nil if not registered.
func (c *Client) Registration(user, domain string) (map[string]string, error) {
	out, err := c.API("show registrations as json")
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Rows []map[string]string `json:"rows"`
	}
	if strings.TrimSpace(out) == "" {
		return nil, nil
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		return nil, fmt.Errorf("parse registrations: %w", err)
	}
	for _, row := range parsed.Rows {
		if row["reg_user"] == user && (row["realm"] == domain || domain == "") {
			return row, nil
		}
	}
	return nil, nil
}

func readHeaders(r *bufio.Reader) (map[string]string, error) {
	headers := map[string]string{}
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			return headers, nil
		}
		if idx := strings.Index(line, ":"); idx > 0 {
			key := strings.TrimSpace(line[:idx])
			val := strings.TrimSpace(line[idx+1:])
			headers[key] = val
		}
	}
}
