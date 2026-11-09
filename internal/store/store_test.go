package store

import (
	"errors"
	"testing"
	"time"

	"verleih/internal/domain"
)

const (
	normal = "Normales Fahrrad"
	ebike  = "E-Bike"
)

func setup(t *testing.T) *MemoryStore {
	s := NewMemoryStore()
	add := func(nr, typ string, bat float64, charging bool) {
		if err := s.AddItem(domain.Item{Nr: nr, Name: "T", Type: typ, Battery: bat, Charging: charging}); err != nil {
			t.Fatal(err)
		}
	}
	add("N-1", normal, 0, false)
	add("N-2", normal, 0, false)
	add("E-1", ebike, 100, false)
	add("E-2", ebike, 23, false)
	add("E-3", ebike, 15, true) // lädt schon (Testfall)
	if _, err := s.AddPerson(domain.Person{Name: "Max", Birthday: "1990-01-01", IDType: "Reisepass", IDNumber: "42"}); err != nil {
		t.Fatal(err)
	}
	return s
}

// age lässt die Akku-Zeit um d zurücklaufen (simuliert, dass d vergangen ist).
func age(s *MemoryStore, nr string, d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	it := s.items[nr]
	it.BatteryUpdated = it.BatteryUpdated.Add(-d)
	s.items[nr] = it
}

func book(s *MemoryStore, cat string, acc ...string) (domain.Rental, error) {
	return s.CreateRental(cat, 1, time.Now(), 1, "", acc)
}

func TestAutoAssignByCategory(t *testing.T) {
	s := setup(t)
	r1, err := book(s, ebike)
	if err != nil || r1.ItemNr != "E-1" {
		t.Fatalf("vollstes E-Bike erwartet, got %+v %v", r1, err)
	}
	r2, _ := book(s, ebike)
	if r2.ItemNr != "E-2" {
		t.Errorf("nächstes E-Bike erwartet, got %s", r2.ItemNr)
	}
	if _, err := book(s, ebike); !errors.Is(err, ErrConflict) {
		t.Error("E-3 lädt: es darf kein E-Bike mehr frei sein")
	}
}

func TestBatteryDrainsWhileRented(t *testing.T) {
	s := setup(t)
	r, _ := book(s, ebike) // E-1 (100 %)
	age(s, "E-1", 3*time.Hour)
	_, _ = s.ReturnRental(r.ID, nil)
	if it, _ := s.Item("E-1"); it.Battery < 69 || it.Battery > 71 || it.Charging {
		t.Errorf("nach 3 h Fahrt ca. 70 %% und weiter verfügbar: %+v", it)
	}
}

func TestLowBatteryChargesThenIsAvailableAgain(t *testing.T) {
	s := setup(t)
	_, _ = book(s, ebike)      // E-1 ausgeliehen
	r2, _ := book(s, ebike)    // E-2 (23 %) ausgeliehen
	age(s, "E-2", 1*time.Hour) // 23 % - 10 % = 13 %
	_, _ = s.ReturnRental(r2.ID, nil)
	if it, _ := s.Item("E-2"); !it.Charging || it.Battery > 20 {
		t.Fatalf("E-2 muss jetzt laden: %+v", it)
	}
	if _, err := book(s, ebike); !errors.Is(err, ErrConflict) {
		t.Error("ein ladendes E-Bike darf nicht ausgeliehen werden")
	}
	age(s, "E-2", 5*time.Hour) // 5 h laden: wieder voll
	if it, _ := s.Item("E-2"); it.Charging || it.Battery != 100 {
		t.Errorf("E-2 muss wieder voll und verfügbar sein: %+v", it)
	}
}

func TestDepositAndAccessories(t *testing.T) {
	s := setup(t)
	r, err := book(s, normal, "helmet", "lock") // 20 + 10 + 15
	if err != nil || r.Deposit != 45 || len(r.Accessories) != 2 {
		t.Fatalf("Kaution falsch: %+v %v", r, err)
	}
	if _, err := book(s, normal, "jetpack"); err == nil {
		t.Error("unbekanntes Zubehör muss scheitern")
	}
	if _, err := book(s, normal, "lock", "lock"); err == nil {
		t.Error("doppeltes Zubehör muss scheitern")
	}
	back, _ := s.ReturnRental(r.ID, nil)
	if !back.DepositReturned {
		t.Error("Kaution wird ohne Schaden zurückgegeben")
	}
	r2, _ := book(s, normal)
	back, _ = s.ReturnRental(r2.ID, &domain.DamageReport{})
	if back.DepositReturned {
		t.Error("mit Schadenbericht wird die Kaution einbehalten")
	}
}

func TestDynamicPricePerCategory(t *testing.T) {
	s := setup(t)
	if _, err := book(s, normal); err != nil { // 1 von 2 vermietet -> nur noch eins frei -> 150 %
		t.Fatal(err)
	}
	q, _ := s.Quote(normal, 1, time.Now(), 1, "", nil)
	if q.Total != 4.5 { // 3,00 € * 1,5
		t.Errorf("Normales Fahrrad: letztes freies kostet 150 %%, got %v", q.Total)
	}
	q, _ = s.Quote(ebike, 1, time.Now(), 1, "", nil)
	if q.Total != 7 {
		t.Errorf("E-Bike-Preis darf nicht von anderen Kategorien abhängen, got %v", q.Total)
	}
}

func TestDamageTakesOutOfService(t *testing.T) {
	s := setup(t)
	r, _ := book(s, normal)
	_, _ = s.ReturnRental(r.ID, &domain.DamageReport{})
	if st := s.Stats(); st.OutOfService != 1 || st.Charging != 1 {
		t.Errorf("Stats falsch: %+v", st)
	}
	if err := s.ReviewDone(r.ItemNr); err != nil {
		t.Fatal(err)
	}
}
