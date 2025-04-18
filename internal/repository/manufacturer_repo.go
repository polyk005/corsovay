package repository

import (
	"cursovay/internal/model"
	"encoding/csv"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// ManufacturerRepository реализует хранение данных о производителях
type ManufacturerRepository struct {
	filePath  string
	data      []model.Manufacturer
	mu        sync.RWMutex
	currentID int
}

// NewManufacturerRepository создает новый экземпляр репозитория
func NewManufacturerRepository(filePath string) *ManufacturerRepository {
	repo := &ManufacturerRepository{
		filePath: filePath,
	}

	// Автоматическая загрузка данных при создании
	if err := repo.Load(); err != nil {
		// Можно добавить логирование ошибки
	}

	return repo
}

func (r *ManufacturerRepository) SetFilePath(path string) {
	r.filePath = path
}

// Load загружает данные из CSV файла
func (r *ManufacturerRepository) Load() error {
	file, err := os.Open(r.filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return err
	}

	r.data = make([]model.Manufacturer, 0, len(records))

	for _, record := range records {
		if len(record) < 9 { // Проверяем, что есть все 9 полей
			continue
		}

		id, err := strconv.Atoi(strings.TrimSpace(record[0]))
		if err != nil {
			continue
		}

		foundedYear, err := strconv.Atoi(strings.TrimSpace(record[7]))
		if err != nil {
			foundedYear = 0 // или можно вернуть ошибку
		}

		revenue, err := strconv.ParseFloat(strings.TrimSpace(record[8]), 64)
		if err != nil {
			revenue = 0
		}

		manufacturer := model.Manufacturer{
			ID:          id,
			Name:        strings.TrimSpace(record[1]),
			Country:     strings.TrimSpace(record[2]),
			Address:     strings.TrimSpace(record[3]),
			Phone:       strings.TrimSpace(record[4]),
			Email:       strings.TrimSpace(record[5]),
			ProductType: strings.TrimSpace(record[6]),
			FoundedYear: foundedYear,
			Revenue:     revenue,
		}
		r.data = append(r.data, manufacturer)
	}

	return nil
}

// GetAll возвращает всех производителей
func (r *ManufacturerRepository) GetAll() ([]model.Manufacturer, error) {
	return r.data, nil
}

// GetByID находит производителя по ID
func (r *ManufacturerRepository) GetByID(id int) (*model.Manufacturer, error) {
	for _, m := range r.data {
		if m.ID == id {
			return &m, nil
		}
	}
	return nil, nil
}

// Create добавляет нового производителя
func (r *ManufacturerRepository) Create(m *model.Manufacturer) error {
	// Генерируем новый ID
	maxID := 0
	for _, item := range r.data {
		if item.ID > maxID {
			maxID = item.ID
		}
	}
	m.ID = maxID + 1

	r.data = append(r.data, *m)
	return r.Save()
}

// Save сохраняет все данные в файл
func (r *ManufacturerRepository) Save() error {
	file, err := os.Create(r.filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	for _, m := range r.data {
		record := []string{
			strconv.Itoa(m.ID),
			m.Name,
			m.Country,
			m.Address,
			m.Phone,
			m.Email,
			m.ProductType,
			strconv.Itoa(m.FoundedYear),
			strconv.FormatFloat(m.Revenue, 'f', 2, 64),
		}
		if err := writer.Write(record); err != nil {
			return err
		}
	}

	return nil
}

// Update обновляет данные производителя
func (r *ManufacturerRepository) Update(m *model.Manufacturer) error {
	for i, item := range r.data {
		if item.ID == m.ID {
			r.data[i] = *m
			return r.Save()
		}
	}
	return nil
}

// Delete удаляет производителя
func (r *ManufacturerRepository) Delete(id int) error {
	for i, item := range r.data {
		if item.ID == id {
			r.data = append(r.data[:i], r.data[i+1:]...)
			return r.Save()
		}
	}
	return nil
}

func (r *ManufacturerRepository) SortBy(column string, ascending bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	sort.Slice(r.data, func(i, j int) bool {
		switch column {
		case "name":
			if ascending {
				return r.data[i].Name < r.data[j].Name
			}
			return r.data[i].Name > r.data[j].Name
		case "revenue":
			if ascending {
				return r.data[i].Revenue < r.data[j].Revenue
			}
			return r.data[i].Revenue > r.data[j].Revenue
		// ... другие поля
		default:
			return false
		}
	})
}

func (r *ManufacturerRepository) Search(column, query string) []model.Manufacturer {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var results []model.Manufacturer
	for _, m := range r.data {
		var field string
		switch column {
		case "name":
			field = m.Name
		case "email":
			field = m.Email
		// ... другие поля
		default:
			continue
		}

		if strings.Contains(strings.ToLower(field), strings.ToLower(query)) {
			results = append(results, m)
		}
	}
	return results
}
