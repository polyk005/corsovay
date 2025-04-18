package main

import (
	"cursovay/internal/controller"
	"cursovay/internal/repository"
	"cursovay/internal/view"
	"cursovay/pkg/localization"
	"log"
	"path/filepath"
	"runtime"
)

func main() {
	// Получаем путь к корню проекта
	_, filename, _, _ := runtime.Caller(0)
	rootDir := filepath.Dir(filepath.Dir(filename))

	// Формируем путь к файлам локализации
	localesPath := filepath.Join(rootDir, "assets", "locales", "ru.json")

	// Инициализация локализации
	locale, err := localization.NewLocale(localesPath)
	if err != nil {
		log.Fatalf("Не удалось загрузить локализацию: %v\nПроверьте путь: %s", err, localesPath)
	}

	// Инициализация репозитория
	repo := repository.NewManufacturerRepository(filepath.Join(rootDir, "data", "manufacturers.csv"))

	// Инициализация контроллера
	ctrl := controller.NewManufacturerController(repo)

	// Создание и отображение главного окна
	mainWindow := view.NewMainWindow(ctrl, locale)
	mainWindow.Show()
}
