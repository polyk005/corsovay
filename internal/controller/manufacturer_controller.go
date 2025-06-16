package controller

import (
	"bytes"
	"cursovay/internal/model"
	"cursovay/internal/repository"
	"cursovay/internal/service"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"image/color"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jung-kurt/gofpdf"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/draw"
)

type ManufacturerController struct {
	service       *service.ManufacturerService
	manufacturers []model.Manufacturer
	currentFile   string
	mu            sync.RWMutex
}

// ColoredBox implements the plot.Thumbnailer interface for legend
type ColoredBox struct {
	Color color.Color
}

func (b *ColoredBox) Thumbnail(c *draw.Canvas) {
	pts := []vg.Point{
		{X: c.Min.X, Y: c.Min.Y},
		{X: c.Min.X, Y: c.Max.Y},
		{X: c.Max.X, Y: c.Max.Y},
		{X: c.Max.X, Y: c.Min.Y},
	}
	c.FillPolygon(b.Color, pts)
}

// ChartLabels implements the plotter.XYLabeller interface
type ChartLabels struct {
	XYs    plotter.XYs
	Labels []string
}

func (l ChartLabels) Len() int {
	return len(l.XYs)
}

func (l ChartLabels) XY(i int) (x, y float64) {
	return l.XYs[i].X, l.XYs[i].Y
}

func (l ChartLabels) Label(i int) string {
	if i >= 0 && i < len(l.Labels) {
		return l.Labels[i]
	}
	return ""
}

// ChartConfig содержит настройки для графиков
type ChartConfig struct {
	Width       vg.Length // в пикселях
	Height      vg.Length // в пикселях
	FontSize    vg.Length
	BarWidth    vg.Length
	MarginTop   vg.Length
	MarginRight vg.Length
	MarginLeft  vg.Length
	MarginBot   vg.Length
}

// DefaultChartConfig возвращает конфигурацию по умолчанию
func DefaultChartConfig() ChartConfig {
	return ChartConfig{
		Width:       600,
		Height:      800,
		FontSize:    12,
		BarWidth:    20,
		MarginTop:   10,
		MarginRight: 10,
		MarginLeft:  10,
		MarginBot:   10,
	}
}

// ChartLocalization содержит локализованные строки для графиков
type ChartLocalization struct {
	Title   string `json:"title"`
	XLabel  string `json:"x_label,omitempty"`
	YLabel  string `json:"y_label,omitempty"`
}

type ChartsLocalization struct {
	RevenueBar    ChartLocalization `json:"revenue_bar"`
	FoundedBar    ChartLocalization `json:"founded_bar"`
	ProductPie    ChartLocalization `json:"product_pie"`
	RevenueTrend  ChartLocalization `json:"revenue_trend"`
}

type Localization struct {
	Charts ChartsLocalization `json:"charts"`
}

var currentLocalization Localization

func (c *ManufacturerController) LoadLocalization(lang string) error {
	file, err := os.Open(fmt.Sprintf("localization/%s.json", lang))
	if err != nil {
		return fmt.Errorf("failed to open localization file: %v", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&currentLocalization); err != nil {
		return fmt.Errorf("failed to decode localization: %v", err)
	}

	return nil
}

func NewManufacturerController(repo *repository.ManufacturerRepository) *ManufacturerController {
	return &ManufacturerController{
		service:       service.NewManufacturerService(repo),
		manufacturers: []model.Manufacturer{},
		currentFile:   "",
		mu:            sync.RWMutex{},
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

func (c *ManufacturerController) FileExists(filePath string) bool {
	if _, err := os.Stat(filePath); err == nil {
		return true
	}
	return false
}

func (c *ManufacturerController) LoadFromFile(filePath string) ([]model.Manufacturer, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("ошибка открытия файла: %v", err) // Изменено на return nil, error
	}
	defer file.Close()

	// Создаем CSV reader
	r := csv.NewReader(file)
	r.Comma = ','             // Указываем разделитель
	r.TrimLeadingSpace = true // Убираем пробелы в начале поля

	// Читаем заголовок
	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения заголовка: %v", err) // Изменено на return nil, error
	}
	if len(header) != 9 {
		return nil, fmt.Errorf("невалидный CSV-заголовок") // Изменено на return nil, error
	}

	var manufacturers []model.Manufacturer
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue // Пропускаем битые строки
		}

		if len(record) != 9 {
			continue
		}

		// Парсинг полей
		id, _ := strconv.Atoi(record[0])
		year, _ := strconv.Atoi(record[7])
		revenue, _ := strconv.ParseFloat(record[8], 64)

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
	return manufacturers, nil // Возвращаем оба значения
}

func (c *ManufacturerController) GetCurrentData() []model.Manufacturer {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.manufacturers
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

func (c *ManufacturerController) UpdateManufacturers(data []model.Manufacturer) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.manufacturers = data
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

func (c *ManufacturerController) ExportToJSON(filePath string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Convert manufacturers to JSON
	jsonData, err := json.MarshalIndent(c.manufacturers, "", "    ")
	if err != nil {
		return fmt.Errorf("failed to marshal data to JSON: %v", err)
	}

	// Write to file
	if err := os.WriteFile(filePath, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write JSON file: %v", err)
	}

	return nil
}

func (c *ManufacturerController) GenerateChart(params map[string]interface{}) ([]byte, error) {
	// Получаем параметры
	chartType := params["type"].(string)
	colorScheme := params["colorScheme"].(string)
	showValues := params["showValues"].(bool)
	sortData := params["sortData"].(bool)

	// Получаем данные
	manufacturers := c.GetCurrentData()
	if len(manufacturers) == 0 {
		return nil, fmt.Errorf("no data available")
	}

	switch chartType {
	case "revenue_bar":
		return c.generateRevenueBarChart(manufacturers, colorScheme, showValues, sortData)
	case "founded_bar":
		return c.generateFoundedYearBarChart(manufacturers, colorScheme, showValues, sortData)
	case "product_pie":
		return c.generateProductTypePieChart(manufacturers, colorScheme, showValues)
	case "revenue_line":
		return c.generateRevenueTrendChart(manufacturers, colorScheme, showValues, sortData)
	default:
		return nil, fmt.Errorf("unknown chart type: %s", chartType)
	}
}

func (c *ManufacturerController) generateRevenueBarChart(manufacturers []model.Manufacturer, colorScheme string, showValues, sortData bool) ([]byte, error) {
	config := DefaultChartConfig()
	
	// Создаем новый график
	p := plot.New()
	
	// Устанавливаем размеры и отступы
	p.X.Label.TextStyle.Font.Size = config.FontSize
	p.Y.Label.TextStyle.Font.Size = config.FontSize
	p.Title.TextStyle.Font.Size = config.FontSize + 2
	p.Legend.TextStyle.Font.Size = config.FontSize - 2
	
	p.X.Label.Padding = config.MarginBot
	p.Y.Label.Padding = config.MarginLeft
	
	// Устанавливаем заголовки из локализации
	p.Title.Text = currentLocalization.Charts.RevenueBar.Title
	p.X.Label.Text = currentLocalization.Charts.RevenueBar.XLabel
	p.Y.Label.Text = currentLocalization.Charts.RevenueBar.YLabel

	// Подготавливаем данные
	var values []float64
	var labels []string
	for _, m := range manufacturers {
		values = append(values, m.Revenue)
		labels = append(labels, m.Name)
	}

	// Сортируем данные если нужно
	if sortData {
		// Создаем временный слайс для сортировки
		type kv struct {
			Value float64
			Label string
		}
		sorted := make([]kv, len(values))
		for i := range values {
			sorted[i] = kv{values[i], labels[i]}
		}
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].Value > sorted[j].Value
		})
		// Обновляем исходные слайсы
		for i := range sorted {
			values[i] = sorted[i].Value
			labels[i] = sorted[i].Label
		}
	}

	// Создаем столбчатый график
	bars, err := plotter.NewBarChart(plotter.Values(values), vg.Points(20))
	if err != nil {
		return nil, err
	}

	// Применяем цветовую схему
	switch colorScheme {
	case "Blue Theme":
		bars.Color = color.RGBA{0, 0, 255, 255}
	case "Green Theme":
		bars.Color = color.RGBA{0, 255, 0, 255}
	case "Rainbow":
		// В новой версии нет поддержки множества цветов для баров,
		// поэтому используем один цвет
		bars.Color = color.RGBA{255, 0, 0, 255}
	}

	// Добавляем значения если нужно
	if showValues {
		for i, v := range values {
			labelXYs := plotter.XYs{{X: float64(i), Y: v}}
			labels, err := plotter.NewLabels(&ChartLabels{
				XYs:    labelXYs,
				Labels: []string{fmt.Sprintf("%.2f", v)},
			})
			if err != nil {
				return nil, err
			}
			p.Add(labels)
		}
	}

	p.Add(bars)
	p.NominalX(labels...)

	// Сохраняем график в буфер с фиксированным размером
	w := new(bytes.Buffer)
	wt, err := p.WriterTo(config.Width, config.Height, "png")
	if err != nil {
		return nil, err
	}
	_, err = wt.WriteTo(w)
	if err != nil {
		return nil, err
	}

	return w.Bytes(), nil
}

func (c *ManufacturerController) generateFoundedYearBarChart(manufacturers []model.Manufacturer, colorScheme string, showValues, sortData bool) ([]byte, error) {
	config := DefaultChartConfig()
	
	// Создаем новый график
	p := plot.New()
	
	// Устанавливаем размеры и отступы
	p.X.Label.TextStyle.Font.Size = config.FontSize
	p.Y.Label.TextStyle.Font.Size = config.FontSize
	p.Title.TextStyle.Font.Size = config.FontSize + 2
	p.Legend.TextStyle.Font.Size = config.FontSize - 2
	
	p.X.Label.Padding = config.MarginBot
	p.Y.Label.Padding = config.MarginLeft
	
	// Устанавливаем заголовки из локализации
	p.Title.Text = currentLocalization.Charts.FoundedBar.Title
	p.X.Label.Text = currentLocalization.Charts.FoundedBar.XLabel
	p.Y.Label.Text = currentLocalization.Charts.FoundedBar.YLabel

	// Подготавливаем данные
	var values []float64
	var labels []string
	for _, m := range manufacturers {
		values = append(values, float64(m.FoundedYear))
		labels = append(labels, m.Name)
	}

	// Сортируем данные если нужно
	if sortData {
		type kv struct {
			Value float64
			Label string
		}
		sorted := make([]kv, len(values))
		for i := range values {
			sorted[i] = kv{values[i], labels[i]}
		}
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].Value < sorted[j].Value
		})
		for i := range sorted {
			values[i] = sorted[i].Value
			labels[i] = sorted[i].Label
		}
	}

	// Создаем столбчатый график
	bars, err := plotter.NewBarChart(plotter.Values(values), vg.Points(20))
	if err != nil {
		return nil, err
	}

	// Применяем цветовую схему
	switch colorScheme {
	case "Blue Theme":
		bars.Color = color.RGBA{0, 0, 255, 255}
	case "Green Theme":
		bars.Color = color.RGBA{0, 255, 0, 255}
	case "Rainbow":
		bars.Color = color.RGBA{255, 0, 0, 255}
	}

	// Добавляем значения если нужно
	if showValues {
		for i, v := range values {
			labelXYs := plotter.XYs{{X: float64(i), Y: v}}
			labels, err := plotter.NewLabels(&ChartLabels{
				XYs:    labelXYs,
				Labels: []string{fmt.Sprintf("%.0f", v)},
			})
			if err != nil {
				return nil, err
			}
			p.Add(labels)
		}
	}

	p.Add(bars)
	p.NominalX(labels...)

	// Сохраняем график в буфер с фиксированным размером
	w := new(bytes.Buffer)
	wt, err := p.WriterTo(config.Width, config.Height, "png")
	if err != nil {
		return nil, err
	}
	_, err = wt.WriteTo(w)
	if err != nil {
		return nil, err
	}

	return w.Bytes(), nil
}

func (c *ManufacturerController) generateProductTypePieChart(manufacturers []model.Manufacturer, colorScheme string, showValues bool) ([]byte, error) {
	config := DefaultChartConfig()
	// Для круговой диаграммы используем квадратный размер
	if config.Width > config.Height {
		config.Width = config.Height
	} else {
		config.Height = config.Width
	}
	
	// Создаем новый график
	p := plot.New()
	
	// Устанавливаем размеры и отступы
	p.Title.TextStyle.Font.Size = config.FontSize + 2
	p.Legend.TextStyle.Font.Size = config.FontSize - 2
	
	// Устанавливаем заголовки из локализации
	p.Title.Text = currentLocalization.Charts.ProductPie.Title

	// Создаем карту для подсчета количества каждого типа продукции
	productCounts := make(map[string]float64)
	for _, m := range manufacturers {
		productCounts[m.ProductType]++
	}

	// Создаем данные для круговой диаграммы
	var values plotter.Values
	var labels []string

	// Создаем слайсы для сортировки
	type kv struct {
		Key   string
		Value float64
	}
	var sorted []kv
	for k, v := range productCounts {
		sorted = append(sorted, kv{k, v})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Value > sorted[j].Value
	})

	// Добавляем секторы
	colors := []color.Color{
		color.RGBA{255, 0, 0, 255},
		color.RGBA{0, 255, 0, 255},
		color.RGBA{0, 0, 255, 255},
		color.RGBA{255, 255, 0, 255},
		color.RGBA{255, 0, 255, 255},
		color.RGBA{0, 255, 255, 255},
	}

	total := 0.0
	for _, item := range sorted {
		total += item.Value
	}

	for i, item := range sorted {
		value := item.Value
		percentage := value / total
		values = append(values, percentage)
		labels = append(labels, item.Key)
		colorIndex := i % len(colors)
		if showValues {
			label := fmt.Sprintf("%s\n%.1f%%", item.Key, percentage*100)
			// Create a custom legend entry with a colored box
			p.Legend.Add(label, &ColoredBox{
				Color: colors[colorIndex],
			})
		}
	}

	// Создаем круговую диаграмму
	pie, err := plotter.NewBarChart(values, vg.Points(20))
	if err != nil {
		return nil, err
	}
	pie.Color = color.RGBA{0, 0, 255, 255}

	p.Add(pie)
	p.NominalX(labels...)

	// Сохраняем график в буфер с фиксированным размером
	w := new(bytes.Buffer)
	wt, err := p.WriterTo(config.Width, config.Height, "png")
	if err != nil {
		return nil, err
	}
	_, err = wt.WriteTo(w)
	if err != nil {
		return nil, err
	}

	return w.Bytes(), nil
}

func (c *ManufacturerController) generateRevenueTrendChart(manufacturers []model.Manufacturer, colorScheme string, showValues, sortData bool) ([]byte, error) {
	config := DefaultChartConfig()
	// Для графика тренда делаем более широкий размер
	config.Width = 800
	config.Height = 600
	
	// Создаем новый график
	p := plot.New()
	
	// Устанавливаем размеры и отступы
	p.X.Label.TextStyle.Font.Size = config.FontSize
	p.Y.Label.TextStyle.Font.Size = config.FontSize
	p.Title.TextStyle.Font.Size = config.FontSize + 2
	p.Legend.TextStyle.Font.Size = config.FontSize - 2
	
	p.X.Label.Padding = config.MarginBot
	p.Y.Label.Padding = config.MarginLeft
	
	// Устанавливаем заголовки из локализации
	p.Title.Text = currentLocalization.Charts.RevenueTrend.Title
	p.X.Label.Text = currentLocalization.Charts.RevenueTrend.XLabel
	p.Y.Label.Text = currentLocalization.Charts.RevenueTrend.YLabel

	// Создаем точки для графика
	pts := make(plotter.XYs, len(manufacturers))
	for i, m := range manufacturers {
		pts[i].X = float64(m.FoundedYear)
		pts[i].Y = m.Revenue
	}

	// Сортируем точки по году основания если нужно
	if sortData {
		sort.Slice(pts, func(i, j int) bool {
			return pts[i].X < pts[j].X
		})
	}

	// Создаем линейный график
	line, err := plotter.NewLine(pts)
	if err != nil {
		return nil, err
	}

	// Создаем точки на графике
	scatter, err := plotter.NewScatter(pts)
	if err != nil {
		return nil, err
	}

	// Применяем цветовую схему
	switch colorScheme {
	case "Blue Theme":
		line.Color = color.RGBA{0, 0, 255, 255}
		scatter.Color = color.RGBA{0, 0, 200, 255}
	case "Green Theme":
		line.Color = color.RGBA{0, 255, 0, 255}
		scatter.Color = color.RGBA{0, 200, 0, 255}
	case "Rainbow":
		line.Color = color.RGBA{255, 0, 0, 255}
		scatter.Color = color.RGBA{0, 0, 255, 255}
	default:
		line.Color = color.RGBA{0, 0, 0, 255}
		scatter.Color = color.RGBA{0, 0, 0, 255}
	}

	// Добавляем значения если нужно
	if showValues {
		labelStrings := make([]string, len(pts))
		for i := range pts {
			labelStrings[i] = fmt.Sprintf("%.2f", pts[i].Y)
		}
		labels, err := plotter.NewLabels(&ChartLabels{
			XYs:    pts,
			Labels: labelStrings,
		})
		if err != nil {
			return nil, err
		}
		p.Add(labels)
	}

	p.Add(line, scatter)
	p.Add(plotter.NewGrid())

	// Сохраняем график в буфер с фиксированным размером
	w := new(bytes.Buffer)
	wt, err := p.WriterTo(config.Width, config.Height, "png")
	if err != nil {
		return nil, err
	}
	_, err = wt.WriteTo(w)
	if err != nil {
		return nil, err
	}

	return w.Bytes(), nil
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

// GetUniqueProductTypes возвращает список уникальных типов продукции
func (c *ManufacturerController) GetUniqueProductTypes() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Используем map для хранения уникальных значений
	uniqueTypes := make(map[string]bool)
	
	// Собираем все уникальные типы продукции
	for _, m := range c.manufacturers {
		if m.ProductType != "" {
			uniqueTypes[m.ProductType] = true
		}
	}

	// Преобразуем map в slice
	result := make([]string, 0, len(uniqueTypes))
	for productType := range uniqueTypes {
		result = append(result, productType)
	}

	// Сортируем результат для удобства использования
	sort.Strings(result)

	return result
}
