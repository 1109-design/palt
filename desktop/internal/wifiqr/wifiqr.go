// Package wifiqr reads and writes the Wi-Fi network QR format Android and
// iOS use for "Share Wi-Fi":
//
//	WIFI:T:WPA;S:My Network;P:secret;H:false;;
//
// Special characters (\ ; , : ") are escaped with a backslash.
// Must stay identical to mobile/lib/features/wifi_share.dart.
package wifiqr

import (
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"strings"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
)

// Network is a Wi-Fi network and how to join it.
type Network struct {
	SSID     string `json:"ssid"`
	Password string `json:"password"`
	Security string `json:"security"` // "WPA" (also WPA2/WPA3), "WEP", "nopass"
	Hidden   bool   `json:"hidden"`
}

// Parse reads a WIFI: QR payload.
func Parse(s string) (Network, error) {
	if !strings.HasPrefix(s, "WIFI:") {
		return Network{}, errors.New("not a Wi-Fi QR code")
	}
	var n Network
	for _, field := range split(s[len("WIFI:"):], ';') {
		k, v, ok := cut(field)
		if !ok {
			continue
		}
		switch k {
		case "S":
			n.SSID = v
		case "P":
			n.Password = v
		case "T":
			n.Security = strings.ToUpper(v)
		case "H":
			n.Hidden = strings.EqualFold(v, "true")
		}
	}
	if n.SSID == "" {
		return Network{}, errors.New("Wi-Fi QR code has no network name")
	}
	if n.Security == "" || n.Security == "NOPASS" {
		if n.Password == "" {
			n.Security = "nopass"
		} else {
			n.Security = "WPA"
		}
	}
	return n, nil
}

// String writes the WIFI: QR payload.
func (n Network) String() string {
	t := n.Security
	if t == "" {
		t = "nopass"
	}
	s := "WIFI:T:" + t + ";S:" + escape(n.SSID) + ";"
	if t != "nopass" {
		s += "P:" + escape(n.Password) + ";"
	}
	if n.Hidden {
		s += "H:true;"
	}
	return s + ";"
}

// FromImage looks for a Wi-Fi QR code in a picture file.
func FromImage(path string) (Network, error) {
	f, err := os.Open(path)
	if err != nil {
		return Network{}, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return Network{}, err
	}
	bmp, err := gozxing.NewBinaryBitmapFromImage(img)
	if err != nil {
		return Network{}, err
	}
	hints := map[gozxing.DecodeHintType]interface{}{gozxing.DecodeHintType_TRY_HARDER: true}
	res, err := qrcode.NewQRCodeReader().Decode(bmp, hints)
	if err != nil {
		return Network{}, err
	}
	return Parse(res.GetText())
}

func escape(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `;`, `\;`, `,`, `\,`, `:`, `\:`, `"`, `\"`)
	return r.Replace(s)
}

// split cuts s at unescaped sep, removing the escapes.
func split(s string, sep byte) []string {
	var out []string
	var cur strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\\' && i+1 < len(s) {
			i++
			cur.WriteByte(s[i])
			continue
		}
		if c == sep {
			out = append(out, cur.String())
			cur.Reset()
			continue
		}
		cur.WriteByte(c)
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

// cut splits "K:value" at the first colon (values were already unescaped,
// and keys never contain colons).
func cut(field string) (string, string, bool) {
	i := strings.IndexByte(field, ':')
	if i <= 0 {
		return "", "", false
	}
	return field[:i], field[i+1:], true
}
