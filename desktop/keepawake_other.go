//go:build !linux

package main

import "errors"

func setIdleInhibit(bool) error { return errors.New("not supported") }
