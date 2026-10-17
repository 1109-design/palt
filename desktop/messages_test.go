package main

import "testing"

func TestOneTimeCode(t *testing.T) {
	for in, want := range map[string]string{
		"Your verification code is 482913":    "482913",
		"Use 123-456 to log in. Code expires": "123456",
		"Meet at 1530 tomorrow":               "",
		"Your OTP: 5521. Do not share":        "5521",
	} {
		if got := oneTimeCode(in); got != want {
			t.Errorf("oneTimeCode(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNumberKey(t *testing.T) {
	if numberKey("+263 77 123 4567") != numberKey("0771234567") {
		t.Error("local and international forms of one number differ")
	}
}
