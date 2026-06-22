package events

import "testing"

func TestParseEventBlock(t *testing.T) {
	// values are URL-encoded; %20 -> space; stops at blank line (ignores body)
	block := "Event-Name: CHANNEL_CREATE\n" +
		"Caller-Caller-ID-Number: 4201\n" +
		"Caller-Destination-Number: 4444\n" +
		"Caller-Caller-ID-Name: Jamal%20S\n" +
		"\n" +
		"this is the event body, ignored\n"
	m := parseEventBlock([]byte(block))
	if m["Event-Name"] != "CHANNEL_CREATE" {
		t.Errorf("Event-Name = %q", m["Event-Name"])
	}
	if m["Caller-Caller-ID-Name"] != "Jamal S" {
		t.Errorf("URL-decode failed: %q", m["Caller-Caller-ID-Name"])
	}
	if _, ok := m["this is the event body, ignored"]; ok {
		t.Error("parsed past the blank line into the body")
	}
}

func TestParseVoiceMessage(t *testing.T) {
	cases := []struct {
		in               string
		wantNew, wantOld int
	}{
		{"2/1 (0/0)", 2, 1},
		{"0/0 (0/0)", 0, 0},
		{"5/3", 5, 3},
		{"", 0, 0},
		{"garbage", 0, 0},
	}
	for _, c := range cases {
		n, o := parseVoiceMessage(c.in)
		if n != c.wantNew || o != c.wantOld {
			t.Errorf("parseVoiceMessage(%q) = %d/%d, want %d/%d", c.in, n, o, c.wantNew, c.wantOld)
		}
	}
}

func TestNormalize(t *testing.T) {
	cases := []struct {
		name     string
		in       map[string]string
		wantType string
		wantOK   bool
		check    func(Event) bool
	}{
		{"create", map[string]string{
			"Event-Name": "CHANNEL_CREATE", "Unique-ID": "u1",
			"Call-Direction": "inbound", "Caller-Caller-ID-Number": "4201",
			"Caller-Destination-Number": "4444",
		}, "call.started", true, func(e Event) bool {
			return e.Data["uuid"] == "u1" && e.Data["caller"] == "4201" && e.Data["destination"] == "4444"
		}},
		{"answer", map[string]string{"Event-Name": "CHANNEL_ANSWER", "Unique-ID": "u1"},
			"call.answered", true, func(e Event) bool { return e.Data["uuid"] == "u1" }},
		{"hangup", map[string]string{
			"Event-Name": "CHANNEL_HANGUP_COMPLETE", "Unique-ID": "u1",
			"Hangup-Cause": "NORMAL_CLEARING", "variable_billsec": "42",
		}, "call.ended", true, func(e Event) bool {
			return e.Data["cause"] == "NORMAL_CLEARING" && e.Data["billsec"] == "42"
		}},
		{"agent status", map[string]string{
			"Event-Name": "CUSTOM", "Event-Subclass": "callcenter::info",
			"CC-Action": "agent-status-change", "CC-Agent": "4201@d", "CC-Agent-Status": "Available",
		}, "agent.status", true, func(e Event) bool {
			return e.Data["agent"] == "4201@d" && e.Data["status"] == "Available"
		}},
		{"queue member", map[string]string{
			"Event-Name": "CUSTOM", "Event-Subclass": "callcenter::info",
			"CC-Action": "members-count", "CC-Queue": "support@d",
		}, "queue.member", true, func(e Event) bool { return e.Data["queue"] == "support@d" }},
		{"conference", map[string]string{
			"Event-Name": "CUSTOM", "Event-Subclass": "conference::maintenance",
			"Action": "add-member", "Conference-Name": "standup",
		}, "conference", true, func(e Event) bool {
			return e.Data["name"] == "standup" && e.Data["action"] == "add-member"
		}},
		{"message waiting", map[string]string{
			"Event-Name": "MESSAGE_WAITING", "MWI-Messages-Waiting": "yes",
			"MWI-Message-Account": "sip:1001@192.168.48.143", "MWI-Voice-Message": "2/1 (0/0)",
		}, "voicemail.mwi", true, func(e Event) bool {
			return e.Data["account"] == "1001@192.168.48.143" && e.Data["user"] == "1001" &&
				e.Data["domain"] == "192.168.48.143" && e.Data["waiting"] == "yes" &&
				e.Data["new"] == "2" && e.Data["saved"] == "1"
		}},
		{"ignored DTMF", map[string]string{"Event-Name": "DTMF"}, "", false, nil},
		{"ignored custom subclass", map[string]string{
			"Event-Name": "CUSTOM", "Event-Subclass": "sofia::register",
		}, "", false, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e, ok := normalize(c.in)
			if ok != c.wantOK {
				t.Fatalf("ok = %v, want %v", ok, c.wantOK)
			}
			if !ok {
				return
			}
			if e.Type != c.wantType {
				t.Errorf("type = %q, want %q", e.Type, c.wantType)
			}
			if c.check != nil && !c.check(e) {
				t.Errorf("data check failed: %+v", e.Data)
			}
		})
	}
}
