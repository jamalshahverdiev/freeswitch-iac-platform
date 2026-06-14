package renderer

import (
	"encoding/xml"
	"sort"

	"github.com/jamalshahverdiev/freeswitch-iac-platform/control-plane/internal/models"
)

type dpDocument struct {
	XMLName  xml.Name    `xml:"document"`
	Type     string      `xml:"type,attr"`
	Section  dpSection   `xml:"section"`
}

type dpSection struct {
	Name     string       `xml:"name,attr"`
	Contexts []dpContext  `xml:"context"`
}

type dpContext struct {
	Name       string        `xml:"name,attr"`
	Extensions []dpExtension `xml:"extension"`
}

type dpExtension struct {
	Name       string        `xml:"name,attr"`
	Conditions []dpCondition `xml:"condition"`
}

type dpCondition struct {
	Field      string `xml:"field,attr,omitempty"`
	Expression string `xml:"expression,attr,omitempty"`
	// FreeSWITCH time-of-day attributes (emitted only when set).
	Wday      string     `xml:"wday,attr,omitempty"`
	Mday      string     `xml:"mday,attr,omitempty"`
	Mon       string     `xml:"mon,attr,omitempty"`
	Mweek     string     `xml:"mweek,attr,omitempty"`
	Week      string     `xml:"week,attr,omitempty"`
	Hour      string     `xml:"hour,attr,omitempty"`
	Minute    string     `xml:"minute,attr,omitempty"`
	TimeOfDay string     `xml:"time-of-day,attr,omitempty"`
	DateTime  string     `xml:"date-time,attr,omitempty"`
	Actions   []dpAction `xml:"action"`
}

// applyTimeAttrs copies supported FreeSWITCH time attributes onto the
// condition. Unknown keys are ignored (never reach the XML).
func applyTimeAttrs(c *dpCondition, attrs map[string]string) {
	for k, v := range attrs {
		switch k {
		case "wday":
			c.Wday = v
		case "mday":
			c.Mday = v
		case "mon":
			c.Mon = v
		case "mweek":
			c.Mweek = v
		case "week":
			c.Week = v
		case "hour":
			c.Hour = v
		case "minute":
			c.Minute = v
		case "time-of-day":
			c.TimeOfDay = v
		case "date-time":
			c.DateTime = v
		}
	}
}

type dpAction struct {
	Application string `xml:"application,attr"`
	Data        string `xml:"data,attr,omitempty"`
}

// RenderDialplan builds FreeSWITCH dialplan XML grouped by context.
// Disabled extensions are skipped. Optionally filter to a single context.
func RenderDialplan(exts []models.DialplanExtension, contextFilter string) ([]byte, error) {
	byContext := map[string][]dpExtension{}
	for _, e := range exts {
		if !e.Enabled {
			continue
		}
		if contextFilter != "" && e.Context != contextFilter {
			continue
		}
		ext := dpExtension{Name: e.Name}
		for _, c := range e.Conditions {
			cond := dpCondition{Field: c.Field, Expression: c.Expression}
			applyTimeAttrs(&cond, c.TimeAttrs)
			for _, a := range c.Actions {
				cond.Actions = append(cond.Actions, dpAction{Application: a.Application, Data: a.Data})
			}
			ext.Conditions = append(ext.Conditions, cond)
		}
		byContext[e.Context] = append(byContext[e.Context], ext)
	}

	names := make([]string, 0, len(byContext))
	for name := range byContext {
		names = append(names, name)
	}
	sort.Strings(names)

	doc := dpDocument{Type: "freeswitch/xml", Section: dpSection{Name: "dialplan"}}
	for _, name := range names {
		doc.Section.Contexts = append(doc.Section.Contexts, dpContext{
			Name:       name,
			Extensions: byContext[name],
		})
	}
	return marshal(doc)
}
