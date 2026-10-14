package models

import "time"

type HistoryFile struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
	Path string `json:"path,omitempty"` // where the file lives on this machine
}

type HistoryEntry struct {
	ID             string        `json:"id"`
	PartnerName    string        `json:"partnerName"`
	Files          []HistoryFile `json:"files"`
	TotalSize      int64         `json:"totalSize"`
	Direction      string        `json:"direction"` // "incoming" or "outgoing"
	Timestamp      time.Time     `json:"timestamp"`
	Status         string        `json:"status"`           // "completed" or "error"
	ErrorMessage   string        `json:"errorMessage"`     // optional
	DurationMillis int64         `json:"durationMillis"`   // how long the transfer took
	Kind           string        `json:"kind,omitempty"`   // "text" for messages; empty means files
	Text           string        `json:"text,omitempty"`   // message body when Kind == "text"
	Copied         bool          `json:"copied,omitempty"` // incoming text was auto-copied to the clipboard
	Route          string        `json:"route,omitempty"`  // "direct" or "relay"; empty on older entries
}
