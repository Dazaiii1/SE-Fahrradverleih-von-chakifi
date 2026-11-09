// Package domain enthält die Kern-Structs und die Business-Logik (Preise, Gutscheine, Validierung).
package domain

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

// PriceModel legt fest, in welcher Einheit abgerechnet wird.
type PriceModel string

const (
	PerHour PriceModel = "hour" // Preis pro angefangene Stunde
	PerDay  PriceModel = "day"  // Preis pro angefangenen Tag (24h)
)

const (
	MaxPriceFactor = 1.5 // Höchstpreis = 150 % des Basispreises
	DynamicStart   = 0.5 // ab 50 % Auslastung steigt der Preis
)

// IDTypes sind die erlaubten Ausweisarten.
var IDTypes = []string{"Personalausweis", "Reisepass", "Führerschein"}

var (
	nrRe       = regexp.MustCompile(`^[A-Za-z0-9-]{1,20}$`)
	idNumberRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9 /-]{0,39}$`)
)

func round2(f float64) float64 { return math.Round(f*100) / 100 }

// DamageReport ist ein Schadenbericht bei der Rückgabe (Beschreibung und Foto sind optional).
type DamageReport struct {
	Description string    `json:"description"`
	HasPhoto    bool      `json:"hasPhoto"`
	PhotoData   []byte    `json:"-"`
	PhotoMime   string    `json:"-"`
	ReportedAt  time.Time `json:"reportedAt"`
}

// Item ist ein verleihbarer Gegenstand (Basis für Ski, Bike, ...).
type Item struct {
	Nr             string        `json:"nr"`
	Type           string        `json:"type"`
	Name           string        `json:"name"`
	PriceModel     PriceModel    `json:"priceModel"`
	Rate           float64       `json:"rate"`    // Basispreis
	Battery        float64       `json:"battery"` // nur E-Bike, in %
	Charging       bool          `json:"charging"`
	BatteryUpdated time.Time     `json:"-"`
	OutOfService   bool          `json:"outOfService"`
	Damage         *DamageReport `json:"damage,omitempty"`
}

func (i Item) Validate() error {
	name := strings.TrimSpace(i.Name)
	switch {
	case !nrRe.MatchString(i.Nr):
		return errors.New("Nr darf nur Buchstaben, Ziffern und '-' enthalten (max. 20 Zeichen)")
	case name == "" || utf8.RuneCountInString(name) > 100:
		return errors.New("Bezeichnung ist Pflicht (max. 100 Zeichen)")
	case !slices.Contains(AppConfig.Types, i.Type):
		return errors.New("Unbekannter Typ")
	case i.PriceModel != PerHour && i.PriceModel != PerDay:
		return errors.New("Preismodell muss 'hour' oder 'day' sein")
	case i.Rate <= 0 || i.Rate > 10000:
		return errors.New("Preis muss zwischen 0 und 10000 liegen")
	}
	return nil
}

// MaxRate ist der Höchstpreis (150 % des Basispreises).
func (i Item) MaxRate() float64 { return round2(i.Rate * MaxPriceFactor) }

// CalcPrice berechnet den Preis für die Dauer in Stunden; factor ist der Auslastungsfaktor (1.0 = Basispreis).
func (i Item) CalcPrice(hours int, factor float64) float64 {
	return round2(float64(i.Units(hours)) * i.Rate * factor)
}

// Units sind die abgerechneten Einheiten: Stunden, bzw. angefangene Tage bei Tagespreis.
func (i Item) Units(hours int) int {
	if i.PriceModel == PerDay {
		return int(math.Ceil(float64(hours) / 24))
	}
	return hours
}

// DynamicFactor liefert den Preisfaktor: 1.0 bis 50 % Auslastung, danach linear bis 1.5,
// der erreicht wird, wenn nur noch ein Artikel frei ist.
func DynamicFactor(inService, rented int) float64 {
	if inService <= 0 {
		return 1
	}
	r := float64(rented) / float64(inService)
	if r < DynamicStart {
		return 1
	}
	if inService-rented <= 1 {
		return MaxPriceFactor
	}
	rMax := float64(inService-1) / float64(inService)
	return 1 + (MaxPriceFactor-1)*(r-DynamicStart)/(rMax-DynamicStart)
}

// Person ist ein Kunde, identifiziert über Ausweisart + Nummer.
type Person struct {
	ID              int    `json:"id"`
	Name            string `json:"name"`
	Birthday        string `json:"birthday"` // YYYY-MM-DD
	IDType          string `json:"idType"`
	IDNumber        string `json:"idNumber"`
	HasScan         bool   `json:"hasScan"`
	ScanData        []byte `json:"-"`
	ScanMime        string `json:"-"`
	LoyaltyBookings int    `json:"loyaltyBookings"` // Buchungen seit der letzten Nutzung von LOYALTY50
}

func (p Person) Validate() error {
	name := strings.TrimSpace(p.Name)
	if name == "" || utf8.RuneCountInString(name) > 100 {
		return errors.New("Name ist Pflicht (max. 100 Zeichen)")
	}
	bd, err := time.Parse("2006-01-02", p.Birthday)
	if err != nil || bd.After(time.Now()) || bd.Year() < 1900 {
		return errors.New("Geburtstag ungültig (Format JJJJ-MM-TT)")
	}
	if !slices.Contains(IDTypes, p.IDType) {
		return errors.New("Unbekannte Ausweisart")
	}
	if !idNumberRe.MatchString(p.IDNumber) {
		return errors.New("Ausweisnummer ungültig (Buchstaben, Ziffern, Leerzeichen, '-' und '/', max. 40 Zeichen)")
	}
	return nil
}

// Rental ist eine Ausleihe mit Beginn, Dauer und festem Preis.
type Rental struct {
	ID              int        `json:"id"`
	ItemNr          string     `json:"itemNr"`
	ItemName        string     `json:"itemName"`
	Category        string     `json:"category"`
	Accessories     []string   `json:"accessories"`
	Deposit         float64    `json:"deposit"`
	DepositReturned bool       `json:"depositReturned"`
	PersonID        int        `json:"personId"`
	PersonName      string     `json:"personName"`
	Start           time.Time  `json:"start"`
	Hours           int        `json:"hours"`
	Factor          float64    `json:"factor"`
	Voucher         string     `json:"voucher,omitempty"`
	DiscountPercent int        `json:"discountPercent"`
	Price           float64    `json:"price"`
	ReturnedAt      *time.Time `json:"returnedAt,omitempty"`
}

func (r Rental) Active() bool               { return r.ReturnedAt == nil }
func (r Rental) End() time.Time             { return r.Start.Add(time.Duration(r.Hours) * time.Hour) }
func (r Rental) Overdue(now time.Time) bool { return r.Active() && now.After(r.End()) }

// Quote ist die Preisberechnung vor bzw. bei der Buchung.
type Quote struct {
	ItemNr          string     `json:"itemNr"`
	ItemName        string     `json:"itemName"`
	Hours           int        `json:"hours"`
	Unit            PriceModel `json:"unit"`  // hour | day
	Units           int        `json:"units"` // abgerechnete Einheiten
	Rate            float64    `json:"rate"`  // Basispreis pro Einheit
	Category        string     `json:"category"`
	Accessories     []string   `json:"accessories"`
	Deposit         float64    `json:"deposit"`
	BaseAmount      float64    `json:"baseAmount"` // Preis zum Basispreis
	Factor          float64    `json:"factor"`
	Subtotal        float64    `json:"subtotal"` // mit Auslastungsaufschlag
	Voucher         string     `json:"voucher,omitempty"`
	DiscountPercent int        `json:"discountPercent"`
	DiscountAmount  float64    `json:"discountAmount"`
	Total           float64    `json:"total"`
}

// BuildQuote berechnet den Preis inkl. Auslastungsaufschlag und (höchstens einem) Gutschein.
func BuildQuote(item Item, hours, inService, rented int, code string, ctx BookingContext) (Quote, error) {
	f := DynamicFactor(inService, rented)
	q := Quote{ItemNr: item.Nr, ItemName: item.Name, Hours: hours, Unit: item.PriceModel, Units: item.Units(hours), Rate: item.Rate, Factor: round2(f), BaseAmount: item.CalcPrice(hours, 1), Subtotal: item.CalcPrice(hours, f)}
	q.Total = q.Subtotal
	if strings.TrimSpace(code) != "" {
		v, ok := FindVoucher(code)
		if !ok {
			return q, errors.New("Unbekannter Gutscheincode")
		}
		if err := v.Check(ctx); err != nil {
			return q, fmt.Errorf("Gutschein nicht anwendbar: %w", err)
		}
		q.Voucher, q.DiscountPercent = v.Code, v.Percent
		q.DiscountAmount = round2(q.Subtotal * float64(v.Percent) / 100)
		q.Total = round2(q.Subtotal - q.DiscountAmount)
	}
	return q, nil
}
