package view

import (
	"cursovay/internal/controller"
	"cursovay/internal/model"
	"cursovay/pkg/localization"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne"
	"fyne.io/fyne/app"
	"fyne.io/fyne/container"
	"fyne.io/fyne/dialog"
	"fyne.io/fyne/storage"
	"fyne.io/fyne/widget"
)

type MainWindow struct {
	app        fyne.App
	window     fyne.Window
	controller *controller.ManufacturerController
	locale     *localization.Locale
	table      *widget.Table
}

func NewMainWindow(controller *controller.ManufacturerController, locale *localization.Locale) *MainWindow {
	if controller == nil {
		log.Fatal("Контроллер не инициализирован")
	}
	a := app.New()
	w := a.NewWindow(locale.Translate("Manufacturers Database"))

	return &MainWindow{
		app:        a,
		window:     w,
		controller: controller,
		locale:     locale,
	}
}

func (mw *MainWindow) Show() {
	mw.table = mw.createManufacturersTable()

	fileMenu := fyne.NewMenu(mw.locale.Translate("File"),
		fyne.NewMenuItem(mw.locale.Translate("Open"), mw.onOpen),
		fyne.NewMenuItem(mw.locale.Translate("Save"), mw.onSave),
		fyne.NewMenuItem(mw.locale.Translate("Exit"), func() {
			mw.app.Quit()
		}),
	)

	helpMenu := fyne.NewMenu(mw.locale.Translate("Help"),
		fyne.NewMenuItem(mw.locale.Translate("About"), mw.showAboutDialog),
	)

	mainMenu := fyne.NewMainMenu(fileMenu, helpMenu)
	mw.window.SetMainMenu(mainMenu)

	mw.window.SetContent(container.NewBorder(nil, nil, nil, nil, mw.table))
	mw.window.Resize(fyne.NewSize(800, 600))
	mw.window.ShowAndRun()
}

func (mw *MainWindow) onOpen() {
	// Создаем фильтр для CSV файлов
	filter := storage.NewExtensionFileFilter([]string{".csv"})

	// Получаем текущую рабочую директорию
	dir, err := os.Getwd()
	if err != nil {
		log.Println("Не удалось получить текущую директорию:", err)
		dir = ""
	}

	// Создаем URI для начальной директории
	dirURI := storage.NewFileURI(dir)
	listableDirURI, err := storage.ListerForURI(dirURI)
	if err != nil {
		log.Println("Не удалось создать listable URI:", err)
		listableDirURI = nil
	}

	// Создаем диалог открытия файла
	fileDialog := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil {
			dialog.ShowError(err, mw.window)
			return
		}
		if reader == nil {
			return // Пользователь отменил
		}
		defer reader.Close()

		// Получаем путь к файлу
		fileURI := reader.URI()
		filePath := strings.TrimPrefix(fileURI.String(), "file://")

		// Загружаем данные из файла
		if err := mw.controller.LoadFromFile(filePath); err != nil {
			dialog.ShowError(err, mw.window)
			return
		}

		mw.refreshTable()
		dialog.ShowInformation("Файл открыт", "Данные загружены из: "+filePath, mw.window)
	}, mw.window)

	// Настраиваем диалог
	fileDialog.SetFilter(filter)
	if listableDirURI != nil {
		fileDialog.SetLocation(listableDirURI)
	}
	fileDialog.Show()
}

func (mw *MainWindow) onSave() {
	// Создаем фильтр для CSV файлов
	filter := storage.NewExtensionFileFilter([]string{".csv"})

	// Получаем текущую рабочую директорию
	dir, err := os.Getwd()
	if err != nil {
		log.Println("Не удалось получить текущую директорию:", err)
		dir = ""
	}

	// Создаем URI для начальной директории
	dirURI := storage.NewFileURI(dir)
	listableDirURI, err := storage.ListerForURI(dirURI)
	if err != nil {
		log.Println("Не удалось создать listable URI:", err)
		listableDirURI = nil
	}

	// Создаем диалог сохранения файла
	saveDialog := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
		if err != nil {
			dialog.ShowError(err, mw.window)
			return
		}
		if writer == nil {
			return // Пользователь отменил
		}
		defer writer.Close()

		// Получаем путь к файлу
		fileURI := writer.URI()
		filePath := strings.TrimPrefix(fileURI.String(), "file://")

		// Добавляем расширение .csv, если его нет
		if !strings.HasSuffix(strings.ToLower(filePath), ".csv") {
			filePath += ".csv"
		}

		// Сохраняем данные в файл
		if err := mw.controller.SaveToFile(filePath); err != nil {
			dialog.ShowError(err, mw.window)
			return
		}

		dialog.ShowInformation("Файл сохранен", "Данные сохранены в: "+filePath, mw.window)
	}, mw.window)

	// Настраиваем диалог
	saveDialog.SetFilter(filter)
	if listableDirURI != nil {
		saveDialog.SetLocation(listableDirURI)
	}

	// Устанавливаем имя файла по умолчанию
	if dir != "" {
		defaultFileURI := storage.NewFileURI(filepath.Join(dir, "manufacturers.csv"))
		if listableDefaultURI, err := storage.ListerForURI(defaultFileURI); err == nil {
			saveDialog.SetLocation(listableDefaultURI)
		}
	}

	saveDialog.Show()
}

// Реализация диалога "О программе"
func (mw *MainWindow) showAboutDialog() {
	aboutText := `Производители строительных материалов
Версия 1.0.0

Автор: Поляков Кирилл Дмитриевич
Группа: ИЦТМС-2-2
НИУ МГСУ`

	dialog.ShowCustom(
		"О программе",
		"Закрыть",
		container.NewVBox(
			widget.NewLabel(aboutText),
			widget.NewLabel("© 2023"),
		),
		mw.window,
	)
}

// Обновление таблицы с данными
func (mw *MainWindow) refreshTable() {
	manufacturers, err := mw.controller.GetAllManufacturers()
	if err != nil {
		dialog.ShowError(err, mw.window)
		return
	}
	if manufacturers == nil {
		dialog.ShowError(fmt.Errorf("нет доступных данных"), mw.window)
		return
	}

	mw.table = mw.createManufacturersTableWithData(manufacturers)
	mw.window.SetContent(container.NewBorder(nil, nil, nil, nil, mw.table))
}

// Создание таблицы с данными
func (mw *MainWindow) createManufacturersTable() *widget.Table {
	// Получаем данные из контроллера
	manufacturers, err := mw.controller.GetAllManufacturers()
	if err != nil {
		dialog.ShowError(err, mw.window)
		return widget.NewTable(nil, nil, nil)
	}

	return mw.createManufacturersTableWithData(manufacturers)
}

func (mw *MainWindow) createManufacturersTableWithData(data []model.Manufacturer) *widget.Table {
	return widget.NewTable(
		func() (int, int) { return len(data), 8 }, // 8 - количество столбцов
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(tci widget.TableCellID, co fyne.CanvasObject) {
			if tci.Row < len(data) {
				manufacturer := data[tci.Row]
				switch tci.Col {
				case 0:
					co.(*widget.Label).SetText(manufacturer.Name)
				case 1:
					co.(*widget.Label).SetText(manufacturer.Country)
				case 2:
					co.(*widget.Label).SetText(manufacturer.Address)
				case 3:
					co.(*widget.Label).SetText(manufacturer.Phone)
				case 4:
					co.(*widget.Label).SetText(manufacturer.Email)
				case 5:
					co.(*widget.Label).SetText(manufacturer.ProductType)
				case 6:
					co.(*widget.Label).SetText(fmt.Sprintf("%d", manufacturer.FoundedYear))
				case 7:
					co.(*widget.Label).SetText(fmt.Sprintf("%.2f", manufacturer.Revenue))
				}
			}
		},
	)
}
