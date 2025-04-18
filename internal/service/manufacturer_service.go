package service

import (
	"bytes"
	"cursovay/internal/model"
	"cursovay/internal/repository"
	"html/template"
	"os/exec"
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

func (s *ManufacturerService) ExportToPDF(filePath string) error {
	data, err := s.repo.GetAll()
	if err != nil {
		return err
	}

	tmpl := `... HTML шаблон для PDF ...`
	t := template.Must(template.New("pdf").Parse(tmpl))

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return err
	}

	// Используем wkhtmltopdf для конвертации HTML в PDF
	cmd := exec.Command("wkhtmltopdf", "-", filePath)
	cmd.Stdin = &buf
	return cmd.Run()
}

func (s *ManufacturerService) Print() error {
	// Логика для печати
	return nil
}

func (s *ManufacturerService) GenerateChart(column string) ([]byte, error) {
	// Логика для генерации графика
	return nil, nil
}
