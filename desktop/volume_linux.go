// volume_linux.go reads and sets this computer's output volume through
// PipeWire/PulseAudio (pactl), for the media remote on paired phones.
package main

import (
	"bufio"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var percentRe = regexp.MustCompile(`(\d+)%`)

// systemVolume returns the default output's volume (0-100, -1 unknown).
func systemVolume() SystemVolume {
	v := SystemVolume{Level: -1}
	out, err := exec.Command("pactl", "get-sink-volume", "@DEFAULT_SINK@").Output()
	if err != nil {
		return v
	}
	if m := percentRe.FindStringSubmatch(string(out)); m != nil {
		v.Level, _ = strconv.Atoi(m[1])
	}
	out, _ = exec.Command("pactl", "get-sink-mute", "@DEFAULT_SINK@").Output()
	v.Muted = strings.Contains(string(out), "yes")
	return v
}

// setSystemVolume sets the level (0-100), or mute on/off/toggle.
func setSystemVolume(action string, level int) error {
	switch action {
	case "level":
		level = max(0, min(level, 150))
		return exec.Command("pactl", "set-sink-volume", "@DEFAULT_SINK@", fmt.Sprintf("%d%%", level)).Run()
	case "mute":
		return exec.Command("pactl", "set-sink-mute", "@DEFAULT_SINK@", "toggle").Run()
	}
	return errUnknownAction
}

// watchVolume calls onChange when an output's volume or mute changes.
func watchVolume(onChange func()) {
	go func() {
		for {
			cmd := exec.Command("pactl", "subscribe")
			out, err := cmd.StdoutPipe()
			if err == nil && cmd.Start() == nil {
				s := bufio.NewScanner(out)
				for s.Scan() {
					if strings.Contains(s.Text(), "on sink") || strings.Contains(s.Text(), "on server") {
						onChange()
					}
				}
				cmd.Wait()
			}
			time.Sleep(10 * time.Second) // audio server restarted; follow again
		}
	}()
}
