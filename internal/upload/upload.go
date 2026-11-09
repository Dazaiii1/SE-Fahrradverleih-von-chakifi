// Package upload prüft hochgeladene Dateien (Ausweis-Scan, Schadenfoto), bevor sie gespeichert werden.
// Nur PDF/JPEG/PNG sind erlaubt. Geprüft werden Dateiname, Endungen, Größe und der echte Inhalt (Magic Bytes).
package upload

import (
	"bytes"
	"encoding/base64"
	"errors"
	"path"
	"strings"
)

const MaxBytes = 2 << 20 // 2 MB

type File struct {
	Data []byte
	Mime string
}

// Endungen, die in einem Dateinamen nie vorkommen dürfen (z.B. "shell.php.pdf").
var dangerous = map[string]bool{
	"php": true, "php3": true, "php4": true, "php5": true, "php7": true, "phtml": true, "phar": true,
	"asp": true, "aspx": true, "jsp": true, "jspx": true, "cgi": true, "pl": true, "py": true, "rb": true,
	"sh": true, "bash": true, "exe": true, "dll": true, "bat": true, "cmd": true, "com": true, "ps1": true,
	"vbs": true, "js": true, "mjs": true, "html": true, "htm": true, "xhtml": true, "xml": true, "svg": true,
	"jar": true, "war": true, "go": true, "so": true,
}

var errInvalid = errors.New("Upload ungültig: erlaubt sind nur echte PDF-, JPG- oder PNG-Dateien bis 2 MB")

// Parse prüft einen Upload (Data-URL mit base64) und liefert Daten + geprüften MIME-Typ.
func Parse(name, dataURL string, allowPDF bool) (*File, error) {
	const marker = ";base64,"
	i := strings.Index(dataURL, marker)
	if !strings.HasPrefix(dataURL, "data:") || i < 0 {
		return nil, errInvalid
	}
	raw := dataURL[i+len(marker):]
	if base64.StdEncoding.DecodedLen(len(raw)) > MaxBytes+3 {
		return nil, errInvalid
	}
	data, err := base64.StdEncoding.DecodeString(raw)
	if err != nil || len(data) == 0 || len(data) > MaxBytes {
		return nil, errInvalid
	}
	ext, err := checkName(name)
	if err != nil {
		return nil, err
	}
	mime := sniff(data)
	ok := (mime == "application/pdf" && ext == "pdf" && allowPDF) ||
		(mime == "image/jpeg" && (ext == "jpg" || ext == "jpeg")) ||
		(mime == "image/png" && ext == "png")
	if !ok {
		return nil, errInvalid
	}
	if mime == "application/pdf" && hasActiveContent(data) {
		return nil, errors.New("Upload ungültig: PDF mit aktiven Inhalten (JavaScript/Launch) ist nicht erlaubt")
	}
	return &File{Data: data, Mime: mime}, nil
}

func checkName(name string) (string, error) {
	name = path.Base(strings.ReplaceAll(strings.ToLower(strings.TrimSpace(name)), "\\", "/"))
	if name == "" || len(name) > 100 {
		return "", errInvalid
	}
	for _, r := range name {
		if r < 0x20 {
			return "", errInvalid
		}
	}
	parts := strings.Split(name, ".")
	if len(parts) < 2 {
		return "", errInvalid
	}
	for _, p := range parts[1 : len(parts)-1] { // alle Endungen vor der letzten
		if dangerous[p] {
			return "", errInvalid
		}
	}
	ext := parts[len(parts)-1]
	if ext != "pdf" && ext != "jpg" && ext != "jpeg" && ext != "png" {
		return "", errInvalid
	}
	return ext, nil
}

// sniff erkennt den Typ am Dateianfang, nicht an dem, was der Client behauptet.
func sniff(d []byte) string {
	switch {
	case bytes.HasPrefix(d, []byte("%PDF-")):
		return "application/pdf"
	case bytes.HasPrefix(d, []byte{0xFF, 0xD8, 0xFF}):
		return "image/jpeg"
	case bytes.HasPrefix(d, []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}):
		return "image/png"
	}
	return ""
}

func hasActiveContent(d []byte) bool {
	for _, k := range []string{"/JavaScript", "/Launch", "/EmbeddedFile"} {
		if bytes.Contains(d, []byte(k)) {
			return true
		}
	}
	return false
}
