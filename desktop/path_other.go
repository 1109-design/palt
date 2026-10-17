//go:build !linux

package main

func pathKind(string) string { return "lan" }

func pathSpeed(string) int { return 1 }
