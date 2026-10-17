//go:build !linux

package main

func pausePlaying() []string { return nil }

func resumePlayers([]string) {}

func listPlayers() []MediaPlayer { return []MediaPlayer{} }

func playerAction(string, string, float64) error { return errUnknownPlayer }

func watchPlayers(func()) {}
