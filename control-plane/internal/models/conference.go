package models

import "time"

type ConferenceProfile struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	Rate            int               `json:"rate"`
	IntervalMs      int               `json:"interval_ms"`
	EnergyLevel     int               `json:"energy_level"`
	ComfortNoise    bool              `json:"comfort_noise"`
	MohSound        string            `json:"moh_sound"`
	VideoMode       string            `json:"video_mode"`
	VideoLayout     string            `json:"video_layout"`
	VideoCanvasSize string            `json:"video_canvas_size"`
	VideoFPS        int               `json:"video_fps"`
	AutoRecord      string            `json:"auto_record"`
	Params          map[string]string `json:"params"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

type ConferenceRoom struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Number     string    `json:"number"`
	Domain     string    `json:"domain"`
	Context    string    `json:"context"`
	Profile    string    `json:"profile"`
	Pin        string    `json:"pin"`
	MaxMembers int       `json:"max_members"`
	Priority   int       `json:"priority"`
	Enabled    bool      `json:"enabled"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
