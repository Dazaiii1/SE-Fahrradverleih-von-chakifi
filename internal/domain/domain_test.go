package domain

import (
	"testing"
	"time"
)

func TestCalcPrice(t *testing.T) {
	h := Item{PriceModel: PerHour, Rate: 4}
	d := Item{PriceModel: PerDay, Rate: 30}
	cases := []struct {
		item   Item
		hours  int
		factor float64
		want   float64
	}{{h, 3, 1, 12}, {h, 1, 1, 4}, {d, 1, 1, 30}, {d, 24, 1, 30}, {d, 25, 1, 60}, {h, 2, 1.5, 12}}
	for _, c := range cases {
		if got := c.item.CalcPrice(c.hours, c.factor); got != c.want {
			t.Errorf("%s %dh x%v: got %v, want %v", c.item.PriceModel, c.hours, c.factor, got, c.want)
		}
	}
}

func TestDynamicFactor(t *testing.T) {
	// 10 Artikel: bis 4 vermietet = 1.0, bei 5 (50 %) = 1.0, danach linear, bei 9 (nur 1 frei) = 1.5
	if f := DynamicFactor(10, 4); f != 1 {
		t.Errorf("unter 50 %%: %v", f)
	}
	if f := DynamicFactor(10, 5); f != 1 {
		t.Errorf("genau 50 %%: %v", f)
	}
	prev := 1.0
	for rented := 6; rented <= 9; rented++ {
		f := DynamicFactor(10, rented)
		if f <= prev {
			t.Errorf("Preis muss steigen: %d -> %v (vorher %v)", rented, f, prev)
		}
		prev = f
	}
	if f := DynamicFactor(10, 9); f != MaxPriceFactor {
		t.Errorf("nur noch einer frei: %v, want %v", f, MaxPriceFactor)
	}
	if f := DynamicFactor(1, 0); f != 1 {
		t.Errorf("1 Artikel, 0 vermietet: %v", f)
	}
}

func TestVouchers(t *testing.T) {
	p := Person{Birthday: "1990-03-05"}
	at := func(s string) time.Time { x, _ := time.Parse("2006-01-02", s); return x }
	w, _ := FindVoucher("willkommen10")
	if w.Check(BookingContext{Person: p, PriorBookings: 0, Start: at("2026-01-01")}) != nil {
		t.Error("Willkommen10 bei erster Buchung muss gelten")
	}
	if w.Check(BookingContext{Person: p, PriorBookings: 1, Start: at("2026-01-01")}) == nil {
		t.Error("Willkommen10 bei zweiter Buchung darf nicht gelten")
	}
	b, _ := FindVoucher(CodeBirthday)
	if b.Check(BookingContext{Person: p, Start: at("2026-03-05")}) != nil || b.Check(BookingContext{Person: p, Start: at("2026-03-06")}) == nil {
		t.Error("Birthday10 nur am Geburtstag")
	}
	l, _ := FindVoucher(CodeLoyalty)
	if l.Check(BookingContext{Person: Person{LoyaltyBookings: 1}}) == nil || l.Check(BookingContext{Person: Person{LoyaltyBookings: 2}}) != nil {
		t.Error("LOYALTY50 erst nach zwei Buchungen")
	}
}

func TestBuildQuote(t *testing.T) {
	item := Item{Nr: "X", PriceModel: PerHour, Rate: 10}
	ctx := BookingContext{Person: Person{Birthday: "1990-01-01"}}
	q, err := BuildQuote(item, 2, 10, 9, CodeWelcome, ctx) // Faktor 1.5, 20 * 1.5 = 30, -10 % = 27
	if err != nil || q.Subtotal != 30 || q.Total != 27 {
		t.Fatalf("Quote falsch: %+v %v", q, err)
	}
	if _, err := BuildQuote(item, 2, 10, 0, "gibtsnicht", ctx); err == nil {
		t.Error("unbekannter Code muss scheitern")
	}
}

func TestValidate(t *testing.T) {
	ok := Person{Name: "A", Birthday: "1990-01-01", IDType: "Reisepass", IDNumber: "P 123-4"}
	if ok.Validate() != nil {
		t.Error("gültige Person abgelehnt")
	}
	for _, bad := range []Person{
		{Name: "A", Birthday: "2999-01-01", IDType: "Reisepass", IDNumber: "1"},
		{Name: "A", Birthday: "1990-01-01", IDType: "Foo", IDNumber: "1"},
		{Name: "A", Birthday: "1990-01-01", IDType: "Reisepass", IDNumber: "<script>"},
	} {
		if bad.Validate() == nil {
			t.Errorf("ungültige Person akzeptiert: %+v", bad)
		}
	}
}
