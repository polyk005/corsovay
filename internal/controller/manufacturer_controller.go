package controller

import (
	"bufio"
	"cursovay/internal/model"
	"cursovay/internal/repository"
	"cursovay/internal/service"
	"os"
	"strconv"
	"strings"
)

type ManufacturerController struct {
	service       *service.ManufacturerService
	manufacturers []model.Manufacturer
}

func NewManufacturerController(repo *repository.ManufacturerRepository) *ManufacturerController {
	return &ManufacturerController{
		service:       service.NewManufacturerService(repo),
		manufacturers: []model.Manufacturer{},
	}
}

func (c *ManufacturerController) GetAllManufacturers() ([]model.Manufacturer, error) {
	return c.service.GetAll()
}

func (c *ManufacturerController) GetManufacturerByID(id int) (*model.Manufacturer, error) {
	return c.service.GetByID(id)
}

func (c *ManufacturerController) CreateManufacturer(m *model.Manufacturer) error {
	return c.service.Create(m)
}

func (c *ManufacturerController) UpdateManufacturer(m *model.Manufacturer) error {
	return c.service.Update(m)
}

func (c *ManufacturerController) DeleteManufacturer(id int) error {
	return c.service.Delete(id)
}

func (c *ManufacturerController) LoadFromFile(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	var manufacturers []model.Manufacturer
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Split(line, ",")
		if len(fields) != 9 { // Убедитесь, что у вас 9 полей
			continue // Пропустить строки с неправильным количеством полей
		}

		// Преобразуем данные в нужные типы
		id, _ := strconv.Atoi(fields[0])
		foundedYear, _ := strconv.Atoi(fields[7])
		revenue, _ := strconv.ParseFloat(fields[8], 64)

		manufacturer := model.Manufacturer{
			ID:          id,
			Name:        fields[1],
			Country:     fields[2],
			Address:     fields[3],
			Phone:       fields[4],
			Email:       fields[5],
			ProductType: fields[6],
			FoundedYear: foundedYear,
			Revenue:     revenue,
		}
		manufacturers = append(manufacturers, manufacturer)
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	c.manufacturers = manufacturers
	return nil
}

func (c *ManufacturerController) SaveToFile(filePath string) error {
	// Логика сохранения в файл
	return nil
}
