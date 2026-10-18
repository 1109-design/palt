// commands_app.go lets paired phones run named commands on this computer:
// Lock, Suspend and Mute built in, plus shell commands the user adds here
// (~/.config/palt/commands.json). Phones only see names and ids; they can
// never send a command line of their own.
//
// Packet types (must match mobile/lib/features/commands.dart). Requests
// carry an id and are answered by "<type>.reply" with the same id.
//
//	commands.list  phone → desktop  {} → {commands: [{id, name}]}
//	commands.run   phone → desktop  {id} → {output?, error?}
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"palt/internal/link"
)

// Command is one thing a phone can run here.
type Command struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Command string `json:"command"`
	BuiltIn bool   `json:"builtIn"`
}

const (
	commandTimeout   = 60 * time.Second
	commandOutputMax = 4000
)

var builtInCommands = []Command{
	{ID: "lock", Name: "Lock screen", BuiltIn: true},
	{ID: "suspend", Name: "Suspend", BuiltIn: true},
	{ID: "mute", Name: "Mute / unmute", BuiltIn: true},
}

var commandsMu sync.Mutex

func commandsFile() string {
	dir, err := configDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "commands.json")
}

func loadCommands() []Command {
	var cmds []Command
	if b, err := os.ReadFile(commandsFile()); err == nil {
		json.Unmarshal(b, &cmds)
	}
	return cmds
}

func saveCommands(cmds []Command) error {
	b, _ := json.MarshalIndent(cmds, "", "  ")
	return os.WriteFile(commandsFile(), b, 0o600)
}

func (a *App) setupCommands() {
	a.links.Handle("commands.list", func(l *link.Link, p link.Packet) {
		list := []map[string]string{}
		for _, c := range a.GetCommands() {
			list = append(list, map[string]string{"id": c.ID, "name": c.Name})
		}
		l.SendID("commands.list.reply", p.ID, map[string]any{"commands": list})
	})
	a.links.Handle("commands.run", func(l *link.Link, p link.Packet) {
		var b struct {
			ID string `json:"id"`
		}
		p.Decode(&b)
		go func() {
			out, err := a.runCommand(b.ID)
			reply := map[string]string{"output": out}
			if err != nil {
				reply["error"] = err.Error()
			}
			log.Printf("[commands] %s ran %q: %v", l.Peer.Name, b.ID, err)
			l.SendID("commands.run.reply", p.ID, reply)
		}()
	})
}

func (a *App) runCommand(id string) (string, error) {
	switch id {
	case "lock":
		return "", lockScreen()
	case "suspend":
		return "", exec.Command("systemctl", "suspend").Run()
	case "mute":
		return "", setSystemVolume("mute", 0)
	}
	for _, c := range a.GetCommands() {
		if c.ID == id && !c.BuiltIn {
			ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
			defer cancel()
			out, err := exec.CommandContext(ctx, "sh", "-c", c.Command).CombinedOutput()
			s := strings.TrimSpace(string(out))
			if len(s) > commandOutputMax {
				s = s[len(s)-commandOutputMax:]
			}
			if ctx.Err() != nil {
				err = errors.New("took too long and was stopped")
			}
			return s, err
		}
	}
	return "", errors.New("that command was removed on the computer")
}

// lockScreen locks this session, through GNOME first.
func lockScreen() error {
	err := exec.Command("dbus-send", "--session", "--dest=org.gnome.ScreenSaver", "--type=method_call",
		"/org/gnome/ScreenSaver", "org.gnome.ScreenSaver.Lock").Run()
	if err != nil {
		err = exec.Command("loginctl", "lock-session").Run()
	}
	return err
}

// ─── Wails bindings ──────────────────────────────────────────────────────────

// GetCommands lists the built-in commands, then the user's.
func (a *App) GetCommands() []Command {
	commandsMu.Lock()
	defer commandsMu.Unlock()
	return append(append([]Command{}, builtInCommands...), loadCommands()...)
}

// SaveCommand adds a command (empty id) or changes one.
func (a *App) SaveCommand(c Command) error {
	c.Name, c.Command = strings.TrimSpace(c.Name), strings.TrimSpace(c.Command)
	if c.Name == "" || c.Command == "" {
		return errors.New("give it a name and a command")
	}
	commandsMu.Lock()
	defer commandsMu.Unlock()
	cmds := loadCommands()
	c.BuiltIn = false
	if c.ID == "" {
		c.ID = fmt.Sprintf("c%d", time.Now().UnixNano())
		cmds = append(cmds, c)
	} else {
		found := false
		for i := range cmds {
			if cmds[i].ID == c.ID {
				cmds[i], found = c, true
			}
		}
		if !found {
			return errors.New("no such command")
		}
	}
	return saveCommands(cmds)
}

// DeleteCommand removes one of the user's commands.
func (a *App) DeleteCommand(id string) error {
	commandsMu.Lock()
	defer commandsMu.Unlock()
	cmds := loadCommands()
	out := cmds[:0]
	for _, c := range cmds {
		if c.ID != id {
			out = append(out, c)
		}
	}
	return saveCommands(out)
}
