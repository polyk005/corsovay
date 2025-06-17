package main

import (
	"cursovay/internal/controller"
	"cursovay/internal/repository"
	"cursovay/internal/view"
	"cursovay/pkg/localization"
	"log"
	"os"
	"path/filepath"

	"fyne.io/fyne/app"
)

func main() {
	myApp := app.NewWithID("ru.mydomain.proizvoditeli")

	// Определяем путь к системной директории для локализации
	configDir, err := os.UserConfigDir()
	if err != nil {
		log.Fatalf("Не удалось определить системную директорию: %v", err)
	}
	localesDir := filepath.Join(configDir, "ManufacturersDB", "locales")

	// Создаем директорию, если она не существует
	if err := os.MkdirAll(localesDir, 0755); err != nil {
		log.Printf("Ошибка создания директории локализации: %v", err)
	}

	// Инициализация локализации
	locale, err := localization.NewLocale(localesDir)
	if err != nil {
		log.Printf("Ошибка загрузки локализации: %v", err)
		// Создаем пустую локализацию, если файлы не найдены
		locale = &localization.Locale{
			Translations: make(map[string]string),
			Language:     "ru",
		}
	}

	repo := repository.NewManufacturerRepository("")
	controller := controller.NewManufacturerController(repo)

	// Загружаем локализацию для графиков
	if err := controller.LoadLocalization("ru"); err != nil {
		log.Printf("Ошибка загрузки локализации графиков: %v", err)
		// Продолжаем работу, будут использованы значения по умолчанию
	}

	// Инициализация главного окна
	mainWindow := view.NewMainWindow(myApp, controller, locale)
	mainWindow.Show()
}
