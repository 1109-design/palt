package thumbs

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var videoExts = map[string]bool{
	".mp4": true, ".mov": true, ".m4v": true, ".webm": true, ".mkv": true, ".avi": true, ".3gp": true,
}

// ErrNoFFmpeg means video previews need ffmpeg, which isn't installed.
var ErrNoFFmpeg = errors.New("ffmpeg is not installed")

// IsVideo reports whether name has a video extension we can preview.
func IsVideo(name string) bool {
	return videoExts[strings.ToLower(filepath.Ext(name))]
}

var durationRe = regexp.MustCompile(`Duration: (\d+):(\d\d):(\d\d(?:\.\d+)?)`)

// Video returns a JPEG frame from the video at path and its length in
// seconds (0 if unknown), using cacheDir when set. It needs ffmpeg.
func Video(path, cacheDir string) ([]byte, float64, error) {
	if !IsVideo(path) {
		return nil, 0, ErrNotImage
	}
	st, err := os.Stat(path)
	if err != nil {
		return nil, 0, err
	}

	var jpgPath, durPath string
	if cacheDir != "" {
		h := sha1.Sum(fmt.Appendf(nil, "video|%s|%d|%d|%d", path, st.Size(), st.ModTime().UnixNano(), MaxSide))
		base := filepath.Join(cacheDir, hex.EncodeToString(h[:]))
		jpgPath, durPath = base+".jpg", base+".dur"
		if jpg, err := os.ReadFile(jpgPath); err == nil {
			d, _ := os.ReadFile(durPath)
			secs, _ := strconv.ParseFloat(strings.TrimSpace(string(d)), 64)
			return jpg, secs, nil
		}
	}

	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		return nil, 0, ErrNoFFmpeg
	}
	// Grab a frame 1 s in (skips black first frames); short clips fall back to 0.
	jpg, secs, err := grabFrame(ffmpeg, path, "1")
	if err == nil && len(jpg) == 0 {
		jpg, secs, err = grabFrame(ffmpeg, path, "0")
	}
	if err != nil {
		return nil, 0, err
	}
	if len(jpg) == 0 {
		return nil, 0, fmt.Errorf("no frame in %s", filepath.Base(path))
	}
	if jpgPath != "" && os.MkdirAll(cacheDir, 0o700) == nil {
		_ = os.WriteFile(jpgPath, jpg, 0o600)
		_ = os.WriteFile(durPath, []byte(strconv.FormatFloat(secs, 'f', 2, 64)), 0o600)
	}
	return jpg, secs, nil
}

func grabFrame(ffmpeg, path, at string) ([]byte, float64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	scale := fmt.Sprintf("scale='if(gt(iw,ih),min(%d,iw),-2)':'if(gt(iw,ih),-2,min(%d,ih))'", MaxSide, MaxSide)
	cmd := exec.CommandContext(ctx, ffmpeg,
		"-hide_banner", "-nostdin", "-loglevel", "info",
		"-ss", at, "-i", path,
		"-frames:v", "1", "-vf", scale,
		"-f", "image2pipe", "-c:v", "mjpeg", "-q:v", "4", "pipe:1")
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	runErr := cmd.Run()

	secs := 0.0
	if m := durationRe.FindStringSubmatch(errOut.String()); m != nil {
		h, _ := strconv.Atoi(m[1])
		mi, _ := strconv.Atoi(m[2])
		s, _ := strconv.ParseFloat(m[3], 64)
		secs = float64(h*3600+mi*60) + s
	}
	if runErr != nil && out.Len() == 0 {
		return nil, secs, fmt.Errorf("ffmpeg could not read %s", filepath.Base(path))
	}
	return out.Bytes(), secs, nil
}
