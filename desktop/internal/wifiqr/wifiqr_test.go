package wifiqr

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
)

func TestParseAndWrite(t *testing.T) {
	n, err := Parse(`WIFI:T:WPA;S:Café\;Net;P:pa\:ss\\w;H:true;;`)
	if err != nil || n.SSID != "Café;Net" || n.Password != `pa:ss\w` || n.Security != "WPA" || !n.Hidden {
		t.Fatalf("parse: %+v %v", n, err)
	}
	back, err := Parse(n.String())
	if err != nil || back != n {
		t.Fatalf("round trip: %+v %v", back, err)
	}
	open, _ := Parse("WIFI:S:Guest;;")
	if open.Security != "nopass" || open.Password != "" {
		t.Fatalf("open network: %+v", open)
	}
	if _, err := Parse("https://example.com"); err == nil {
		t.Fatal("non-Wi-Fi payload accepted")
	}
}

func TestFromImage(t *testing.T) {
	want := Network{SSID: "HPCSikabala", Password: "hunter2;", Security: "WPA"}
	m, err := qrcode.NewQRCodeWriter().Encode(want.String(), gozxing.BarcodeFormat_QR_CODE, 300, 300, nil)
	if err != nil {
		t.Fatal(err)
	}
	img := image.NewGray(image.Rect(0, 0, 300, 300))
	for y := 0; y < 300; y++ {
		for x := 0; x < 300; x++ {
			if !m.Get(x, y) {
				img.Pix[y*img.Stride+x] = 255
			}
		}
	}
	path := filepath.Join(t.TempDir(), "qr.png")
	f, _ := os.Create(path)
	png.Encode(f, img)
	f.Close()
	got, err := FromImage(path)
	if err != nil || got != want {
		t.Fatalf("from image: %+v %v", got, err)
	}
}
