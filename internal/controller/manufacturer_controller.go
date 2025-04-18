package controller

import (
	"cursovay/internal/model"
	"cursovay/internal/repository"
	"cursovay/internal/service"
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
)

type ManufacturerController struct {
	service       *service.ManufacturerService
	manufacturers []model.Manufacturer
	currentFile   string
}

func NewManufacturerController(repo *repository.ManufacturerRepository) *ManufacturerController {
	return &ManufacturerController{
		service:       service.NewManufacturerService(repo),
		manufacturers: []model.Manufacturer{},
		currentFile:   "",
	}
}

func (c *ManufacturerController) GetAllManufacturers() ([]model.Manufacturer, error) {
	// Если данные уже загружены, возвращаем их
	if len(c.manufacturers) > 0 {
		return c.manufacturers, nil
	}

	// Иначе загружаем из сервиса
	var err error
	c.manufacturers, err = c.service.GetAll()
	if err != nil {
		return nil, err
	}
	return c.manufacturers, nil
}

func (c *ManufacturerController) GetManufacturerByID(id int) (*model.Manufacturer, error) {
	for _, m := range c.manufacturers {
		if m.ID == id {
			return &m, nil
		}
	}
	return nil, errors.New("manufacturer not found")
}

func (c *ManufacturerController) CreateManufacturer(m *model.Manufacturer) error {
	if err := c.service.Create(m); err != nil {
		return err
	}
	c.manufacturers = append(c.manufacturers, *m)
	return nil

}

func (c *ManufacturerController) UpdateManufacturer(m *model.Manufacturer) error {
	for i, item := range c.manufacturers {
		if item.ID == m.ID {
			c.manufacturers[i] = *m
			// Сохраняем в файл, если он указан
			if c.currentFile != "" {
				return c.SaveToFile(c.currentFile)
			}
			return nil
		}
	}
	return errors.New("manufacturer not found")
}

func (c *ManufacturerController) DeleteManufacturer(id int) error {
	for i, item := range c.manufacturers {
		if item.ID == id {
			// Создаем новый слайс без удаленного элемента
			newManufacturers := make([]model.Manufacturer, 0, len(c.manufacturers)-1)
			newManufacturers = append(newManufacturers, c.manufacturers[:i]...)
			newManufacturers = append(newManufacturers, c.manufacturers[i+1:]...)

			c.manufacturers = newManufacturers

			if c.currentFile != "" {
				return c.SaveToFile(c.currentFile)
			}
			return nil
		}
	}
	return errors.New("manufacturer not found")
}

func (c *ManufacturerController) FileExists(filePath string) bool {
	if _, err := os.Stat(filePath); err == nil {
		return true
	}
	return false
}

func (c *ManufacturerController) LoadFromFile(filePath string) error {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		// Создаем пустой файл, если его нет
		file, createErr := os.Create(filePath)
		if createErr != nil {
			return fmt.Errorf("не удалось создать файл: %v", createErr)
		}
		file.Close()

		// Инициализируем пустую базу
		c.manufacturers = []model.Manufacturer{}
		c.currentFile = filePath
		return nil
	}

	// Проверяем, не пустой ли файл
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("ошибка проверки файла: %v", err)
	}

	if fileInfo.Size() == 0 {
		// Файл существует, но пустой - инициализируем пустую базу
		c.manufacturers = []model.Manufacturer{}
		c.currentFile = filePath
		return nil
	}

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("ошибка открытия файла: %v", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Comma = ','
	reader.FieldsPerRecord = -1 // Разрешаем разное количество полей

	records, err := reader.ReadAll()
	if err != nil {
		return err
	}

	var manufacturers []model.Manufacturer
	for i, record := range records {
		if i == 0 && len(record) > 0 && record[0] == "ID" {
			continue // Пропускаем заголовок
		}

		if len(record) < 8 {
			continue // Пропускаем неполные записи
		}

		id, _ := strconv.Atoi(record[0])
		year, _ := strconv.Atoi(record[6])
		revenue, _ := strconv.ParseFloat(record[7], 64)

		manufacturers = append(manufacturers, model.Manufacturer{
			ID:          id,
			Name:        record[1],
			Country:     record[2],
			Address:     record[3],
			Phone:       record[4],
			Email:       record[5],
			ProductType: record[6],
			FoundedYear: year,
			Revenue:     revenue,
		})
	}

	c.manufacturers = manufacturers
	c.currentFile = filePath
	return nil
}

func (c *ManufacturerController) SaveToFile(filePath string) error {
	// Создаем директорию если ее нет
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return fmt.Errorf("не удалось создать директорию: %v", err)
	}

	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("не удалось создать файл: %v", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Записываем заголовки
	headers := []string{"ID", "Name", "Country", "Address", "Phone", "Email", "ProductType", "FoundedYear", "Revenue"}
	if err := writer.Write(headers); err != nil {
		return fmt.Errorf("ошибка записи заголовков: %v", err)
	}

	// Записываем данные
	for _, m := range c.manufacturers {
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
			return fmt.Errorf("ошибка записи данных: %v", err)
		}
	}

	c.currentFile = filePath
	return nil
}

func (c *ManufacturerController) ExportToPDF(filePath string) error {
	// Здесь должна быть реализация экспорта в PDF
	// В реальном приложении можно использовать библиотеку типа gofpdf
	return errors.New("PDF export not implemented yet")
}

func (c *ManufacturerController) Print() error {
	// Здесь должна быть реализация печати
	// В реальном приложении можно использовать системные вызовы печати
	return errors.New("printing not implemented yet")
}

func (c *ManufacturerController) GenerateChart(column string) ([]byte, error) {
	// Здесь должна быть реализация генерации графика
	// В реальном приложении можно использовать библиотеку типа gonum/plot
	return nil, errors.New("chart generation not implemented yet")
}

func (c *ManufacturerController) SortBy(column string, ascending bool) {
	sort.Slice(c.manufacturers, func(i, j int) bool {
		switch column {
		case "id":
			if ascending {
				return c.manufacturers[i].ID < c.manufacturers[j].ID
			}
			return c.manufacturers[i].ID > c.manufacturers[j].ID
		case "name":
			if ascending {
				return c.manufacturers[i].Name < c.manufacturers[j].Name
			}
			return c.manufacturers[i].Name > c.manufacturers[j].Name
		case "country":
			if ascending {
				return c.manufacturers[i].Country < c.manufacturers[j].Country
			}
			return c.manufacturers[i].Country > c.manufacturers[j].Country
		case "foundedYear":
			if ascending {
				return c.manufacturers[i].FoundedYear < c.manufacturers[j].FoundedYear
			}
			return c.manufacturers[i].FoundedYear > c.manufacturers[j].FoundedYear
		case "revenue":
			if ascending {
				return c.manufacturers[i].Revenue < c.manufacturers[j].Revenue
			}
			return c.manufacturers[i].Revenue > c.manufacturers[j].Revenue
		default:
			return false
		}
	})
}

func (c *ManufacturerController) NewDatabase() {
	c.manufacturers = []model.Manufacturer{}
	c.currentFile = ""
}

func (c *ManufacturerController) AddManufacturer(m *model.Manufacturer) error {
	// Добавляем в память
	c.manufacturers = append(c.manufacturers, *m)

	// Сохраняем в файл
	if c.currentFile != "" {
		return c.SaveToFile(c.currentFile)
	}
	return nil
}

func (c *ManufacturerController) SetCurrentFile(path string) {
	c.currentFile = path
}

func (c *ManufacturerController) GetCurrentFile() string {
	return c.currentFile
}

func (c *ManufacturerController) HasUnsavedChanges() bool {
	// Здесь можно добавить логику для определения несохраненных изменений
	return false
}

func (c *ManufacturerController) GetManufacturerByRow(row int) (*model.Manufacturer, error) {
	if row < 0 || row >= len(c.manufacturers) {
		return nil, errors.New("invalid row index")
	}
	return &c.manufacturers[row], nil
}

func (c *ManufacturerController) GetNextID() int {
	maxID := 0
	for _, m := range c.manufacturers {
		if m.ID > maxID {
			maxID = m.ID
		}
	}
	return maxID + 1
}

func (c *ManufacturerController) GetManufacturerByIndex(index int) (*model.Manufacturer, error) {
	if index < 0 || index >= len(c.manufacturers) {
		return nil, errors.New("invalid index")
	}
	return &c.manufacturers[index], nil
}

// In controller/manufacturer_controller.go
func (c *ManufacturerController) GetManufacturers() []model.Manufacturer {
	return c.manufacturers
}
