package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"verleih/internal/domain"
	"verleih/internal/handler"
	"verleih/internal/store"
)

//go:embed web
var webFS embed.FS

// secure setzt Sicherheits-Header (Schutz gegen XSS, Clickjacking, MIME-Sniffing).
func secure(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", "default-src 'self'; img-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

func main() {
	if v, err := strconv.ParseFloat(os.Getenv("BATTERY_SPEEDUP"), 64); err == nil && v > 0 {
		domain.BatterySpeed = v // Zeitraffer für Tests
	}
	st := store.NewMemoryStore()
	for _, i := range domain.SeedItems() {
		if err := st.AddItem(i); err != nil {
			log.Fatal(err)
		}
	}

	mux := http.NewServeMux()
	(&handler.Handler{Store: st}).Routes(mux)
	sub, _ := fs.Sub(webFS, "web")
	mux.Handle("/", http.FileServer(http.FS(sub)))

	srv := &http.Server{Addr: ":8080", Handler: secure(mux), ReadHeaderTimeout: 5 * time.Second}
	log.Printf("%s läuft auf http://localhost:8080", domain.AppConfig.AppName)
	log.Fatal(srv.ListenAndServe())
}
