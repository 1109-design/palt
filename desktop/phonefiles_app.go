// phonefiles_app.go browses a paired phone's storage from the desktop:
// folders, thumbnails, download, upload, delete, new folder. It also asks
// the phone to take a photo, which arrives as a transfer and opens.
//
// Packet types (must match mobile/lib/features/phone_files.dart). Requests
// carry an id and are answered by "<type>.reply" with the same id.
//
//	files.list    desktop → phone  {path} → {path, entries: [{name, dir,
//	                               size, modified}], access}
//	files.thumb   desktop → phone  {path, size} → {data} (base64 JPEG)
//	files.get     desktop → phone  {paths} → {} then sends them as a transfer
//	files.delete  desktop → phone  {paths} → {}
//	files.mkdir   desktop → phone  {path} → {}
//	files.allow   desktop → phone  {} → {} opens "All files access" settings
//	storage.info  desktop → phone  {} → PhoneStorage
//	camera.photo  desktop → phone  {} → {} opens the camera; the photo
//	                               follows as a transfer
//
// Uploads are ordinary transfers whose offer names the folder (dest).
package main

import (
	"errors"
	"log"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"palt/internal/link"
)

// PhoneFile is one entry of a phone folder.
type PhoneFile struct {
	Name     string `json:"name"`
	Dir      bool   `json:"dir"`
	Size     int64  `json:"size"`
	Modified int64  `json:"modified"` // unix ms
}

// PhoneFolder is a phone folder's listing.
type PhoneFolder struct {
	Path    string      `json:"path"`
	Entries []PhoneFile `json:"entries"`
	// Access is false until "All files access" is allowed on the phone.
	Access bool `json:"access"`
}

// PhoneStorage is a phone's space and memory. Sizes are bytes.
type PhoneStorage struct {
	Total      int64 `json:"total"`
	Free       int64 `json:"free"`
	Categories struct {
		Images    int64 `json:"images"`
		Videos    int64 `json:"videos"`
		Audio     int64 `json:"audio"`
		Documents int64 `json:"documents"`
		Other     int64 `json:"other"`
		System    int64 `json:"system"` // apps, system and anything unindexed
	} `json:"categories"`
	Folders []struct {
		Name string `json:"name"`
		Path string `json:"path"`
		Size int64  `json:"size"`
	} `json:"folders"`
	Largest []struct {
		Path string `json:"path"`
		Size int64  `json:"size"`
	} `json:"largest"`
	Volumes []struct {
		Name      string `json:"name"`
		Path      string `json:"path"`
		Removable bool   `json:"removable"`
		Total     int64  `json:"total"`
		Free      int64  `json:"free"`
	} `json:"volumes"`
	RAM struct {
		Total     int64 `json:"total"`
		Available int64 `json:"available"`
		Low       bool  `json:"low"`
		Threshold int64 `json:"threshold"`
		Cached    int64 `json:"cached"`
		SwapTotal int64 `json:"swapTotal"`
		SwapFree  int64 `json:"swapFree"`
	} `json:"ram"`
}

const (
	filesTimeout = 20 * time.Second
	// photoWait is how long a requested photo may take to arrive.
	photoWait = 5 * time.Minute
)

var photos = struct {
	sync.Mutex
	waiting map[string]time.Time // device name → asked until
}{waiting: map[string]time.Time{}}

func (a *App) filesLink(device string) (*link.Link, error) {
	l, err := a.linkByName(device)
	if err != nil {
		return nil, err
	}
	if !l.Supports("files.list") {
		return nil, errors.New("update PALT on " + device + " to browse its files")
	}
	return l, nil
}

func (a *App) filesRequest(device, t string, body any, out any) error {
	l, err := a.filesLink(device)
	if err != nil {
		return err
	}
	p, err := l.Request(t, body, filesTimeout)
	if err != nil {
		return err
	}
	if out == nil {
		out = &struct{}{}
	}
	return smsReply(p, out)
}

// openArrivedPhoto opens a photo the user asked a device to take.
func openArrivedPhoto(sender string, savedPaths []string) {
	photos.Lock()
	until, ok := photos.waiting[sender]
	if ok && len(savedPaths) == 1 && time.Now().Before(until) {
		delete(photos.waiting, sender)
	} else {
		ok = false
	}
	photos.Unlock()
	if !ok {
		return
	}
	ext := strings.ToLower(filepath.Ext(savedPaths[0]))
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".heic" && ext != ".webp" {
		return
	}
	if err := exec.Command("xdg-open", savedPaths[0]).Start(); err != nil {
		log.Printf("[files] open photo: %v", err)
	}
}

// ─── Wails bindings ──────────────────────────────────────────────────────────

// ListPhoneFiles lists a folder on the phone ("" = its main storage).
func (a *App) ListPhoneFiles(device, path string) (PhoneFolder, error) {
	var f PhoneFolder
	err := a.filesRequest(device, "files.list", map[string]string{"path": path}, &f)
	if f.Entries == nil {
		f.Entries = []PhoneFile{}
	}
	return f, err
}

// PhoneThumbnail returns a small preview of a phone picture or video as a
// data: URL.
func (a *App) PhoneThumbnail(device, path string) (string, error) {
	var r struct {
		Data string `json:"data"`
	}
	if err := a.filesRequest(device, "files.thumb", map[string]any{"path": path, "size": 256}, &r); err != nil {
		return "", err
	}
	if r.Data == "" {
		return "", errors.New("no preview")
	}
	return "data:image/jpeg;base64," + r.Data, nil
}

// DownloadPhoneFiles has the phone send files here (Downloads/PALT).
func (a *App) DownloadPhoneFiles(device string, paths []string) error {
	return a.filesRequest(device, "files.get", map[string]any{"paths": paths}, nil)
}

// UploadToPhone picks files and sends them into folder dir on the phone.
func (a *App) UploadToPhone(device, dir string) error {
	paths, err := wailsruntime.OpenMultipleFilesDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "Send files to " + device,
	})
	if err != nil || len(paths) == 0 {
		return err
	}
	return a.UploadPathsToPhone(device, dir, paths)
}

// UploadPathsToPhone sends dropped files into folder dir on the phone.
func (a *App) UploadPathsToPhone(device, dir string, paths []string) error {
	if _, err := a.filesLink(device); err != nil {
		return err
	}
	return a.sendPathsTo("", 0, device, paths, dir)
}

// DeletePhoneFiles deletes files or folders on the phone.
func (a *App) DeletePhoneFiles(device string, paths []string) error {
	return a.filesRequest(device, "files.delete", map[string]any{"paths": paths}, nil)
}

// NewPhoneFolder makes a folder on the phone.
func (a *App) NewPhoneFolder(device, path string) error {
	return a.filesRequest(device, "files.mkdir", map[string]string{"path": path}, nil)
}

// GetPhoneStorage reads the phone's storage, what fills it, and its memory.
func (a *App) GetPhoneStorage(device string) (PhoneStorage, error) {
	var st PhoneStorage
	l, err := a.filesLink(device)
	if err != nil {
		return st, err
	}
	if !l.Supports("storage.info") {
		return st, errors.New("update PALT on " + device + " to see its storage")
	}
	p, err := l.Request("storage.info", map[string]any{}, 45*time.Second)
	if err != nil {
		return st, err
	}
	err = smsReply(p, &st)
	return st, err
}

// AllowPhoneFiles opens "All files access" for PALT on the phone.
func (a *App) AllowPhoneFiles(device string) error {
	return a.filesRequest(device, "files.allow", map[string]any{}, nil)
}

// TakePhoto opens the phone's camera; the photo opens here when it arrives.
func (a *App) TakePhoto(device string) error {
	l, err := a.linkByName(device)
	if err != nil {
		return err
	}
	if !l.Supports("camera.photo") {
		return errors.New("update PALT on " + device + " to take photos")
	}
	photos.Lock()
	photos.waiting[device] = time.Now().Add(photoWait)
	photos.Unlock()
	p, err := l.Request("camera.photo", map[string]any{}, filesTimeout)
	if err != nil {
		return err
	}
	return smsReply(p, &struct{}{})
}
