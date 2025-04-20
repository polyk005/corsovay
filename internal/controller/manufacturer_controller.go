package controller

import (
	"bytes"
	"cursovay/internal/model"
	"cursovay/internal/repository"
	"cursovay/internal/service"
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jung-kurt/gofpdf"
	"github.com/wcharczuk/go-chart"
)

type ManufacturerController struct {
	service       *service.ManufacturerService
	manufacturers []model.Manufacturer
	currentFile   string
	mu            sync.Mutex
}

func NewManufacturerController(repo *repository.ManufacturerRepository) *ManufacturerController {
	return &ManufacturerController{
		service:       service.NewManufacturerService(repo),
		manufacturers: []model.Manufacturer{},
		currentFile:   "",
		mu:            sync.Mutex{},
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
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, m := range c.manufacturers {
		if m.ID == id {
			// Возвращаем копию, чтобы избежать изменений
			result := *&m
			return &result, nil
		}
	}
	return nil, fmt.Errorf("производитель с ID %d не найден", id)
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
	c.mu.Lock()
	defer c.mu.Unlock()

	// Удаляем через сервис
	if err := c.service.Delete(id); err != nil {
		return err
	}

	// Удаляем из локального кэша и находим индекс удаленного элемента
	deletedIndex := -1
	for i, item := range c.manufacturers {
		if item.ID == id {
			deletedIndex = i
			c.manufacturers = append(c.manufacturers[:i], c.manufacturers[i+1:]...)
			break
		}
	}

	if deletedIndex == -1 {
		return fmt.Errorf("manufacturer with ID %d not found in cache", id)
	}

	// Перенумеровываем оставшиеся записи, начиная с удаленного индекса
	for i := deletedIndex; i < len(c.manufacturers); i++ {
		c.manufacturers[i].ID = i + 1 // ID начинаются с 1
	}

	// Сохраняем изменения, если файл указан
	if c.currentFile != "" {
		if err := c.SaveToFile(c.currentFile); err != nil {
			// Восстанавливаем старые ID при ошибке сохранения
			// (в реальном приложении нужно более сложное восстановление)
			return fmt.Errorf("failed to save after deletion: %v", err)
		}
	}

	return nil
}

func (c *ManufacturerController) forceSaveToFile(filePath string) error {
	// Создаём временный файл
	tempFile := filePath + ".tmp"
	file, err := os.Create(tempFile)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %v", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)

	// Записываем заголовки
	headers := []string{"ID", "Name", "Country", "Address", "Phone", "Email", "ProductType", "FoundedYear", "Revenue"}
	if err := writer.Write(headers); err != nil {
		return fmt.Errorf("failed to write headers: %v", err)
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
			return fmt.Errorf("failed to write record: %v", err)
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return fmt.Errorf("csv writer error: %v", err)
	}

	// Закрываем файл перед переименованием
	file.Close()

	// Заменяем оригинальный файл временным
	if err := os.Rename(tempFile, filePath); err != nil {
		return fmt.Errorf("failed to replace original file: %v", err)
	}

	return nil
}

func (c *ManufacturerController) loadFromFile(filePath string) []model.Manufacturer {
	file, err := os.Open(filePath)
	if err != nil {
		return nil
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil
	}

	var manufacturers []model.Manufacturer
	for i, record := range records {
		if i == 0 { // Пропускаем заголовок
			continue
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

	return manufacturers
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
	c.mu.Lock()
	defer c.mu.Unlock()

	// Создаем новый PDF документ
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()

	// Устанавливаем шрифт и заголовок
	pdf.SetFont("Arial", "B", 16)
	pdf.Cell(40, 10, "Manufacturers Database Report")
	pdf.Ln(12)

	// Добавляем дату генерации
	pdf.SetFont("Arial", "", 10)
	pdf.Cell(40, 10, "Generated: "+time.Now().Format("2006-01-02 15:04:05"))
	pdf.Ln(15)

	// Устанавливаем шрифт для таблицы
	pdf.SetFont("Arial", "B", 12)

	// Заголовки столбцов
	headers := []string{"ID", "Name", "Country", "Revenue", "Product Type"}
	widths := []float64{15, 50, 40, 30, 55}

	// Рисуем заголовки
	for i, header := range headers {
		pdf.CellFormat(widths[i], 7, header, "1", 0, "C", false, 0, "")
	}
	pdf.Ln(-1)

	// Устанавливаем шрифт для данных
	pdf.SetFont("Arial", "", 10)

	// Заполняем таблицу данными
	for _, m := range c.manufacturers {
		pdf.CellFormat(widths[0], 6, strconv.Itoa(m.ID), "1", 0, "", false, 0, "")
		pdf.CellFormat(widths[1], 6, m.Name, "1", 0, "", false, 0, "")
		pdf.CellFormat(widths[2], 6, m.Country, "1", 0, "", false, 0, "")
		pdf.CellFormat(widths[3], 6, strconv.FormatFloat(m.Revenue, 'f', 2, 64), "1", 0, "R", false, 0, "")
		pdf.CellFormat(widths[4], 6, m.ProductType, "1", 0, "", false, 0, "")
		pdf.Ln(-1)
	}

	// Сохраняем PDF в файл
	return pdf.OutputFileAndClose(filePath)
}

func (c *ManufacturerController) Print() error {
	// Сначала экспортируем во временный PDF
	tempFile := "print_temp.pdf"
	if err := c.ExportToPDF(tempFile); err != nil {
		return fmt.Errorf("failed to create print file: %v", err)
	}

	// Определяем команду печати в зависимости от ОС
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("lp", tempFile)
	case "darwin":
		cmd = exec.Command("lpr", tempFile)
	case "windows":
		cmd = exec.Command("print", tempFile)
	default:
		return errors.New("printing not supported on this OS")
	}

	// Выполняем команду печати
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("printing failed: %v", err)
	}

	return nil
}

func (c *ManufacturerController) GenerateChart(column string) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Подготавливаем данные для графика
	var values []chart.Value
	for _, m := range c.manufacturers {
		var value float64
		switch column {
		case "revenue":
			value = m.Revenue
		case "foundedYear":
			value = float64(m.FoundedYear)
		default:
			continue
		}

		// Ограничиваем длину названия для читаемости
		name := m.Name
		if len(name) > 15 {
			name = name[:12] + "..."
		}

		values = append(values, chart.Value{
			Label: name,
			Value: value,
		})
	}

	// Создаем график
	graph := chart.BarChart{
		Title: "Manufacturers by " + column,
		Background: chart.Style{
			Padding: chart.Box{
				Top: 40,
			},
		},
		Height:   512,
		BarWidth: 60,
		Bars:     values,
	}

	// Рендерим в буфер
	buffer := bytes.NewBuffer([]byte{})
	err := graph.Render(chart.PNG, buffer)
	if err != nil {
		return nil, fmt.Errorf("failed to render chart: %v", err)
	}

	return buffer.Bytes(), nil
}

func (c *ManufacturerController) Sort(manufacturers []model.Manufacturer, column string, ascending bool) ([]model.Manufacturer, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(manufacturers) == 0 {
		return manufacturers, nil
	}

	// Создаем копию для сортировки, чтобы не менять оригинальный массив
	sorted := make([]model.Manufacturer, len(manufacturers))
	copy(sorted, manufacturers)

	var sortErr error
	sort.Slice(sorted, func(i, j int) bool {
		switch column {
		case "id":
			return c.compareInt(sorted[i].ID, sorted[j].ID, ascending)
		case "name":
			return c.compareString(sorted[i].Name, sorted[j].Name, ascending)
		case "country":
			return c.compareString(sorted[i].Country, sorted[j].Country, ascending)
		case "address":
			return c.compareString(sorted[i].Address, sorted[j].Address, ascending)
		case "phone":
			return c.compareString(sorted[i].Phone, sorted[j].Phone, ascending)
		case "email":
			return c.compareString(sorted[i].Email, sorted[j].Email, ascending)
		case "productType":
			return c.compareString(sorted[i].ProductType, sorted[j].ProductType, ascending)
		case "foundedYear":
			return c.compareInt(sorted[i].FoundedYear, sorted[j].FoundedYear, ascending)
		case "revenue":
			return c.compareFloat(sorted[i].Revenue, sorted[j].Revenue, ascending)
		default:
			sortErr = fmt.Errorf("неизвестный столбец для сортировки: %s", column)
			return false
		}
	})

	return sorted, sortErr
}

// Вспомогательные методы для сравнения разных типов данных
func (c *ManufacturerController) compareInt(a, b int, ascending bool) bool {
	if ascending {
		return a < b
	}
	return a > b
}

func (c *ManufacturerController) compareString(a, b string, ascending bool) bool {
	aLower := strings.ToLower(a)
	bLower := strings.ToLower(b)
	if ascending {
		return aLower < bLower
	}
	return aLower > bLower
}

func (c *ManufacturerController) compareFloat(a, b float64, ascending bool) bool {
	if ascending {
		return a < b
	}
	return a > b
}

func (c *ManufacturerController) NewDatabase() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.manufacturers = []model.Manufacturer{}
	c.currentFile = ""
}

func (c *ManufacturerController) AddManufacturer(m *model.Manufacturer) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Генерируем новый ID (либо 1 если список пустой, либо maxID + 1)
	if len(c.manufacturers) == 0 {
		m.ID = 1
	} else {
		maxID := c.manufacturers[len(c.manufacturers)-1].ID
		m.ID = maxID + 1
	}

	// Добавляем в список
	c.manufacturers = append(c.manufacturers, *m)

	// Автоматически сохраняем, если файл указан
	if c.currentFile != "" {
		if err := c.SaveToFile(c.currentFile); err != nil {
			// Удаляем добавленный элемент при ошибке сохранения
			c.manufacturers = c.manufacturers[:len(c.manufacturers)-1]
			return fmt.Errorf("failed to save after adding: %v", err)
		}
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
	if c.currentFile == "" {
		// Если файл не загружен, считаем что есть несохраненные изменения
		return len(c.manufacturers) > 0
	}

	// Для реальной проверки можно сравнить текущие данные с содержимым файла
	// Здесь упрощенная версия - всегда считаем что есть изменения после редактирования
	return true
}

func (c *ManufacturerController) GetManufacturerByRow(row int) (*model.Manufacturer, error) {
	if row < 0 || row >= len(c.manufacturers) {
		return nil, errors.New("invalid row index")
	}
	return &c.manufacturers[row], nil
}

func (c *ManufacturerController) GetNextID() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.manufacturers) == 0 {
		return 1
	}
	return c.manufacturers[len(c.manufacturers)-1].ID + 1
}

func (c *ManufacturerController) GetManufacturerByIndex(index int) (*model.Manufacturer, error) {
	if index < 0 || index >= len(c.manufacturers) {
		return nil, errors.New("invalid index")
	}
	return &c.manufacturers[index], nil
}

// In controller/manufacturer_controller.go
func (c *ManufacturerController) GetManufacturers() []model.Manufacturer {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Создаем копию, чтобы избежать изменений извне
	manufacturers := make([]model.Manufacturer, len(c.manufacturers))
	copy(manufacturers, c.manufacturers)
	return manufacturers
}

func (c *ManufacturerController) Search(query string) ([]model.Manufacturer, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	query = strings.ToLower(query)
	var results []model.Manufacturer

	for _, m := range c.manufacturers {
		if strings.Contains(strings.ToLower(m.Name), query) ||
			strings.Contains(strings.ToLower(m.Country), query) ||
			strings.Contains(strings.ToLower(m.Address), query) ||
			strings.Contains(m.Phone, query) ||
			strings.Contains(strings.ToLower(m.Email), query) ||
			strings.Contains(strings.ToLower(m.ProductType), query) ||
			strings.Contains(fmt.Sprintf("%d", m.FoundedYear), query) ||
			strings.Contains(fmt.Sprintf("%.2f", m.Revenue), query) {
			results = append(results, m)
		}
	}

	return results, nil
}

// SetManufacturers устанавливает список производителей
func (c *ManufacturerController) SetManufacturers(manufacturers []model.Manufacturer) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.manufacturers = manufacturers
}

func (c *ManufacturerController) GetPrintableData() ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.manufacturers) == 0 {
		return nil, errors.New("база данных пуста")
	}

	// Создаем PDF документ
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 16)
	pdf.Cell(40, 10, "База данных производителей")
	pdf.Ln(12)

	// Заголовки таблицы
	headers := []string{"ID", "Название", "Страна", "Адрес", "Телефон", "Email", "Тип продукции", "Год осн.", "Доход"}
	widths := []float64{10, 30, 20, 40, 25, 40, 30, 15, 20}

	// Устанавливаем шрифт для таблицы
	pdf.SetFont("Arial", "B", 10)
	for i, header := range headers {
		pdf.CellFormat(widths[i], 7, header, "1", 0, "C", false, 0, "")
	}
	pdf.Ln(-1)

	// Данные таблицы
	pdf.SetFont("Arial", "", 8)
	for _, m := range c.manufacturers {
		pdf.CellFormat(widths[0], 6, strconv.Itoa(m.ID), "1", 0, "", false, 0, "")
		pdf.CellFormat(widths[1], 6, m.Name, "1", 0, "", false, 0, "")
		pdf.CellFormat(widths[2], 6, m.Country, "1", 0, "", false, 0, "")
		pdf.CellFormat(widths[3], 6, m.Address, "1", 0, "", false, 0, "")
		pdf.CellFormat(widths[4], 6, m.Phone, "1", 0, "", false, 0, "")
		pdf.CellFormat(widths[5], 6, m.Email, "1", 0, "", false, 0, "")
		pdf.CellFormat(widths[6], 6, m.ProductType, "1", 0, "", false, 0, "")
		pdf.CellFormat(widths[7], 6, strconv.Itoa(m.FoundedYear), "1", 0, "", false, 0, "")
		pdf.CellFormat(widths[8], 6, fmt.Sprintf("%.2f", m.Revenue), "1", 0, "R", false, 0, "")
		pdf.Ln(-1)
	}

	// Сохраняем PDF в буфер
	var buf bytes.Buffer
	err := pdf.Output(&buf)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
