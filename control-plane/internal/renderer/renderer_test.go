package renderer

import (
	"strings"
	"testing"

	"github.com/jamalshahverdiev/freeswitch-iac-platform/control-plane/internal/models"
)

// must wraps a renderer's ([]byte, error) return: must(t)(RenderX(...)).
func must(t *testing.T) func([]byte, error) string {
	t.Helper()
	return func(body []byte, err error) string {
		if err != nil {
			t.Fatalf("render failed: %v", err)
		}
		return string(body)
	}
}

func wantContains(t *testing.T, xml string, subs ...string) {
	t.Helper()
	for _, sub := range subs {
		if !strings.Contains(xml, sub) {
			t.Errorf("rendered XML missing %q\n--- got ---\n%s", sub, xml)
		}
	}
}

func wantNotContains(t *testing.T, xml string, subs ...string) {
	t.Helper()
	for _, sub := range subs {
		if strings.Contains(xml, sub) {
			t.Errorf("rendered XML must NOT contain %q\n--- got ---\n%s", sub, xml)
		}
	}
}

// ---------- directory ----------

func TestRenderDirectoryA1Hash(t *testing.T) {
	out := must(t)(RenderDirectory([]models.DomainWithUsers{{
		Domain: models.Domain{Name: "lab.test", Variables: map[string]string{"zone": "lab", "area": "1"}},
		Users: []models.User{{
			Number: "1001",
			Params: map[string]string{"password": "s3cret", "vm-password": "9999"},
			Variables: map[string]string{"user_context": "company"},
		}},
	}}))

	wantContains(t, out,
		`<domain name="lab.test">`,
		`<user id="1001">`,
		// password is replaced with MD5(user:realm:password)
		`<param name="a1-hash" value="f9c319be8ef5f22bf989e5936b21ed23">`,
		// vm-password passes through untouched
		`<param name="vm-password" value="9999">`,
		`<variable name="user_context" value="company">`,
		`<param name="dial-string"`,
	)
	// The SIP secret must never appear, neither as value nor as param name.
	wantNotContains(t, out, "s3cret", `name="password"`)

	// Domain variables are sorted by key (area before zone).
	if strings.Index(out, `name="area"`) > strings.Index(out, `name="zone"`) {
		t.Error("domain variables are not sorted by key")
	}
}

func TestRenderDirectoryVoicemail(t *testing.T) {
	out := must(t)(RenderDirectory([]models.DomainWithUsers{{
		Domain: models.Domain{Name: "lab.test"},
		Users: []models.User{{
			Number: "1002",
			Params: map[string]string{"password": "s3cret", "vm-password": "stale"},
			Voicemail: &models.Voicemail{
				Enabled: true, Password: "1234", Email: "a@b.com",
				AttachFile: true, EmailAll: false,
			},
		}},
	}}))

	wantContains(t, out,
		`<param name="vm-enabled" value="true">`,
		// typed password wins over the freeform vm-password key
		`<param name="vm-password" value="1234">`,
		`<param name="vm-mailto" value="a@b.com">`,
		`<param name="vm-attach-file" value="true">`,
		`<param name="vm-email-all-messages" value="false">`,
	)
	// the stale freeform vm-password must not survive
	wantNotContains(t, out, `value="stale"`)
}

func TestRenderDirectoryNoVoicemail(t *testing.T) {
	out := must(t)(RenderDirectory([]models.DomainWithUsers{{
		Domain: models.Domain{Name: "lab.test"},
		Users:  []models.User{{Number: "1003", Params: map[string]string{"password": "x"}}},
	}}))
	// nil mailbox renders no vm-* params at all
	wantNotContains(t, out, "vm-enabled", "vm-mailto", "vm-attach-file")
}

func TestRenderVoicemail(t *testing.T) {
	out := must(t)(RenderVoicemail("freeswitch-core:fs:pw"))
	wantContains(t, out,
		`<configuration name="voicemail.conf"`,
		`<profile name="default">`,
		`<param name="odbc-dsn" value="freeswitch-core:fs:pw">`,
		`<param name="file-extension" value="wav">`,
		`<email>`,
		`<param name="email-from" value="${voicemail_account}@${voicemail_domain}">`,
	)
}

func TestRenderVoicemailNoDSN(t *testing.T) {
	out := must(t)(RenderVoicemail(""))
	// empty DSN omits the param entirely (no odbc-dsn line)
	wantNotContains(t, out, "odbc-dsn")
}

// ---------- dialplan ----------

func dpExt(name, context string, prio int, enabled bool) models.DialplanExtension {
	return models.DialplanExtension{
		Name: name, Context: context, Priority: prio, Enabled: enabled,
		Conditions: []models.DialplanCondition{{
			Field: "destination_number", Expression: "^(100)$",
			Actions: []models.DialplanAction{{Application: "answer"}, {Application: "echo"}},
		}},
	}
}

func TestRenderDialplan(t *testing.T) {
	exts := []models.DialplanExtension{
		dpExt("a", "company", 10, true),
		dpExt("disabled", "company", 11, false),
		dpExt("b", "support", 5, true),
	}

	t.Run("groups by context and skips disabled", func(t *testing.T) {
		out := must(t)(RenderDialplan(exts, ""))
		wantContains(t, out,
			`<context name="company">`,
			`<context name="support">`,
			`<extension name="a">`,
			`<action application="answer">`,
		)
		wantNotContains(t, out, `<extension name="disabled">`)
	})

	t.Run("context filter", func(t *testing.T) {
		out := must(t)(RenderDialplan(exts, "support"))
		wantContains(t, out, `<extension name="b">`)
		wantNotContains(t, out, `<extension name="a">`, `<context name="company">`)
	})
}

func TestRenderDialplanTimeAttrs(t *testing.T) {
	exts := []models.DialplanExtension{
		{ // regex + time window (business hours)
			Name: "hours", Context: "company", Priority: 10, Enabled: true,
			Conditions: []models.DialplanCondition{{
				Field: "destination_number", Expression: "^(4444)$",
				TimeAttrs: map[string]string{"wday": "2-6", "hour": "9-17"},
				Actions:   []models.DialplanAction{{Application: "transfer", Data: "support@d"}},
			}},
		},
		{ // pure time gate, no field/expression
			Name: "night", Context: "company", Priority: 11, Enabled: true,
			Conditions: []models.DialplanCondition{{
				TimeAttrs: map[string]string{"time-of-day": "18:00-9:00"},
				Actions:   []models.DialplanAction{{Application: "playback", Data: "closed.wav"}},
			}},
		},
	}
	out := must(t)(RenderDialplan(exts, "company"))
	wantContains(t, out,
		`wday="2-6"`, `hour="9-17"`,
		`field="destination_number" expression="^(4444)$"`,
		`time-of-day="18:00-9:00"`,
	)
	// pure time gate must NOT emit empty field/expression attributes
	wantNotContains(t, out, `field=""`, `expression=""`)
}

// ---------- callcenter ----------

func TestRenderCallcenter(t *testing.T) {
	queues := []models.CCQueue{{
		Name: "support@lab.test", Strategy: "longest-idle-agent",
		MohSound: "local_stream://moh", TimeBaseScore: "system",
		DiscardAbandonedAfter: 60,
		Params:                map[string]string{"record-template": "/rec/${uuid}.wav"},
	}}
	agents := []models.CCAgent{{
		Name: "1001@lab.test", Type: "callback", Contact: "user/1001@lab.test",
		Status: "Available", MaxNoAnswer: 3, WrapUpTime: 10,
		RejectDelayTime: 3, BusyDelayTime: 60, NoAnswerDelayTime: 60,
	}}
	tiers := []models.CCTier{{Queue: "support@lab.test", Agent: "1001@lab.test", Level: 1, Position: 1}}

	t.Run("with odbc dsn", func(t *testing.T) {
		out := must(t)(RenderCallcenter(queues, agents, tiers, "fs:user:pass"))
		wantContains(t, out,
			`<configuration name="callcenter.conf"`,
			`<param name="odbc-dsn" value="fs:user:pass">`,
			`<queue name="support@lab.test">`,
			`<param name="strategy" value="longest-idle-agent">`,
			// extra params from the JSONB map are merged in
			`<param name="record-template" value="/rec/${uuid}.wav">`,
			`<agent name="1001@lab.test" type="callback" contact="user/1001@lab.test" status="Available"`,
			`<tier agent="1001@lab.test" queue="support@lab.test" level="1" position="1">`,
		)
	})

	t.Run("no sqlite fallback without dsn", func(t *testing.T) {
		out := must(t)(RenderCallcenter(queues, agents, tiers, ""))
		wantNotContains(t, out, "odbc-dsn")
	})
}

// ---------- conference ----------

func TestRenderConference(t *testing.T) {
	base := models.ConferenceProfile{
		Name: "audio-only", Rate: 48000, IntervalMs: 20, EnergyLevel: 200,
		ComfortNoise: true, MohSound: "local_stream://moh",
		VideoLayout: "group:grid", VideoCanvasSize: "1280x720", VideoFPS: 15,
	}

	t.Run("audio profile omits video params", func(t *testing.T) {
		out := must(t)(RenderConference([]models.ConferenceProfile{base}))
		wantContains(t, out, `<profile name="audio-only">`, `<param name="rate" value="48000">`)
		wantNotContains(t, out, "video-mode", "video-layout-name", "auto-record")
	})

	t.Run("video mux profile", func(t *testing.T) {
		p := base
		p.Name = "video-grid"
		p.VideoMode = "mux"
		p.AutoRecord = "/rec/${conference_name}.mp4"
		p.Params = map[string]string{"energy-level": "300"} // override via map
		out := must(t)(RenderConference([]models.ConferenceProfile{p}))
		wantContains(t, out,
			`<param name="video-mode" value="mux">`,
			`<param name="video-layout-name" value="group:grid">`,
			`<param name="video-canvas-size" value="1280x720">`,
			`<param name="auto-record" value="/rec/${conference_name}.mp4">`,
			// map params win over canonical columns
			`<param name="energy-level" value="300">`,
		)
		wantNotContains(t, out, `<param name="energy-level" value="200">`)
	})
}

func TestConferenceRoomExtension(t *testing.T) {
	room := models.ConferenceRoom{
		ID: "id-1", Name: "standup", Number: "3500", Domain: "lab.test",
		Context: "company", Profile: "video-grid", Priority: 5, Enabled: true,
	}

	t.Run("plain room", func(t *testing.T) {
		ext := ConferenceRoomExtension(room)
		if ext.Name != "conference-standup" || ext.Context != "company" || ext.Priority != 5 {
			t.Errorf("unexpected extension meta: %+v", ext)
		}
		cond := ext.Conditions[0]
		if cond.Expression != "^(3500)$" {
			t.Errorf("expression = %q", cond.Expression)
		}
		last := cond.Actions[len(cond.Actions)-1]
		if last.Application != "conference" || last.Data != "standup@video-grid" {
			t.Errorf("conference action = %+v", last)
		}
	})

	t.Run("pin and max members", func(t *testing.T) {
		r := room
		r.Pin = "1234"
		r.MaxMembers = 8
		ext := ConferenceRoomExtension(r)
		var apps []string
		var confData string
		for _, a := range ext.Conditions[0].Actions {
			apps = append(apps, a.Application)
			if a.Application == "conference" {
				confData = a.Data
			}
			if a.Application == "set" && a.Data != "conference_max_members=8" {
				t.Errorf("set action = %q", a.Data)
			}
		}
		if confData != "standup@video-grid+1234" {
			t.Errorf("pin dialstring = %q", confData)
		}
		if strings.Join(apps, ",") != "answer,set,conference" {
			t.Errorf("action order = %v", apps)
		}
	})
}

// ---------- not-found fallback ----------

func TestNotFoundDocument(t *testing.T) {
	wantContains(t, NotFoundDocument, `<result status="not found"/>`, `type="freeswitch/xml"`)
}
