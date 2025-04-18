package controller

import (
	"cursovay/internal/model"
	"cursovay/internal/repository"
	"cursovay/internal/service"
)

type ManufacturerController struct {
	service *service.ManufacturerService
}

func NewManufacturerController(repo *repository.ManufacturerRepository) *ManufacturerController {
	return &ManufacturerController{
		service: service.NewManufacturerService(repo),
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
	// Логика загрузки из файла
	return nil
}

func (c *ManufacturerController) SaveToFile(filePath string) error {
	// Логика сохранения в файл
	return nil
}
