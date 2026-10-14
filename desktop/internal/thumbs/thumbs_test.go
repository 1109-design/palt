package thumbs

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func testImage(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 0, 255})
		}
	}
	return img
}

func decode(t *testing.T, b []byte) image.Image {
	t.Helper()
	img, err := jpeg.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	return img
}

func TestScalesPNG(t *testing.T) {
	var src bytes.Buffer
	png.Encode(&src, testImage(1200, 600))
	out, err := Make(src.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if b := decode(t, out).Bounds(); b.Dx() != 480 || b.Dy() != 240 {
		t.Fatalf("size %v", b)
	}
}

// withOrientation inserts an EXIF APP1 segment with the given orientation.
func withOrientation(jpg []byte, o uint16) []byte {
	tiff := []byte("MM\x00\x2a\x00\x00\x00\x08") // big endian, IFD at 8
	ifd := make([]byte, 2+12+4)
	binary.BigEndian.PutUint16(ifd[0:], 1)
	binary.BigEndian.PutUint16(ifd[2:], 0x0112)
	binary.BigEndian.PutUint16(ifd[4:], 3) // SHORT
	binary.BigEndian.PutUint32(ifd[6:], 1)
	binary.BigEndian.PutUint16(ifd[10:], o)
	payload := append([]byte("Exif\x00\x00"), append(tiff, ifd...)...)
	seg := []byte{0xFF, 0xE1, 0, 0}
	binary.BigEndian.PutUint16(seg[2:], uint16(len(payload)+2))
	seg = append(seg, payload...)
	return append(append([]byte{0xFF, 0xD8}, seg...), jpg[2:]...)
}

func TestAppliesExifRotation(t *testing.T) {
	var src bytes.Buffer
	jpeg.Encode(&src, testImage(400, 200), nil)
	out, err := Make(withOrientation(src.Bytes(), 6))
	if err != nil {
		t.Fatal(err)
	}
	if b := decode(t, out).Bounds(); b.Dx() != 200 || b.Dy() != 400 {
		t.Fatalf("rotated size %v, want 200x400", b)
	}
}

func TestGetCachesAndRejects(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "pic.png")
	var src bytes.Buffer
	png.Encode(&src, testImage(50, 50))
	os.WriteFile(p, src.Bytes(), 0o644)

	cache := filepath.Join(dir, "cache")
	a, err := Get(p, cache)
	if err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(cache)
	if len(entries) != 1 {
		t.Fatalf("cache entries = %d", len(entries))
	}
	b, _ := Get(p, cache)
	if !bytes.Equal(a, b) {
		t.Fatal("cached preview differs")
	}

	txt := filepath.Join(dir, "notes.txt")
	os.WriteFile(txt, []byte("hi"), 0o644)
	if _, err := Get(txt, cache); err != ErrNotImage {
		t.Fatalf("txt: %v", err)
	}
	fake := filepath.Join(dir, "fake.jpg")
	os.WriteFile(fake, []byte("not really"), 0o644)
	if _, err := Get(fake, cache); err == nil {
		t.Fatal("corrupt jpg should fail")
	}
}

func TestVideoFrame(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed")
	}
	dir := t.TempDir()
	clip := filepath.Join(dir, "clip.mp4")
	// 3 s portrait test pattern.
	gen := exec.Command(ffmpeg, "-hide_banner", "-loglevel", "error", "-f", "lavfi",
		"-i", "testsrc=duration=3:size=360x640:rate=10", "-pix_fmt", "yuv420p", clip)
	if out, err := gen.CombinedOutput(); err != nil {
		t.Skipf("cannot make test clip: %v %s", err, out)
	}
	jpg, secs, err := Video(clip, filepath.Join(dir, "cache"))
	if err != nil {
		t.Fatal(err)
	}
	b := decode(t, jpg).Bounds()
	if b.Dy() != 480 || b.Dx() != 270 {
		t.Fatalf("frame size %v, want 270x480", b)
	}
	if secs < 2.9 || secs > 3.1 {
		t.Fatalf("duration %v", secs)
	}
	// Second call is served from cache.
	jpg2, secs2, err := Video(clip, filepath.Join(dir, "cache"))
	if err != nil || !bytes.Equal(jpg, jpg2) || secs2 < 2.9 {
		t.Fatalf("cache: %v %v", err, secs2)
	}
}
