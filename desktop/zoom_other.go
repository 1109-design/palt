//go:build !linux || !webkit2_41

package main

// setNativeZoom: no native page zoom here; the page falls back to CSS zoom.
func setNativeZoom(z float64) bool { return false }
