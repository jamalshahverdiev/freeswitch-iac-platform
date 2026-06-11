package renderer

import (
	"encoding/xml"
	"strconv"

	"github.com/jamalshahverdiev/freeswitch-iac-platform/control-plane/internal/models"
)

type ccDocument struct {
	XMLName xml.Name  `xml:"document"`
	Type    string    `xml:"type,attr"`
	Section ccSection `xml:"section"`
}

type ccSection struct {
	Name          string          `xml:"name,attr"`
	Configuration ccConfiguration `xml:"configuration"`
}

type ccConfiguration struct {
	Name        string    `xml:"name,attr"`
	Description string    `xml:"description,attr"`
	Settings    ccParams  `xml:"settings"`
	Queues      ccQueues  `xml:"queues"`
	Agents      []ccAgent `xml:"agents>agent"`
	Tiers       []ccTier  `xml:"tiers>tier"`
}

type ccParams struct {
	Params []param `xml:"param"`
}

type ccQueues struct {
	Queues []ccQueue `xml:"queue"`
}

type ccQueue struct {
	Name   string  `xml:"name,attr"`
	Params []param `xml:"param"`
}

type ccAgent struct {
	Name              string `xml:"name,attr"`
	Type              string `xml:"type,attr"`
	Contact           string `xml:"contact,attr"`
	Status            string `xml:"status,attr"`
	MaxNoAnswer       int    `xml:"max-no-answer,attr"`
	WrapUpTime        int    `xml:"wrap-up-time,attr"`
	RejectDelayTime   int    `xml:"reject-delay-time,attr"`
	BusyDelayTime     int    `xml:"busy-delay-time,attr"`
	NoAnswerDelayTime int    `xml:"no-answer-delay-time,attr"`
}

type ccTier struct {
	Agent    string `xml:"agent,attr"`
	Queue    string `xml:"queue,attr"`
	Level    int    `xml:"level,attr"`
	Position int    `xml:"position,attr"`
}

// RenderCallcenter builds callcenter.conf from desired state. odbcDSN points
// mod_callcenter's RUNTIME tables at PostgreSQL ("dsn:user:pass"); empty means
// the param is omitted (module would fall back to sqlite — we always set it).
func RenderCallcenter(queues []models.CCQueue, agents []models.CCAgent, tiers []models.CCTier, odbcDSN string) ([]byte, error) {
	cfg := ccConfiguration{
		Name:        "callcenter.conf",
		Description: "CallCenter",
	}
	if odbcDSN != "" {
		cfg.Settings.Params = append(cfg.Settings.Params, param{Name: "odbc-dsn", Value: odbcDSN})
	}

	for _, q := range queues {
		params := map[string]string{
			"strategy":                "" + q.Strategy,
			"moh-sound":               q.MohSound,
			"time-base-score":         q.TimeBaseScore,
			"max-wait-time":           strconv.Itoa(q.MaxWaitTime),
			"max-wait-time-with-no-agent":              strconv.Itoa(q.MaxWaitTimeWithNoAgent),
			"max-wait-time-with-no-agent-time-reached": strconv.Itoa(q.MaxWaitTimeWithNoAgentTimeReached),
			"tier-rules-apply":              strconv.FormatBool(q.TierRulesApply),
			"tier-rule-wait-second":         strconv.Itoa(q.TierRuleWaitSecond),
			"tier-rule-wait-multiply-level": strconv.FormatBool(q.TierRuleWaitMultiplyLevel),
			"tier-rule-no-agent-no-wait":    strconv.FormatBool(q.TierRuleNoAgentNoWait),
			"discard-abandoned-after":       strconv.Itoa(q.DiscardAbandonedAfter),
			"abandoned-resume-allowed":      strconv.FormatBool(q.AbandonedResumeAllowed),
		}
		for k, v := range q.Params {
			params[k] = v
		}
		cfg.Queues.Queues = append(cfg.Queues.Queues, ccQueue{
			Name:   q.Name,
			Params: sortedParams(params),
		})
	}

	for _, a := range agents {
		cfg.Agents = append(cfg.Agents, ccAgent{
			Name:              a.Name,
			Type:              a.Type,
			Contact:           a.Contact,
			Status:            a.Status,
			MaxNoAnswer:       a.MaxNoAnswer,
			WrapUpTime:        a.WrapUpTime,
			RejectDelayTime:   a.RejectDelayTime,
			BusyDelayTime:     a.BusyDelayTime,
			NoAnswerDelayTime: a.NoAnswerDelayTime,
		})
	}

	for _, t := range tiers {
		cfg.Tiers = append(cfg.Tiers, ccTier{
			Agent:    t.Agent,
			Queue:    t.Queue,
			Level:    t.Level,
			Position: t.Position,
		})
	}

	doc := ccDocument{
		Type:    "freeswitch/xml",
		Section: ccSection{Name: "configuration", Configuration: cfg},
	}
	return marshal(doc)
}
