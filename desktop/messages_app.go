// messages_app.go shows a paired phone's SMS and contacts: conversations
// and threads on request, new texts as notifications (with a one-click copy
// for 2FA codes), and sending texts through the phone. No MMS.
//
// Packet types (must match mobile/lib/features/messages.dart). Requests
// carry an id and are answered by "<type>.reply" with the same id.
//
//	sms.conversations  computer → phone  {limit} → {threads}
//	sms.thread         computer → phone  {threadId, before, limit} → {messages}
//	sms.send           computer → phone  {address, body} → {ok, error}
//	sms.received       phone → computer  SMSMessage
//	sms.changed        phone → computer  {}
//	contacts           computer → phone  {version} → {version, parts}
//	contacts.part      phone → computer  {version, part, parts, contacts}
//	contacts.changed   phone → computer  {}
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"palt/internal/link"
)

const smsRequestTimeout = 20 * time.Second

// SMSMessage is one text, or a conversation's newest text (with Unread).
type SMSMessage struct {
	ID       int64  `json:"id"`
	ThreadID int64  `json:"threadId"`
	Address  string `json:"address"`
	Body     string `json:"body"`
	Date     int64  `json:"date"` // unix ms
	Type     int    `json:"type"` // 1 received, 2 sent, 4-6 sending/failed/queued
	Read     bool   `json:"read"`
	Name     string `json:"name"`             // contact name, "" unknown
	Unread   int    `json:"unread,omitempty"` // conversations only
}

// Contact is one of the phone's contacts with a number.
type Contact struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Numbers []string `json:"numbers"`
	Photo   string   `json:"photo"` // data: URL or ""
}

type contactBook struct {
	Version  string    `json:"version"`
	Contacts []Contact `json:"contacts"`
}

type messagesState struct {
	mu       sync.Mutex
	incoming map[string]*contactBook // by device ID: parts arriving
	books    map[string]*contactBook // by device ID
}

func (a *App) setupMessages() {
	a.messages.incoming = map[string]*contactBook{}
	a.messages.books = map[string]*contactBook{}

	a.links.Handle("sms.received", func(l *link.Link, p link.Packet) {
		var m SMSMessage
		if p.Decode(&m) != nil {
			return
		}
		a.onSMS(l.Peer.Name, m)
	})
	a.links.Handle("sms.changed", func(l *link.Link, p link.Packet) {
		if a.ctx != nil {
			wailsruntime.EventsEmit(a.ctx, "sms_changed", l.Peer.Name)
		}
	})
	a.links.Handle("contacts.part", a.onContactsPart)
	a.links.Handle("contacts.changed", func(l *link.Link, p link.Packet) { go a.syncContacts(l) })

	prevUp := a.links.OnUp
	a.links.OnUp = func(l *link.Link) {
		if prevUp != nil {
			prevUp(l)
		}
		if l.Supports("contacts") {
			go a.syncContacts(l)
		}
	}
}

// ─── Contacts ────────────────────────────────────────────────────────────────

func contactsFile(deviceID string) string {
	dir, err := configDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "devices", deviceID, "contacts.json")
}

func (a *App) contactBook(deviceID string) *contactBook {
	a.messages.mu.Lock()
	defer a.messages.mu.Unlock()
	if b := a.messages.books[deviceID]; b != nil {
		return b
	}
	b := &contactBook{}
	if f := contactsFile(deviceID); f != "" {
		if data, err := os.ReadFile(f); err == nil {
			json.Unmarshal(data, b)
		}
	}
	a.messages.books[deviceID] = b
	return b
}

// syncContacts asks the phone for its contacts unless they haven't changed.
func (a *App) syncContacts(l *link.Link) {
	have := a.contactBook(l.Peer.ID).Version
	p, err := l.Request("contacts", map[string]string{"version": have}, smsRequestTimeout)
	if err != nil {
		log.Printf("[contacts] %s: %v", l.Peer.Name, err)
		return
	}
	var r struct {
		Version string `json:"version"`
		Parts   int    `json:"parts"`
		Error   string `json:"error"`
	}
	p.Decode(&r)
	if r.Error != "" {
		log.Printf("[contacts] %s: %s", l.Peer.Name, r.Error)
	} else if r.Parts > 0 {
		log.Printf("[contacts] %s: syncing (%d parts)", l.Peer.Name, r.Parts)
	}
}

func (a *App) onContactsPart(l *link.Link, p link.Packet) {
	var b struct {
		Version  string    `json:"version"`
		Part     int       `json:"part"`
		Parts    int       `json:"parts"`
		Contacts []Contact `json:"contacts"`
	}
	if p.Decode(&b) != nil {
		return
	}
	a.messages.mu.Lock()
	in := a.messages.incoming[l.Peer.ID]
	if b.Part == 0 || in == nil || in.Version != b.Version {
		in = &contactBook{Version: b.Version}
		a.messages.incoming[l.Peer.ID] = in
	}
	in.Contacts = append(in.Contacts, b.Contacts...)
	done := b.Part == b.Parts-1
	if done {
		delete(a.messages.incoming, l.Peer.ID)
		a.messages.books[l.Peer.ID] = in
	}
	a.messages.mu.Unlock()
	if !done {
		return
	}
	log.Printf("[contacts] %s: %d contacts", l.Peer.Name, len(in.Contacts))
	if f := contactsFile(l.Peer.ID); f != "" {
		os.MkdirAll(filepath.Dir(f), 0o700)
		if data, err := json.Marshal(in); err == nil {
			os.WriteFile(f, data, 0o600)
		}
	}
	if a.ctx != nil {
		wailsruntime.EventsEmit(a.ctx, "contacts_changed", l.Peer.Name)
	}
}

// numberKey compares phone numbers loosely: the last 9 digits.
func numberKey(n string) string {
	d := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, n)
	if len(d) > 9 {
		d = d[len(d)-9:]
	}
	return d
}

// ─── New texts ───────────────────────────────────────────────────────────────

// otpRe finds a one-time code: 4-8 digits (maybe split by a space or dash)
// in a text that talks about a code.
var (
	otpHint = regexp.MustCompile(`(?i)\b(code|otp|pin|verif|password|passcode|token|kodi)`)
	otpRe   = regexp.MustCompile(`\b(\d{3}[ -]?\d{3}|\d{4,8})\b`)
)

func oneTimeCode(body string) string {
	if !otpHint.MatchString(body) {
		return ""
	}
	m := otpRe.FindString(body)
	return strings.NewReplacer(" ", "", "-", "").Replace(m)
}

func (a *App) onSMS(device string, m SMSMessage) {
	from := m.Name
	if from == "" {
		from = m.Address
	}
	log.Printf("[sms] New text on %s from %s", device, from)
	if a.ctx != nil {
		wailsruntime.EventsEmit(a.ctx, "sms_received", map[string]any{"device": device, "message": m})
	}
	if !a.setting(device, "notifications") {
		return
	}
	actions := []NotifyAction{{Key: "default", Label: "Open"}, {Key: "reply", Label: "Reply"}}
	code := oneTimeCode(m.Body)
	if code != "" {
		actions = append([]NotifyAction{{Key: "copy", Label: "Copy " + code}}, actions...)
	}
	notify(Notification{
		Title:    from,
		Body:     m.Body,
		Icon:     "mail-message-new",
		Category: "im.received",
		Actions:  actions,
		OnAction: func(key string) {
			if key == "copy" {
				a.setClipboard(code)
				return
			}
			a.showWindow()
			wailsruntime.EventsEmit(a.ctx, "open_messages", map[string]any{
				"device": device, "threadId": m.ThreadID, "address": m.Address, "reply": key == "reply",
			})
		},
	})
}

// ─── Wails bindings ──────────────────────────────────────────────────────────

func (a *App) messagesLink(device string) (*link.Link, error) {
	l, err := a.linkByName(device)
	if err != nil {
		return nil, err
	}
	if !l.Supports("sms.conversations") {
		return nil, fmt.Errorf("update PALT on %s to see its messages", device)
	}
	return l, nil
}

func smsReply(p link.Packet, out any) error {
	var e struct {
		Error string `json:"error"`
	}
	p.Decode(&e)
	if e.Error != "" {
		return errors.New(e.Error)
	}
	return p.Decode(out)
}

// GetConversations lists a phone's SMS conversations, newest first.
func (a *App) GetConversations(device string) ([]SMSMessage, error) {
	l, err := a.messagesLink(device)
	if err != nil {
		return nil, err
	}
	p, err := l.Request("sms.conversations", map[string]int{"limit": 200}, smsRequestTimeout)
	if err != nil {
		return nil, err
	}
	var r struct {
		Threads []SMSMessage `json:"threads"`
	}
	if err := smsReply(p, &r); err != nil {
		return nil, err
	}
	if r.Threads == nil {
		r.Threads = []SMSMessage{}
	}
	return r.Threads, nil
}

// GetThread returns up to 60 texts of a conversation older than before
// (unix ms, 0 = newest), newest first.
func (a *App) GetThread(device string, threadID, before int64) ([]SMSMessage, error) {
	l, err := a.messagesLink(device)
	if err != nil {
		return nil, err
	}
	p, err := l.Request("sms.thread", map[string]int64{"threadId": threadID, "before": before, "limit": 60}, smsRequestTimeout)
	if err != nil {
		return nil, err
	}
	var r struct {
		Messages []SMSMessage `json:"messages"`
	}
	if err := smsReply(p, &r); err != nil {
		return nil, err
	}
	if r.Messages == nil {
		r.Messages = []SMSMessage{}
	}
	return r.Messages, nil
}

// SendSMS sends a text from the phone.
func (a *App) SendSMS(device, address, body string) error {
	if strings.TrimSpace(address) == "" || strings.TrimSpace(body) == "" {
		return errors.New("nothing to send")
	}
	l, err := a.messagesLink(device)
	if err != nil {
		return err
	}
	p, err := l.Request("sms.send", map[string]string{"address": address, "body": body}, smsRequestTimeout)
	if err != nil {
		return err
	}
	var r struct {
		OK bool `json:"ok"`
	}
	if err := smsReply(p, &r); err != nil {
		return err
	}
	if !r.OK {
		return errors.New("the phone couldn't send it")
	}
	return nil
}

// GetContacts returns a paired phone's contacts (from the last sync).
func (a *App) GetContacts(device string) []Contact {
	l := a.links.GetByName(device)
	if l == nil {
		return []Contact{}
	}
	b := a.contactBook(l.Peer.ID)
	if b.Contacts == nil {
		return []Contact{}
	}
	return b.Contacts
}

// ContactPhotos maps numberKey → photo for a phone's contacts with photos.
func (a *App) ContactPhotos(device string) map[string]string {
	out := map[string]string{}
	for _, c := range a.GetContacts(device) {
		if c.Photo == "" {
			continue
		}
		for _, n := range c.Numbers {
			out[numberKey(n)] = c.Photo
		}
	}
	return out
}
