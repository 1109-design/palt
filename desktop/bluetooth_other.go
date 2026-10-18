//go:build !linux

package main

import "errors"

func localBluetooth() BluetoothState { return BluetoothState{Devices: []BluetoothDevice{}} }
func setLocalBluetooth(bool) error   { return errors.New("not supported here") }
