//go:build !linux

package main

import (
	"errors"

	"palt/internal/wifiqr"
)

var errOnWifi = errors.New("this computer is on Wi-Fi")

func startHotspot() (wifiqr.Network, error) {
	return wifiqr.Network{}, errors.New("sharing internet is Linux-only for now")
}

func stopHotspot() error { return nil }

func hotspotActive() bool { return false }
