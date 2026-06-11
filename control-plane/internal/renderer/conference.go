package renderer

import (
	"encoding/xml"
	"fmt"
	"strconv"

	"github.com/jamalshahverdiev/freeswitch-iac-platform/control-plane/internal/models"
)

type confDocument struct {
	XMLName xml.Name    `xml:"document"`
	Type    string      `xml:"type,attr"`
	Section confSection `xml:"section"`
}

type confSection struct {
	Name          string            `xml:"name,attr"`
	Configuration confConfiguration `xml:"configuration"`
}

type confConfiguration struct {
	Name        string        `xml:"name,attr"`
	Description string        `xml:"description,attr"`
	Profiles    []confProfile `xml:"profiles>profile"`
}

type confProfile struct {
	Name   string  `xml:"name,attr"`
	Params []param `xml:"param"`
}

// RenderConference builds conference.conf from desired state. mod_conference
// reads the profile when a room starts, so changes apply to NEW conferences
// without any module reload. Caller controls are not rendered: mod_conference
// installs its built-in "default" control group when none is configured.
func RenderConference(profiles []models.ConferenceProfile) ([]byte, error) {
	cfg := confConfiguration{
		Name:        "conference.conf",
		Description: "Audio/Video Conferences",
	}

	for _, p := range profiles {
		params := map[string]string{
			"rate":          strconv.Itoa(p.Rate),
			"interval":      strconv.Itoa(p.IntervalMs),
			"energy-level":  strconv.Itoa(p.EnergyLevel),
			"comfort-noise": strconv.FormatBool(p.ComfortNoise),
			"moh-sound":     p.MohSound,
		}
		if p.VideoMode != "" {
			params["video-mode"] = p.VideoMode
			params["video-layout-name"] = p.VideoLayout
			params["video-canvas-size"] = p.VideoCanvasSize
			params["video-fps"] = strconv.Itoa(p.VideoFPS)
		}
		if p.AutoRecord != "" {
			params["auto-record"] = p.AutoRecord
		}
		for k, v := range p.Params {
			params[k] = v
		}
		cfg.Profiles = append(cfg.Profiles, confProfile{
			Name:   p.Name,
			Params: sortedParams(params),
		})
	}

	doc := confDocument{
		Type:    "freeswitch/xml",
		Section: confSection{Name: "configuration", Configuration: cfg},
	}
	return marshal(doc)
}

// ConferenceRoomExtension converts a room into the dialplan extension callers
// dial to enter it: number -> answer + conference(name@profile[+pin]).
func ConferenceRoomExtension(r models.ConferenceRoom) models.DialplanExtension {
	data := r.Name + "@" + r.Profile
	if r.Pin != "" {
		data += "+" + r.Pin
	}
	actions := []models.DialplanAction{{Application: "answer"}}
	if r.MaxMembers > 0 {
		actions = append(actions, models.DialplanAction{
			Application: "set",
			Data:        fmt.Sprintf("conference_max_members=%d", r.MaxMembers),
		})
	}
	actions = append(actions, models.DialplanAction{Application: "conference", Data: data})

	return models.DialplanExtension{
		ID:       r.ID,
		Name:     "conference-" + r.Name,
		Domain:   r.Domain,
		Context:  r.Context,
		Priority: r.Priority,
		Enabled:  r.Enabled,
		Conditions: []models.DialplanCondition{{
			Field:      "destination_number",
			Expression: "^(" + r.Number + ")$",
			Actions:    actions,
		}},
	}
}
