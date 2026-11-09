package upload

import (
	"encoding/base64"
	"testing"
)

func du(b []byte) string {
	return "data:application/octet-stream;base64," + base64.StdEncoding.EncodeToString(b)
}

var (
	pdf = []byte("%PDF-1.4\n1 0 obj\n<<>>\nendobj\n")
	png = []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A, 0, 0, 0, 0}
	php = []byte("<?php system($_GET['c']); ?>")
)

func TestParse(t *testing.T) {
	cases := []struct {
		name string
		file string
		data []byte
		pdf  bool
		ok   bool
	}{
		{"scan.pdf", "", pdf, true, true},
		{"foto.PNG", "", png, false, true},
		{"scan.pdf.php", "", pdf, true, false},            // Endung am Ende
		{"shell.php.pdf", "", pdf, true, false},           // gefährliche Endung davor
		{"evil.pdf", "", php, true, false},                // PHP-Inhalt als .pdf getarnt
		{"bild.png", "", pdf, true, false},                // Endung passt nicht zum Inhalt
		{"vector.svg", "", []byte("<svg/>"), true, false}, // kein XML/SVG
		{"scan.pdf", "", pdf, false, false},               // PDF nicht erlaubt (Schadenfoto)
		{"../../x.png", "", png, false, true},             // Pfadanteile werden entfernt
		{"noext", "", png, false, false},
	}
	for _, c := range cases {
		_, err := Parse(c.name, du(c.data), c.pdf)
		if (err == nil) != c.ok {
			t.Errorf("%s: ok=%v, err=%v", c.name, c.ok, err)
		}
	}
	if _, err := Parse("a.pdf", "kein-data-url", true); err == nil {
		t.Error("ungültige Data-URL muss scheitern")
	}
	if _, err := Parse("a.pdf", du([]byte("%PDF-1.4 /JavaScript (app.alert(1))")), true); err == nil {
		t.Error("PDF mit JavaScript muss scheitern")
	}
}
