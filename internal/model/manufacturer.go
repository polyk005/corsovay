package model

// Manufacturer представляет производителя строительных материалов
type Manufacturer struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	Country     string  `json:"country"`
	Address     string  `json:"address"`
	Phone       string  `json:"phone"`
	Email       string  `json:"email"`
	ProductType string  `json:"product_type"`
	FoundedYear int     `json:"founded_year"`
	Revenue     float64 `json:"revenue"`
}

// Validate проверяет корректность данных производителя
func (m *Manufacturer) Validate() error {
	// Реализация валидации полей
	return nil
}
