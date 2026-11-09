// Package handler: HTTP-API (nur JSON) auf Basis des Stores.
package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"verleih/internal/domain"
	"verleih/internal/store"
	"verleih/internal/upload"
)

type Handler struct {
	Store store.Store
}

func (h *Handler) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/config", h.config)
	mux.HandleFunc("GET /api/dashboard", h.dashboard)
	mux.HandleFunc("GET /api/vouchers", h.vouchers)
	mux.HandleFunc("GET /api/categories", h.categories)
	mux.HandleFunc("POST /api/quote", h.quote)
	mux.HandleFunc("GET /api/items", h.listItems)
	mux.HandleFunc("POST /api/items", h.addItem)
	mux.HandleFunc("DELETE /api/items/{nr}", h.deleteItem)
	mux.HandleFunc("POST /api/items/{nr}/review-done", h.reviewDone)
	mux.HandleFunc("GET /api/items/{nr}/damage-photo", h.damagePhoto)
	mux.HandleFunc("GET /api/persons", h.listPersons)
	mux.HandleFunc("POST /api/persons", h.addPerson)
	mux.HandleFunc("DELETE /api/persons/{id}", h.deletePerson)
	mux.HandleFunc("GET /api/persons/{id}/id-scan", h.idScan)
	mux.HandleFunc("GET /api/rentals", h.listRentals)
	mux.HandleFunc("POST /api/rentals", h.createRental)
	mux.HandleFunc("POST /api/rentals/{id}/return", h.returnRental)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, err error) {
	code := http.StatusBadRequest
	switch {
	case errors.Is(err, store.ErrNotFound):
		code = http.StatusNotFound
	case errors.Is(err, store.ErrConflict):
		code = http.StatusConflict
	}
	writeJSON(w, code, map[string]string{"error": err.Error()})
}

// decode liest ausschließlich JSON (kein XML, keine unbekannten Felder).
func decode(w http.ResponseWriter, r *http.Request, v any, optional bool) bool {
	if strings.Contains(strings.ToLower(r.Header.Get("Content-Type")), "xml") {
		writeJSON(w, http.StatusUnsupportedMediaType, map[string]string{"error": "XML wird nicht akzeptiert"})
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 6<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		if optional && errors.Is(err, io.EOF) {
			return true
		}
		writeErr(w, errors.New("ungültige Anfrage (JSON erwartet)"))
		return false
	}
	return true
}

func pathID(r *http.Request) int {
	id, _ := strconv.Atoi(r.PathValue("id"))
	return id
}

func (h *Handler) config(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"config": domain.AppConfig, "idTypes": domain.IDTypes})
}

func (h *Handler) dashboard(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, h.Store.Stats())
}

func (h *Handler) categories(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"categories": h.Store.Categories(), "accessories": domain.Accessories})
}

type voucherView struct {
	Code        string `json:"code"`
	Percent     int    `json:"percent"`
	Emoji       string `json:"emoji"`
	Title       string `json:"title"`
	Tagline     string `json:"tagline"`
	Description string `json:"description"`
	Featured    bool   `json:"featured"`
}

func (h *Handler) vouchers(w http.ResponseWriter, r *http.Request) {
	out := []voucherView{}
	for _, v := range domain.Vouchers {
		t := domain.VoucherTexts[v.Code]
		out = append(out, voucherView{v.Code, v.Percent, t.Emoji, t.Title, t.Tagline, t.Description, v.Code == domain.CodeWelcome})
	}
	writeJSON(w, 200, out)
}

type rentalReq struct {
	Category    string    `json:"category"`
	PersonID    int       `json:"personId"`
	Start       time.Time `json:"start"`
	Hours       int       `json:"hours"`
	Voucher     string    `json:"voucher"`
	Accessories []string  `json:"accessories"`
}

func (h *Handler) quote(w http.ResponseWriter, r *http.Request) {
	var req rentalReq
	if !decode(w, r, &req, false) {
		return
	}
	if req.Start.IsZero() {
		req.Start = time.Now()
	}
	q, err := h.Store.Quote(req.Category, req.PersonID, req.Start, req.Hours, req.Voucher, req.Accessories)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, q)
}

type itemView struct {
	domain.Item
	Status     string  `json:"status"` // available | rented | outOfService
	Available  bool    `json:"available"`
	MaxRate    float64 `json:"maxRate"`
	HasBattery bool    `json:"hasBattery"`
}

func (h *Handler) listItems(w http.ResponseWriter, r *http.Request) {
	rented := h.Store.RentedItemNrs()
	out := []itemView{}
	for _, i := range h.Store.Items() {
		st := "available"
		switch {
		case i.OutOfService:
			st = "outOfService"
		case rented[i.Nr]:
			st = "rented"
		case i.Charging:
			st = "charging"
		}
		out = append(out, itemView{i, st, st == "available", i.MaxRate(), domain.HasBattery(i.Type)})
	}
	writeJSON(w, 200, out)
}

type itemReq struct {
	Nr         string            `json:"nr"`
	Type       string            `json:"type"`
	Name       string            `json:"name"`
	PriceModel domain.PriceModel `json:"priceModel"`
	Rate       float64           `json:"rate"`
}

func (h *Handler) addItem(w http.ResponseWriter, r *http.Request) {
	var req itemReq
	if !decode(w, r, &req, false) {
		return
	}
	i := domain.Item{Nr: req.Nr, Type: req.Type, Name: strings.TrimSpace(req.Name), PriceModel: req.PriceModel, Rate: req.Rate}
	if err := h.Store.AddItem(i); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 201, i)
}

func (h *Handler) deleteItem(w http.ResponseWriter, r *http.Request) {
	if err := h.Store.DeleteItem(r.PathValue("nr")); err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(204)
}

func (h *Handler) reviewDone(w http.ResponseWriter, r *http.Request) {
	if err := h.Store.ReviewDone(r.PathValue("nr")); err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(204)
}

// serveFile liefert einen geprüften Upload aus: Typ aus dem Inhalt, kein MIME-Sniffing, PDFs nur als Download.
func serveFile(w http.ResponseWriter, data []byte, mime string) {
	if len(data) == 0 {
		writeErr(w, store.ErrNotFound)
		return
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if mime == "application/pdf" {
		w.Header().Set("Content-Disposition", "attachment; filename=\"dokument.pdf\"")
	} else {
		w.Header().Set("Content-Disposition", "inline")
	}
	_, _ = w.Write(data)
}

func (h *Handler) damagePhoto(w http.ResponseWriter, r *http.Request) {
	it, err := h.Store.Item(r.PathValue("nr"))
	if err != nil || it.Damage == nil {
		writeErr(w, store.ErrNotFound)
		return
	}
	serveFile(w, it.Damage.PhotoData, it.Damage.PhotoMime)
}

func (h *Handler) listPersons(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, h.Store.Persons())
}

type personReq struct {
	Name     string `json:"name"`
	Birthday string `json:"birthday"`
	IDType   string `json:"idType"`
	IDNumber string `json:"idNumber"`
	Scan     string `json:"scan"`     // Data-URL (optional)
	ScanName string `json:"scanName"` // Dateiname des Uploads
}

func (h *Handler) addPerson(w http.ResponseWriter, r *http.Request) {
	var req personReq
	if !decode(w, r, &req, false) {
		return
	}
	p := domain.Person{Name: strings.TrimSpace(req.Name), Birthday: req.Birthday, IDType: req.IDType, IDNumber: strings.TrimSpace(req.IDNumber)}
	if req.Scan != "" {
		f, err := upload.Parse(req.ScanName, req.Scan, true)
		if err != nil {
			writeErr(w, err)
			return
		}
		p.ScanData, p.ScanMime = f.Data, f.Mime
	}
	p, err := h.Store.AddPerson(p)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 201, p)
}

func (h *Handler) deletePerson(w http.ResponseWriter, r *http.Request) {
	if err := h.Store.DeletePerson(pathID(r)); err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(204)
}

func (h *Handler) idScan(w http.ResponseWriter, r *http.Request) {
	p, err := h.Store.Person(pathID(r))
	if err != nil {
		writeErr(w, err)
		return
	}
	serveFile(w, p.ScanData, p.ScanMime)
}

func (h *Handler) listRentals(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, h.Store.Rentals())
}

func (h *Handler) createRental(w http.ResponseWriter, r *http.Request) {
	var req rentalReq
	if !decode(w, r, &req, false) {
		return
	}
	if req.Start.IsZero() {
		req.Start = time.Now()
	}
	rental, err := h.Store.CreateRental(req.Category, req.PersonID, req.Start, req.Hours, req.Voucher, req.Accessories)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 201, rental)
}

type returnReq struct {
	Report *struct {
		Description string `json:"description"`
		Photo       string `json:"photo"`
		PhotoName   string `json:"photoName"`
	} `json:"report"`
}

func (h *Handler) returnRental(w http.ResponseWriter, r *http.Request) {
	var req returnReq
	if !decode(w, r, &req, true) {
		return
	}
	var report *domain.DamageReport
	if req.Report != nil {
		desc := strings.TrimSpace(req.Report.Description)
		if len([]rune(desc)) > 1000 {
			writeErr(w, errors.New("Beschreibung zu lang (max. 1000 Zeichen)"))
			return
		}
		report = &domain.DamageReport{Description: desc}
		if req.Report.Photo != "" {
			f, err := upload.Parse(req.Report.PhotoName, req.Report.Photo, false)
			if err != nil {
				writeErr(w, err)
				return
			}
			report.PhotoData, report.PhotoMime, report.HasPhoto = f.Data, f.Mime, true
		}
	}
	rental, err := h.Store.ReturnRental(pathID(r), report)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, rental)
}
