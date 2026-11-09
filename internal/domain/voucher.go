package domain

import (
	"errors"
	"strings"
	"time"
)

const (
	CodeWelcome  = "Willkommen10"
	CodeBirthday = "Birthday10"
	CodeLoyalty  = "LOYALTY50"
)

// Voucher ist ein Gutschein; pro Buchung ist nur einer erlaubt.
type Voucher struct {
	Code    string
	Percent int
}

var Vouchers = []Voucher{{CodeWelcome, 10}, {CodeBirthday, 10}, {CodeLoyalty, 50}}

// BookingContext enthält, was die Gutscheinregeln über die Buchung wissen müssen.
type BookingContext struct {
	Person        Person
	PriorBookings int // bisherige Buchungen der Person (auch zurückgegebene)
	Start         time.Time
}

func FindVoucher(code string) (Voucher, bool) {
	for _, v := range Vouchers {
		if strings.EqualFold(v.Code, strings.TrimSpace(code)) {
			return v, true
		}
	}
	return Voucher{}, false
}

// Check prüft, ob der Gutschein für diese Buchung gilt.
func (v Voucher) Check(ctx BookingContext) error {
	switch v.Code {
	case CodeWelcome:
		if ctx.PriorBookings > 0 {
			return errors.New("gilt nur für die erste Buchung einer Person")
		}
	case CodeBirthday:
		bd, err := time.Parse("2006-01-02", ctx.Person.Birthday)
		if err != nil || bd.Month() != ctx.Start.Month() || bd.Day() != ctx.Start.Day() {
			return errors.New("gilt nur am Geburtstag der Person")
		}
	case CodeLoyalty:
		if ctx.Person.LoyaltyBookings < 2 {
			return errors.New("gilt erst für die dritte Buchung (nach zwei Buchungen)")
		}
	}
	return nil
}
