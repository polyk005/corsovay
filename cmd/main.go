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
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		log.Fatal("Не удалось определить путь к проекту")
	}
	rootDir := filepath.Dir(filepath.Dir(filename))

	// Формируем путь к директории с локализациями
	localesDir := filepath.Join(rootDir, "assets", "locales")

	// Инициализация локализации (можно передавать "en" или "ru")
	locale, err := localization.NewLocale(localesDir)
	if err != nil {
		log.Fatalf("Не удалось загрузить локализацию: %v\nПроверьте путь: %s", err, localesDir)
	}

	// Остальной код без изменений
	repo := repository.NewManufacturerRepository(filepath.Join(rootDir, "data", "manufacturers.csv"))
	ctrl := controller.NewManufacturerController(repo)
	mainWindow := view.NewMainWindow(ctrl, locale)
	mainWindow.Show()
}
