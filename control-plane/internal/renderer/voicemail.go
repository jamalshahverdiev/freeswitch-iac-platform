package renderer

import "encoding/xml"

type vmDocument struct {
	XMLName xml.Name  `xml:"document"`
	Type    string    `xml:"type,attr"`
	Section vmSection `xml:"section"`
}

type vmSection struct {
	Name          string          `xml:"name,attr"`
	Configuration vmConfiguration `xml:"configuration"`
}

type vmConfiguration struct {
	Name        string      `xml:"name,attr"`
	Description string      `xml:"description,attr"`
	Settings    vmEmpty     `xml:"settings"`
	Profiles    []vmProfile `xml:"profiles>profile"`
}

type vmEmpty struct{}

type vmProfile struct {
	Name   string    `xml:"name,attr"`
	Params []param   `xml:"param"`
	Email  vmEmail   `xml:"email"`
}

type vmEmail struct {
	Params []param `xml:"param"`
}

// RenderVoicemail builds voicemail.conf with a single "default" profile.
// odbcDSN ("dsn:user:pass") points mod_voicemail's message store at PostgreSQL
// (freeswitch_core); empty omits the param. Key-binding params are left to
// mod_voicemail's built-in defaults — only the operationally meaningful
// settings are rendered, so the profile stays config-as-code, not a 40-line
// copy of the on-disk file.
func RenderVoicemail(odbcDSN string) ([]byte, error) {
	profile := vmProfile{
		Name: "default",
		Params: []param{
			{Name: "file-extension", Value: "wav"},
			{Name: "terminator-key", Value: "#"},
			{Name: "max-login-attempts", Value: "3"},
			{Name: "digit-timeout", Value: "10000"},
			{Name: "min-record-len", Value: "3"},
			{Name: "max-record-len", Value: "300"},
			{Name: "max-retries", Value: "3"},
			{Name: "db-password-override", Value: "false"},
			{Name: "allow-empty-password-auth", Value: "true"},
		},
		Email: vmEmail{Params: []param{
			{Name: "date-fmt", Value: "%A, %B %d %Y, %I %M %p"},
			{Name: "email-from", Value: "${voicemail_account}@${voicemail_domain}"},
		}},
	}
	if odbcDSN != "" {
		profile.Params = append(profile.Params, param{Name: "odbc-dsn", Value: odbcDSN})
	}

	doc := vmDocument{
		Type: "freeswitch/xml",
		Section: vmSection{
			Name: "configuration",
			Configuration: vmConfiguration{
				Name:        "voicemail.conf",
				Description: "Voicemail",
				Profiles:    []vmProfile{profile},
			},
		},
	}
	return marshal(doc)
}
