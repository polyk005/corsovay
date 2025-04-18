package model

import (
	"errors"
	"regexp"
	"time"
)

type Manufacturer struct {
	ID          int     `json:"id" csv:"id"`
	Name        string  `json:"name" csv:"name"`
	Country     string  `json:"country" csv:"country"`
	Address     string  `json:"address" csv:"address"`
	Phone       string  `json:"phone" csv:"phone"`
	Email       string  `json:"email" csv:"email"`
	ProductType string  `json:"product_type" csv:"product_type"`
	FoundedYear int     `json:"founded_year" csv:"founded_year"`
	Revenue     float64 `json:"revenue" csv:"revenue"`
	Employees   int     `json:"employees" csv:"employees"`
	Website     string  `json:"website" csv:"website"`
}

func (m *Manufacturer) Validate() error {
	if m.Name == "" {
		return errors.New("name is required")
	}

	if m.FoundedYear < 1800 || m.FoundedYear > time.Now().Year() {
		return errors.New("invalid founded year")
	}

	if m.Revenue < 0 {
		return errors.New("revenue cannot be negative")
	}

	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(m.Email) {
		return errors.New("invalid email format")
	}

	return nil
}
