package renderer

import (
	"encoding/xml"
	"sort"
	"strconv"

	"github.com/jamalshahverdiev/freeswitch-iac-platform/control-plane/internal/models"
)

type cfgDocument struct {
	XMLName xml.Name  `xml:"document"`
	Type    string    `xml:"type,attr"`
	Section cfgSection `xml:"section"`
}

type cfgSection struct {
	Name          string           `xml:"name,attr"`
	Configuration cfgConfiguration `xml:"configuration"`
}

type cfgConfiguration struct {
	Name        string       `xml:"name,attr"`
	Description string       `xml:"description,attr"`
	Profiles    []cfgProfile `xml:"profiles>profile"`
}

type cfgProfile struct {
	Name     string       `xml:"name,attr"`
	Gateways []cfgGateway `xml:"gateways>gateway"`
}

type cfgGateway struct {
	Name   string  `xml:"name,attr"`
	Params []param `xml:"param"`
}

// RenderSofiaGateways builds sofia.conf XML containing only the gateways from
// the desired state, grouped by Sofia profile. Disabled gateways are skipped.
func RenderSofiaGateways(gateways []models.Gateway) ([]byte, error) {
	byProfile := map[string][]cfgGateway{}
	for _, g := range gateways {
		if !g.Enabled {
			continue
		}
		params := map[string]string{
			"proxy":    g.Proxy,
			"register": strconv.FormatBool(g.Register),
		}
		if g.Username != "" {
			params["username"] = g.Username
		}
		if g.Password != "" {
			params["password"] = g.Password
		}
		if g.Realm != "" {
			params["realm"] = g.Realm
		}
		for k, v := range g.Params {
			params[k] = v
		}
		byProfile[g.Profile] = append(byProfile[g.Profile], cfgGateway{
			Name:   g.Name,
			Params: sortedParams(params),
		})
	}

	names := make([]string, 0, len(byProfile))
	for name := range byProfile {
		names = append(names, name)
	}
	sort.Strings(names)

	cfg := cfgConfiguration{Name: "sofia.conf", Description: "Sofia SIP Endpoint"}
	for _, name := range names {
		cfg.Profiles = append(cfg.Profiles, cfgProfile{Name: name, Gateways: byProfile[name]})
	}

	doc := cfgDocument{
		Type:    "freeswitch/xml",
		Section: cfgSection{Name: "configuration", Configuration: cfg},
	}
	return marshal(doc)
}
