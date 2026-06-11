package renderer

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/xml"

	"github.com/jamalshahverdiev/freeswitch-iac-platform/control-plane/internal/models"
)

type dirDocument struct {
	XMLName xml.Name   `xml:"document"`
	Type    string     `xml:"type,attr"`
	Section dirSection `xml:"section"`
}

type dirSection struct {
	Name    string      `xml:"name,attr"`
	Domains []dirDomain `xml:"domain"`
}

type dirDomain struct {
	Name      string    `xml:"name,attr"`
	Params    []param   `xml:"params>param"`
	Variables []param   `xml:"variables>variable"`
	Groups    dirGroups `xml:"groups"`
}

// defaultDialString lets registered users be reached via user/<number>@<domain>.
const defaultDialString = "{presence_id=${dialed_user}@${dialed_domain}}${sofia_contact(${dialed_user}@${dialed_domain})}"

type dirGroups struct {
	Group dirGroup `xml:"group"`
}

type dirGroup struct {
	Name  string    `xml:"name,attr"`
	Users []dirUser `xml:"users>user"`
}

type dirUser struct {
	ID        string  `xml:"id,attr"`
	Params    []param `xml:"params>param"`
	Variables []param `xml:"variables>variable"`
}

// RenderDirectory builds FreeSWITCH directory XML for the given domains/users.
func RenderDirectory(domains []models.DomainWithUsers) ([]byte, error) {
	doc := dirDocument{
		Type:    "freeswitch/xml",
		Section: dirSection{Name: "directory"},
	}
	for _, dw := range domains {
		dd := dirDomain{
			Name:      dw.Domain.Name,
			Params:    []param{{Name: "dial-string", Value: defaultDialString}},
			Variables: sortedParams(dw.Domain.Variables),
			Groups:    dirGroups{Group: dirGroup{Name: "default"}},
		}
		for _, u := range dw.Users {
			dd.Groups.Group.Users = append(dd.Groups.Group.Users, dirUser{
				ID:        u.Number,
				Params:    directoryUserParams(u.Number, dw.Domain.Name, u.Params),
				Variables: sortedParams(u.Variables),
			})
		}
		doc.Section.Domains = append(doc.Section.Domains, dd)
	}
	return marshal(doc)
}

// directoryUserParams renders the user's directory params, replacing the
// plaintext "password" with an "a1-hash" = MD5(user:realm:password) so the
// SIP secret never appears in the /xml/directory response. realm is the domain
// the user authenticates in. Other params (e.g. vm-password) pass through.
func directoryUserParams(number, domain string, in map[string]string) []param {
	out := make(map[string]string, len(in))
	for k, v := range in {
		if k == "password" {
			out["a1-hash"] = a1Hash(number, domain, v)
			continue
		}
		out[k] = v
	}
	return sortedParams(out)
}

func a1Hash(user, realm, password string) string {
	sum := md5.Sum([]byte(user + ":" + realm + ":" + password))
	return hex.EncodeToString(sum[:])
}

func marshal(doc any) ([]byte, error) {
	body, err := xml.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}
	return append([]byte(xmlHeader), body...), nil
}
