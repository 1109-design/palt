//go:build !linux

package main

func localStatus() DeviceStatus { return DeviceStatus{Battery: -1, Signal: -1} }
