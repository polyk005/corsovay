package service

import (
	"cursovay/internal/model"
	"cursovay/internal/repository"
)

type ManufacturerService struct {
	repo *repository.ManufacturerRepository
}

func NewManufacturerService(repo *repository.ManufacturerRepository) *ManufacturerService {
	return &ManufacturerService{
		repo: repo,
	}
}

func (s *ManufacturerService) GetAll() ([]model.Manufacturer, error) {
	// В реальной реализации здесь должна быть бизнес-логика
	// Пока просто возвращаем данные из репозитория
	return s.repo.GetAll()
}

func (s *ManufacturerService) GetByID(id int) (*model.Manufacturer, error) {
	// Реализация получения по ID
	return nil, nil
}

func (s *ManufacturerService) Create(manufacturer *model.Manufacturer) error {
	// Валидация и создание
	if err := manufacturer.Validate(); err != nil {
		return err
	}
	return s.repo.Create(manufacturer)
}

func (s *ManufacturerService) Update(manufacturer *model.Manufacturer) error {
	// Валидация и обновление
	if err := manufacturer.Validate(); err != nil {
		return err
	}
	return s.repo.Update(manufacturer)
}

func (s *ManufacturerService) Delete(id int) error {
	// Удаление
	return s.repo.Delete(id)
}
