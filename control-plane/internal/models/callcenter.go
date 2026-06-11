package models

import "time"

type CCQueue struct {
	ID                                 string            `json:"id"`
	Name                               string            `json:"name"`
	Strategy                           string            `json:"strategy"`
	MohSound                           string            `json:"moh_sound"`
	TimeBaseScore                      string            `json:"time_base_score"`
	MaxWaitTime                        int               `json:"max_wait_time"`
	MaxWaitTimeWithNoAgent             int               `json:"max_wait_time_with_no_agent"`
	MaxWaitTimeWithNoAgentTimeReached  int               `json:"max_wait_time_with_no_agent_time_reached"`
	TierRulesApply                     bool              `json:"tier_rules_apply"`
	TierRuleWaitSecond                 int               `json:"tier_rule_wait_second"`
	TierRuleWaitMultiplyLevel          bool              `json:"tier_rule_wait_multiply_level"`
	TierRuleNoAgentNoWait              bool              `json:"tier_rule_no_agent_no_wait"`
	DiscardAbandonedAfter              int               `json:"discard_abandoned_after"`
	AbandonedResumeAllowed             bool              `json:"abandoned_resume_allowed"`
	Params                             map[string]string `json:"params"`
	CreatedAt                          time.Time         `json:"created_at"`
	UpdatedAt                          time.Time         `json:"updated_at"`
}

type CCAgent struct {
	ID                string            `json:"id"`
	Name              string            `json:"name"`
	Type              string            `json:"type"`
	Contact           string            `json:"contact"`
	Status            string            `json:"status"`
	MaxNoAnswer       int               `json:"max_no_answer"`
	WrapUpTime        int               `json:"wrap_up_time"`
	RejectDelayTime   int               `json:"reject_delay_time"`
	BusyDelayTime     int               `json:"busy_delay_time"`
	NoAnswerDelayTime int               `json:"no_answer_delay_time"`
	Params            map[string]string `json:"params"`
	CreatedAt         time.Time         `json:"created_at"`
	UpdatedAt         time.Time         `json:"updated_at"`
}

type CCTier struct {
	ID        string    `json:"id"`
	Queue     string    `json:"queue"`
	Agent     string    `json:"agent"`
	Level     int       `json:"level"`
	Position  int       `json:"position"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
