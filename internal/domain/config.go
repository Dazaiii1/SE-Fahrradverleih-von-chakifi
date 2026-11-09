package domain

import (
	"errors"
	"time"
)

// Category ist eine Fahrrad-Kategorie: Preis und Kaution gelten pro Kategorie.
type Category struct {
	Name       string     `json:"name"`
	PriceModel PriceModel `json:"priceModel"`
	Rate       float64    `json:"rate"`    // Basispreis
	Deposit    float64    `json:"deposit"` // Kaution
	Battery    bool       `json:"hasBattery"`
}

// CategoryStatus ist eine Kategorie plus aktuelle Verfügbarkeit (Auswahl bei der Ausleihe).
type CategoryStatus struct {
	Category
	Total       int     `json:"total"`
	Available   int     `json:"available"`
	Rented      int     `json:"rented"`
	Charging    int     `json:"charging"`
	CurrentRate float64 `json:"currentRate"` // Preis inkl. Auslastungsaufschlag
}

// Accessory ist Zubehör, das die Kaution erhöht.
type Accessory struct {
	Code    string  `json:"code"`
	Name    string  `json:"name"`
	Deposit float64 `json:"deposit"`
}

var Categories = []Category{
	{"Normales Fahrrad", PerHour, 3, 20, false},
	{"Mountainbike", PerHour, 4.5, 40, false},
	{"E-Bike", PerHour, 7, 100, true},
}

var Accessories = []Accessory{{"helmet", "Helm", 10}, {"lock", "Schloss", 15}}

// Akku-Regeln (nur E-Bikes): ausgeliehen = in Fahrt, der Akku sinkt; bei <= 20 % nicht mehr ausleihbar, es lädt.
const (
	BatteryMinRent = 20.0
	BatteryFull    = 100.0
	DrainPerHour   = 10.0 // % pro Stunde, solange ausgeliehen
	ChargePerHour  = 25.0 // % pro Stunde, solange es lädt
)

// BatterySpeed ist ein Zeitraffer für Tests (Umgebungsvariable BATTERY_SPEEDUP, z.B. 60 = 1 Minute wie 1 Stunde).
var BatterySpeed = 1.0

func CategoryByName(name string) (Category, bool) {
	for _, c := range Categories {
		if c.Name == name {
			return c, true
		}
	}
	return Category{}, false
}

func HasBattery(typ string) bool { c, ok := CategoryByName(typ); return ok && c.Battery }

// ApplyCategory übernimmt Preis (und Akku-Startwerte) aus der Kategorie.
func ApplyCategory(i Item) Item {
	c, ok := CategoryByName(i.Type)
	if !ok {
		return i
	}
	i.PriceModel, i.Rate = c.PriceModel, c.Rate
	if c.Battery {
		if i.Battery <= 0 && !i.Charging {
			i.Battery = 100
		}
		i.Battery = min(i.Battery, 100)
		i.BatteryUpdated = time.Now()
	} else {
		i.Battery, i.Charging = 0, false
	}
	return i
}

// AccessoryInfo prüft die Zubehör-Codes und liefert Namen und Kaution-Aufschlag.
func AccessoryInfo(codes []string) (names []string, deposit float64, err error) {
	seen := map[string]bool{}
	for _, c := range codes {
		if seen[c] {
			return nil, 0, errors.New("Zubehör doppelt angegeben")
		}
		seen[c] = true
		found := false
		for _, a := range Accessories {
			if a.Code == c {
				names, deposit, found = append(names, a.Name), deposit+a.Deposit, true
			}
		}
		if !found {
			return nil, 0, errors.New("Unbekanntes Zubehör")
		}
	}
	return names, deposit, nil
}

// Config beschreibt, WAS verliehen wird.
type Config struct {
	AppName      string   `json:"appName"`
	ItemSingular string   `json:"itemSingular"`
	ItemPlural   string   `json:"itemPlural"`
	NrPrefix     string   `json:"nrPrefix"`
	Types        []string `json:"types"` // = Kategorien
}

func categoryNames() []string {
	out := []string{}
	for _, c := range Categories {
		out = append(out, c.Name)
	}
	return out
}

var AppConfig = Config{
	AppName:      "Fahrrad-Verleih",
	ItemSingular: "Fahrrad",
	ItemPlural:   "Fahrräder",
	NrPrefix:     "BIKE",
	Types:        categoryNames(),
}

// VoucherText sind die Werbetexte der Gutscheine (Gutschein-Seite und Banner).
type VoucherText struct {
	Emoji       string
	Title       string
	Tagline     string
	Description string
}

var VoucherTexts = map[string]VoucherText{
	CodeWelcome: {"🚲", "Willkommen auf dem Sattel!", "10 % auf deine allererste Ausleihe",
		"Neu bei uns? Mit diesem Code startest du günstiger in deine erste Tour. Er gilt einmalig für die erste Buchung jeder Person."},
	CodeBirthday: {"🎂", "Dein Tag, dein Rabatt", "10 % an deinem Geburtstag",
		"Zum Geburtstag schenken wir dir 10 % auf deine Ausleihe. Wir prüfen das Datum automatisch anhand deines Profils."},
	CodeLoyalty: {"🏆", "Treue wird belohnt", "50 % auf jede dritte Buchung",
		"Zwei Ausleihen gemacht? Dann ist die dritte nur halb so teuer. Danach beginnt das Zählen von vorn."},
}

// SeedItems sind Demo-Daten beim Start. Preis kommt aus der Kategorie.
// Zum Testen: ein E-Bike lädt schon (15 %), eines hat nur 23 % (nach kurzer Fahrt unter 20 %).
func SeedItems() []Item {
	list := []Item{
		{Type: "Normales Fahrrad", Name: "KTM City Line 28"}, {Type: "Normales Fahrrad", Name: "Pegasus Solero"},
		{Type: "Normales Fahrrad", Name: "Puch Vienna 7"}, {Type: "Normales Fahrrad", Name: "Woom Explore Tour"},
		{Type: "Mountainbike", Name: "Scott Aspect 29"}, {Type: "Mountainbike", Name: "Cube Reaction"},
		{Type: "Mountainbike", Name: "Trek Marlin 7"}, {Type: "Mountainbike", Name: "Specialized Rockhopper"},
		{Type: "E-Bike", Name: "Cube Reaction Hybrid", Battery: 100}, {Type: "E-Bike", Name: "Haibike Trekking", Battery: 85},
		{Type: "E-Bike", Name: "KTM Macina Cross", Battery: 23}, {Type: "E-Bike", Name: "Giant Explore E+", Battery: 15, Charging: true},
	}
	for i := range list {
		list[i].Nr = fmtNr(AppConfig.NrPrefix, i+1)
	}
	return list
}

func fmtNr(prefix string, n int) string {
	d := []byte{'0' + byte(n/100%10), '0' + byte(n/10%10), '0' + byte(n%10)}
	return prefix + "-" + string(d)
}
