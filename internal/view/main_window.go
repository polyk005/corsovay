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
	table := widget.NewTable(
		func() (int, int) {
			return len(data) + 1, 8 // +1 для заголовков, 8 столбцов
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("Шаблонный текст для определения размера")
		},
		func(tci widget.TableCellID, co fyne.CanvasObject) {
			label := co.(*widget.Label)
			label.Wrapping = fyne.TextTruncate // Обрезаем длинный текст

			// Заголовки столбцов
			if tci.Row == 0 {
				switch tci.Col {
				case 0:
					label.SetText(mw.locale.Translate("Name"))
				case 1:
					label.SetText(mw.locale.Translate("Country"))
				case 2:
					label.SetText(mw.locale.Translate("Address"))
				case 3:
					label.SetText(mw.locale.Translate("Phone"))
				case 4:
					label.SetText(mw.locale.Translate("Email"))
				case 5:
					label.SetText(mw.locale.Translate("Product Type"))
				case 6:
					label.SetText(mw.locale.Translate("Founded Year"))
				case 7:
					label.SetText(mw.locale.Translate("Revenue"))
				}
				label.TextStyle.Bold = true
				return
			}

			// Данные
			if tci.Row-1 < len(data) {
				manufacturer := data[tci.Row-1]
				switch tci.Col {
				case 0:
					label.SetText(manufacturer.Name)
				case 1:
					label.SetText(manufacturer.Country)
				case 2:
					label.SetText(manufacturer.Address)
				case 3:
					label.SetText(manufacturer.Phone)
				case 4:
					label.SetText(manufacturer.Email)
				case 5:
					label.SetText(manufacturer.ProductType)
				case 6:
					label.SetText(fmt.Sprintf("%d", manufacturer.FoundedYear))
				case 7:
					label.SetText(fmt.Sprintf("%.2f", manufacturer.Revenue))
				}
			}
		},
	)

	// Настраиваем размеры столбцов
	table.SetColumnWidth(0, 150) // Название
	table.SetColumnWidth(1, 100) // Страна
	table.SetColumnWidth(2, 200) // Адрес
	table.SetColumnWidth(3, 120) // Телефон
	table.SetColumnWidth(4, 200) // Email
	table.SetColumnWidth(5, 200) // Тип продукции
	table.SetColumnWidth(6, 100) // Год основания
	table.SetColumnWidth(7, 100) // Доход

	return table
}
