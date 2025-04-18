package controller

import (
	"cursovay/internal/model"
	"cursovay/internal/repository"
	"cursovay/internal/service"
	"encoding/csv"
	"errors"
	"os"
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
	if err := c.service.Update(m); err != nil {
		return err
	}
	for i, manufacturer := range c.manufacturers {
		if manufacturer.ID == m.ID {
			c.manufacturers[i] = *m
			break
		}
	}
	return nil
}

func (c *ManufacturerController) DeleteManufacturer(id int) error {
	if err := c.service.Delete(id); err != nil {
		return err
	}
	for i, manufacturer := range c.manufacturers {
		if manufacturer.ID == id {
			c.manufacturers = append(c.manufacturers[:i], c.manufacturers[i+1:]...)
			break
		}
	}
	return nil
}

func (c *ManufacturerController) LoadFromFile(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Comma = ','
	reader.FieldsPerRecord = 8 // 8 полей в каждой строке

	records, err := reader.ReadAll()
	if err != nil {
		return err
	}

	var manufacturers []model.Manufacturer
	for i, record := range records {
		if i == 0 {
			continue // Пропускаем заголовок
		}

		id, err := strconv.Atoi(record[0])
		if err != nil {
			return err
		}

		foundedYear, err := strconv.Atoi(record[6])
		if err != nil {
			return err
		}

		revenue, err := strconv.ParseFloat(record[7], 64)
		if err != nil {
			return err
		}

		manufacturer := model.Manufacturer{
			ID:          id,
			Name:        record[1],
			Country:     record[2],
			Address:     record[3],
			Phone:       record[4],
			Email:       record[5],
			ProductType: record[6],
			FoundedYear: foundedYear,
			Revenue:     revenue,
		}
		manufacturers = append(manufacturers, manufacturer)
	}

	c.manufacturers = manufacturers
	c.currentFile = filePath
	return nil
}

func (c *ManufacturerController) SaveToFile(filePath string) error {
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Записываем заголовки
	headers := []string{
		"ID",
		"Name",
		"Country",
		"Address",
		"Phone",
		"Email",
		"ProductType",
		"FoundedYear",
		"Revenue",
	}
	if err := writer.Write(headers); err != nil {
		return err
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
			return err
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

func (c *ManufacturerController) AddManufacturer(manufacturer *model.Manufacturer) error {
	// Генерируем новый ID
	maxID := 0
	for _, m := range c.manufacturers {
		if m.ID > maxID {
			maxID = m.ID
		}
	}
	manufacturer.ID = maxID + 1

	// Добавляем в сервис и в локальный кэш
	if err := c.service.Create(manufacturer); err != nil {
		return err
	}
	c.manufacturers = append(c.manufacturers, *manufacturer)
	return nil
}

func (c *ManufacturerController) GetCurrentFile() string {
	return c.currentFile
}

func (c *ManufacturerController) HasUnsavedChanges() bool {
	// Здесь можно добавить логику для определения несохраненных изменений
	return false
}
