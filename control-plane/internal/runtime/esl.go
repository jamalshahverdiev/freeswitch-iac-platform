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
	conn, r, err := c.dialAuth()
	if err != nil {
		return "", err
	}
	defer conn.Close()

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

// BGAPI runs `bgapi <command>` (non-blocking) and returns the immediate
// command reply (e.g. "+OK Job-UUID: ..."). Use for commands that would
// otherwise block — like `originate` ringing a phone — past the ESL timeout.
func (c *Client) BGAPI(command string) (string, error) {
	if !c.Enabled() {
		return "", fmt.Errorf("esl not configured")
	}
	conn, r, err := c.dialAuth()
	if err != nil {
		return "", err
	}
	defer conn.Close()

	if _, err := fmt.Fprintf(conn, "bgapi %s\n\n", command); err != nil {
		return "", err
	}
	headers, err := readHeaders(r)
	if err != nil {
		return "", fmt.Errorf("read bgapi response: %w", err)
	}
	return strings.TrimSpace(headers["Reply-Text"]), nil
}

// dialAuth opens an ESL connection and authenticates.
func (c *Client) dialAuth() (net.Conn, *bufio.Reader, error) {
	conn, err := net.DialTimeout("tcp", c.Addr, c.Timeout)
	if err != nil {
		return nil, nil, err
	}
	_ = conn.SetDeadline(time.Now().Add(c.Timeout))
	r := bufio.NewReader(conn)
	if _, err := readHeaders(r); err != nil { // auth/request
		conn.Close()
		return nil, nil, fmt.Errorf("read auth request: %w", err)
	}
	if _, err := fmt.Fprintf(conn, "auth %s\n\n", c.Password); err != nil {
		conn.Close()
		return nil, nil, err
	}
	reply, err := readHeaders(r)
	if err != nil {
		conn.Close()
		return nil, nil, fmt.Errorf("read auth reply: %w", err)
	}
	if !strings.Contains(reply["Reply-Text"], "+OK") {
		conn.Close()
		return nil, nil, fmt.Errorf("esl auth failed: %s", reply["Reply-Text"])
	}
	return conn, r, nil
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

// Channel is one active FreeSWITCH channel (call leg). All fields come from
// `show channels as json`, which emits every value as a string.
type Channel struct {
	UUID      string `json:"uuid"`
	Direction string `json:"direction"`
	Created   string `json:"created_epoch"`
	Name      string `json:"name"`
	State     string `json:"state"`
	CIDName   string `json:"cid_name"`
	CIDNum    string `json:"cid_num"`
	Dest      string `json:"dest"`
	CallState string `json:"callstate"`
	CalleeNum string `json:"callee_num"`
}

// Channels lists the active channels (call legs) via `show channels as json`.
func (c *Client) Channels() ([]Channel, error) {
	body, err := c.API("show channels as json")
	if err != nil {
		return nil, err
	}
	var out struct {
		RowCount int       `json:"row_count"`
		Rows     []Channel `json:"rows"`
	}
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		return nil, fmt.Errorf("parse channels: %w", err)
	}
	if out.Rows == nil {
		out.Rows = []Channel{}
	}
	return out.Rows, nil
}

// Hangup disconnects a channel (uuid_kill). Returns the raw "+OK"/"-ERR" reply.
func (c *Client) Hangup(uuid string) (string, error) {
	return c.API("uuid_kill " + uuid)
}

// TransferChannel transfers a live channel to a destination extension in a
// dialplan context (uuid_transfer ... XML <context>).
func (c *Client) TransferChannel(uuid, dest, context string) (string, error) {
	return c.API(fmt.Sprintf("uuid_transfer %s %s XML %s", uuid, dest, context))
}

// ParkChannel parks a live channel (uuid_park).
func (c *Client) ParkChannel(uuid string) (string, error) {
	return c.API("uuid_park " + uuid)
}

// Eavesdrop lets a supervisor covertly listen to a channel: it originates a call
// to the supervisor's own extension which, on answer, runs the eavesdrop app on
// the target channel. The supervisor hears the conversation silently and can
// switch to whisper / barge with DTMF 1 / 2 / 3.
func (c *Client) Eavesdrop(targetUUID, ext, domain string) (string, error) {
	// bgapi: the originate rings the supervisor and waits for answer, which
	// exceeds the ESL timeout — fire it non-blocking and return the job ack.
	return c.BGAPI(fmt.Sprintf(
		"originate {origination_caller_id_name='Supervisor Spy',origination_caller_id_number=spy}user/%s@%s &eavesdrop(%s)",
		ext, domain, targetUUID))
}
