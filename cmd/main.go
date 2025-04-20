package main

import (
	"cursovay/internal/controller"
	"cursovay/internal/repository"
	"cursovay/internal/view"
	"cursovay/pkg/localization"
	"log"
	"path/filepath"
	"runtime"

	"fyne.io/fyne/app"
)

func main() {
	myApp := app.NewWithID("ru.mydomain.proizvoditeli")

	// Определяем пути
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		log.Fatal("Не удалось определить путь к проекту")
	}
	rootDir := filepath.Dir(filepath.Dir(filename))
	localesDir := filepath.Join(rootDir, "assets", "locales")

	// Инициализация локализации
	locale, err := localization.NewLocale(localesDir)
	if err != nil {
		log.Fatalf("Ошибка загрузки локализации: %v", err)
	}

	repo := repository.NewManufacturerRepository("")
	controller := controller.NewManufacturerController(repo)

	// Инициализация главного окна
	mainWindow := view.NewMainWindow(myApp, controller, locale)
	mainWindow.Show()
}
