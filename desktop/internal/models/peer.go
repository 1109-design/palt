// Package models defines the shared data structures for the PALT application.
package models

// Peer represents a discovered device on the local network.
// It is designed to be JSON-serializable so Wails can send it to the React frontend.
type Peer struct {
	// ID is a stable, unique key derived from the mDNS instance name.
	ID string `json:"id"`

	// DeviceName is the human-readable hostname of the device.
	DeviceName string `json:"deviceName"`

	// IPAddress is the resolved IPv4 address of the peer.
	IPAddress string `json:"ipAddress"`

	// Port is the TCP port the peer's PALT service is listening on.
	Port int `json:"port"`

	// OS describes the operating system of the peer (e.g., "linux", "android", "windows").
	OS string `json:"os"`

	// Via is how this peer is reachable: "direct" (same LAN) or "relay".
	Via string `json:"via"`

	// DeviceID is the peer's stable PALT ID ("" for builds before 2.0).
	DeviceID string `json:"deviceId"`

	// Paired is true while an authenticated link to the peer is up.
	Paired bool `json:"paired"`
}
