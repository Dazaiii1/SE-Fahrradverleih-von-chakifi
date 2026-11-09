// Package store: thread-safer In-Memory-Store hinter einem Interface.
package store

import (
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"verleih/internal/domain"
)

var (
	ErrNotFound = errors.New("nicht gefunden")
	ErrConflict = errors.New("Konflikt")
)

const maxHours = 24 * 365

type Stats struct {
	Available    int     `json:"available"`
	Rented       int     `json:"rented"`
	Total        int     `json:"total"`
	OutOfService int     `json:"outOfService"`
	Charging     int     `json:"charging"`
	Persons      int     `json:"persons"`
	OverdueCount int     `json:"overdue"`
	Revenue      float64 `json:"revenue"`
}

type Store interface {
	Items() []domain.Item
	Item(nr string) (domain.Item, error)
	AddItem(domain.Item) error
	DeleteItem(nr string) error
	ReviewDone(nr string) error
	Persons() []domain.Person
	Person(id int) (domain.Person, error)
	AddPerson(domain.Person) (domain.Person, error)
	DeletePerson(id int) error
	Rentals() []domain.Rental
	Quote(category string, personID int, start time.Time, hours int, code string, accessories []string) (domain.Quote, error)
	CreateRental(category string, personID int, start time.Time, hours int, code string, accessories []string) (domain.Rental, error)
	Categories() []domain.CategoryStatus
	ReturnRental(id int, report *domain.DamageReport) (domain.Rental, error)
	RentedItemNrs() map[string]bool
	Stats() Stats
}

type MemoryStore struct {
	mu       sync.RWMutex
	items    map[string]domain.Item
	persons  map[int]domain.Person
	rentals  map[int]domain.Rental
	nextPers int
	nextRent int
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		items:    map[string]domain.Item{},
		persons:  map[int]domain.Person{},
		rentals:  map[int]domain.Rental{},
		nextPers: 1,
		nextRent: 1,
	}
}

func (s *MemoryStore) Items() []domain.Item {
	s.tick()
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.Item, 0, len(s.items))
	for _, i := range s.items {
		out = append(out, i)
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Nr < out[b].Nr })
	return out
}

func (s *MemoryStore) Item(nr string) (domain.Item, error) {
	s.tick()
	s.mu.RLock()
	defer s.mu.RUnlock()
	i, ok := s.items[nr]
	if !ok {
		return i, ErrNotFound
	}
	return i, nil
}

func (s *MemoryStore) AddItem(i domain.Item) error {
	i = domain.ApplyCategory(i)
	if err := i.Validate(); err != nil {
		return fmt.Errorf("ungültiger Eintrag: %w", err)
	}
	i.OutOfService, i.Damage = false, nil
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.items[i.Nr]; ok {
		return fmt.Errorf("Nr %s existiert bereits: %w", i.Nr, ErrConflict)
	}
	s.items[i.Nr] = i
	return nil
}

func (s *MemoryStore) rentedLocked() map[string]bool {
	m := map[string]bool{}
	for _, r := range s.rentals {
		if r.Active() {
			m[r.ItemNr] = true
		}
	}
	return m
}

func (s *MemoryStore) RentedItemNrs() map[string]bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.rentedLocked()
}

func (s *MemoryStore) DeleteItem(nr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.items[nr]; !ok {
		return ErrNotFound
	}
	if s.rentedLocked()[nr] {
		return fmt.Errorf("%s ist noch ausgeliehen: %w", nr, ErrConflict)
	}
	delete(s.items, nr)
	return nil
}

// ReviewDone beendet die Prüfung: der Artikel ist wieder im Betrieb.
func (s *MemoryStore) ReviewDone(nr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	i, ok := s.items[nr]
	if !ok {
		return ErrNotFound
	}
	if !i.OutOfService {
		return fmt.Errorf("%s ist nicht außer Betrieb: %w", nr, ErrConflict)
	}
	i.OutOfService, i.Damage = false, nil
	s.items[nr] = i
	return nil
}

func (s *MemoryStore) Persons() []domain.Person {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.Person, 0, len(s.persons))
	for _, p := range s.persons {
		out = append(out, p)
	}
	sort.Slice(out, func(a, b int) bool { return out[a].ID < out[b].ID })
	return out
}

func (s *MemoryStore) Person(id int) (domain.Person, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.persons[id]
	if !ok {
		return p, ErrNotFound
	}
	return p, nil
}

func (s *MemoryStore) AddPerson(p domain.Person) (domain.Person, error) {
	if err := p.Validate(); err != nil {
		return p, fmt.Errorf("ungültige Person: %w", err)
	}
	p.HasScan = len(p.ScanData) > 0
	p.LoyaltyBookings = 0
	s.mu.Lock()
	defer s.mu.Unlock()
	p.ID = s.nextPers
	s.nextPers++
	s.persons[p.ID] = p
	return p, nil
}

func (s *MemoryStore) DeletePerson(id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.persons[id]; !ok {
		return ErrNotFound
	}
	for _, r := range s.rentals {
		if r.Active() && r.PersonID == id {
			return fmt.Errorf("Person hat noch eine aktive Ausleihe: %w", ErrConflict)
		}
	}
	delete(s.persons, id)
	return nil
}

func (s *MemoryStore) Rentals() []domain.Rental {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.Rental, 0, len(s.rentals))
	for _, r := range s.rentals {
		out = append(out, r)
	}
	sort.Slice(out, func(a, b int) bool { return out[a].ID > out[b].ID })
	return out
}

// pickLocked wählt ein freies Fahrrad der Kategorie (E-Bike: mit dem vollsten Akku, sonst die niedrigste Nr.).
func (s *MemoryStore) pickLocked(category string, rented map[string]bool) (domain.Item, bool) {
	var best domain.Item
	found := false
	for _, it := range s.items {
		if it.Type != category || it.OutOfService || it.Charging || rented[it.Nr] {
			continue
		}
		if !found || it.Battery > best.Battery || (it.Battery == best.Battery && it.Nr < best.Nr) {
			best, found = it, true
		}
	}
	return best, found
}

// tickLocked aktualisiert die Akkus: ausgeliehen = Akku sinkt, lädt = Akku steigt.
func (s *MemoryStore) tickLocked(now time.Time) {
	rented := s.rentedLocked()
	for nr, it := range s.items {
		if !domain.HasBattery(it.Type) {
			continue
		}
		hours := now.Sub(it.BatteryUpdated).Hours() * domain.BatterySpeed
		switch {
		case rented[nr]:
			it.Battery = max(0, it.Battery-domain.DrainPerHour*hours)
		case it.Charging:
			it.Battery = min(domain.BatteryFull, it.Battery+domain.ChargePerHour*hours)
		}
		it.BatteryUpdated = now
		if !rented[nr] && !it.Charging && it.Battery <= domain.BatteryMinRent {
			it.Charging = true
		}
		if it.Charging && it.Battery >= domain.BatteryFull {
			it.Charging = false
		}
		s.items[nr] = it
	}
}

func (s *MemoryStore) tick() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tickLocked(time.Now())
}

// quoteLocked prüft die Buchung, wählt ein freies Fahrrad der Kategorie und berechnet Preis und Kaution.
func (s *MemoryStore) quoteLocked(category string, personID int, start time.Time, hours int, code string, accessories []string) (domain.Quote, domain.Item, domain.Person, error) {
	var none domain.Quote
	if hours < 1 || hours > maxHours {
		return none, domain.Item{}, domain.Person{}, fmt.Errorf("Dauer muss zwischen 1 und %d Stunden liegen", maxHours)
	}
	cat, ok := domain.CategoryByName(category)
	if !ok {
		return none, domain.Item{}, domain.Person{}, errors.New("Unbekannte Kategorie")
	}
	accNames, accDeposit, err := domain.AccessoryInfo(accessories)
	if err != nil {
		return none, domain.Item{}, domain.Person{}, err
	}
	p, ok := s.persons[personID]
	if !ok {
		return none, domain.Item{}, p, fmt.Errorf("Person %d: %w", personID, ErrNotFound)
	}
	rented := s.rentedLocked()
	item, ok := s.pickLocked(category, rented)
	if !ok {
		return none, item, p, fmt.Errorf("Keine %s verfügbar: %w", cat.Name, ErrConflict)
	}
	inService, rentedInCat, prior := 0, 0, 0
	for _, it := range s.items {
		if it.Type == category && !it.OutOfService {
			inService++
			if rented[it.Nr] {
				rentedInCat++
			}
		}
	}
	for _, r := range s.rentals {
		if r.PersonID == personID {
			prior++
		}
	}
	q, err := domain.BuildQuote(item, hours, inService, rentedInCat, code,
		domain.BookingContext{Person: p, PriorBookings: prior, Start: start})
	if err != nil {
		return q, item, p, err
	}
	q.Category, q.Accessories, q.Deposit = category, accNames, cat.Deposit+accDeposit
	return q, item, p, nil
}

func (s *MemoryStore) Quote(category string, personID int, start time.Time, hours int, code string, accessories []string) (domain.Quote, error) {
	s.tick()
	s.mu.RLock()
	defer s.mu.RUnlock()
	q, _, _, err := s.quoteLocked(category, personID, start, hours, code, accessories)
	return q, err
}

func (s *MemoryStore) CreateRental(category string, personID int, start time.Time, hours int, code string, accessories []string) (domain.Rental, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tickLocked(time.Now())
	q, item, p, err := s.quoteLocked(category, personID, start, hours, code, accessories)
	if err != nil {
		return domain.Rental{}, err
	}
	r := domain.Rental{
		ID: s.nextRent, ItemNr: item.Nr, ItemName: item.Name, Category: category, Accessories: q.Accessories, Deposit: q.Deposit,
		PersonID: p.ID, PersonName: p.Name, Start: start, Hours: hours,
		Factor: q.Factor, Voucher: q.Voucher, DiscountPercent: q.DiscountPercent, Price: q.Total,
	}
	s.nextRent++
	s.rentals[r.ID] = r
	if q.Voucher == domain.CodeLoyalty {
		p.LoyaltyBookings = 0 // gilt als letzte Buchung: Zählung beginnt von vorn
	} else {
		p.LoyaltyBookings++
	}
	s.persons[p.ID] = p
	return r, nil
}

// Categories liefert je Kategorie die Verfügbarkeit und den aktuellen Preis.
func (s *MemoryStore) Categories() []domain.CategoryStatus {
	s.tick()
	s.mu.RLock()
	defer s.mu.RUnlock()
	rented := s.rentedLocked()
	out := []domain.CategoryStatus{}
	for _, c := range domain.Categories {
		cs := domain.CategoryStatus{Category: c}
		for _, it := range s.items {
			if it.Type != c.Name || it.OutOfService {
				continue
			}
			cs.Total++
			switch {
			case rented[it.Nr]:
				cs.Rented++
			case it.Charging:
				cs.Charging++
			default:
				cs.Available++
			}
		}
		cs.CurrentRate = domain.Item{PriceModel: c.PriceModel, Rate: c.Rate}.CalcPrice(1, domain.DynamicFactor(cs.Total, cs.Rented))
		out = append(out, cs)
	}
	return out
}

// ReturnRental nimmt den Artikel zurück. Mit Schadenbericht geht er außer Betrieb (zur Prüfung).
func (s *MemoryStore) ReturnRental(id int, report *domain.DamageReport) (domain.Rental, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tickLocked(time.Now())
	r, ok := s.rentals[id]
	if !ok {
		return r, ErrNotFound
	}
	if !r.Active() {
		return r, fmt.Errorf("bereits zurückgegeben: %w", ErrConflict)
	}
	now := time.Now()
	r.ReturnedAt = &now
	r.DepositReturned = report == nil // mit Schadenbericht wird die Kaution einbehalten
	s.rentals[id] = r
	if report != nil {
		if it, ok := s.items[r.ItemNr]; ok {
			report.ReportedAt = now
			it.OutOfService, it.Damage = true, report
			s.items[it.Nr] = it
		}
	}
	s.tickLocked(now) // E-Bike mit <= 20 % geht jetzt ins Laden
	return r, nil
}

func (s *MemoryStore) Stats() Stats {
	s.tick()
	s.mu.RLock()
	defer s.mu.RUnlock()
	st := Stats{Total: len(s.items), Persons: len(s.persons)}
	now := time.Now()
	rented := s.rentedLocked()
	for _, it := range s.items {
		switch {
		case it.OutOfService:
			st.OutOfService++
		case rented[it.Nr]:
			st.Rented++
		case it.Charging:
			st.Charging++
		default:
			st.Available++
		}
	}
	for _, r := range s.rentals {
		if r.Overdue(now) {
			st.OverdueCount++
		}
		if !r.Active() {
			st.Revenue += r.Price
		}
	}
	return st
}
