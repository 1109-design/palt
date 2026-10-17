//go:build bindings

package main

// generatingBindings is true in the build Wails runs to generate the
// frontend bindings; it must not hand off to a running PALT and exit.
const generatingBindings = true
