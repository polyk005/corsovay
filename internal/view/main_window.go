package view

import (
	"cursovay/internal/controller"
	"cursovay/internal/model"
	"cursovay/pkg/localization"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne"
	"fyne.io/fyne/container"
	"fyne.io/fyne/dialog"
	"fyne.io/fyne/storage"
	"fyne.io/fyne/widget"
)

type MainWindow struct {
	app            fyne.App
	window         fyne.Window
	table          *widget.Table
	mainContainer  *fyne.Container
	tableContainer *fyne.Container
	controller     *controller.ManufacturerController
	locale         *localization.Locale
	currentFile    string
	unsavedChanges bool
	selectedRow    int
}

func NewMainWindow(app fyne.App, controller *controller.ManufacturerController, locale *localization.Locale) *MainWindow {
	if controller == nil {
		log.Fatal("Контроллер не инициализирован")
	}

	w := app.NewWindow(locale.Translate("База данных производителей"))

	// Инициализация пустой базы данных
	controller.NewDatabase()

	return &MainWindow{
		app:            app,
		window:         w,
		controller:     controller,
		locale:         locale,
		currentFile:    "",
		unsavedChanges: false,
	}
}

func (mw *MainWindow) setupMenu() *fyne.MainMenu {
	fileMenu := fyne.NewMenu(mw.locale.Translate("File"),
		fyne.NewMenuItem(mw.locale.Translate("New"), mw.onCreateNewFile),
		fyne.NewMenuItem(mw.locale.Translate("Open"), mw.onOpen),
		fyne.NewMenuItem(mw.locale.Translate("Save"), mw.onSave),
		fyne.NewMenuItem(mw.locale.Translate("Save As"), mw.onSaveAsWithPrompt),
		fyne.NewMenuItem(mw.locale.Translate("Export to PDF"), mw.onExportPDF),
		fyne.NewMenuItem(mw.locale.Translate("Print"), mw.onPrint),
		fyne.NewMenuItem(mw.locale.Translate("Exit"), func() {
			mw.checkUnsavedChanges(func() {
				mw.app.Quit()
			})
		}),
	)

	editMenu := fyne.NewMenu(mw.locale.Translate("Edit"),
		fyne.NewMenuItem(mw.locale.Translate("Add"), mw.onAdd),
		fyne.NewMenuItem(mw.locale.Translate("Edit"), func() { mw.onEdit(-1) }),
		fyne.NewMenuItem(mw.locale.Translate("Delete"), func() { mw.onDelete(-1) }),
	)

	viewMenu := fyne.NewMenu(mw.locale.Translate("View"),
		fyne.NewMenuItem(mw.locale.Translate("Chart"), mw.onShowChart),
	)

	langMenu := fyne.NewMenu(mw.locale.Translate("Language"),
		fyne.NewMenuItem("English", func() { mw.changeLanguage("en") }),
		fyne.NewMenuItem("Русский", func() { mw.changeLanguage("ru") }),
	)

	return fyne.NewMainMenu(
		fileMenu,
		editMenu,
		viewMenu,
		langMenu,
	)
}

func (mw *MainWindow) checkUnsavedChanges(callback func()) {
	if !mw.unsavedChanges {
		callback()
		return
	}

	dialog.ShowConfirm(
		mw.locale.Translate("Unsaved Changes"),
		mw.locale.Translate("You have unsaved changes. Do you want to save them?"),
		func(save bool) {
			if save {
				mw.onSave()
			}
			callback()
		},
		mw.window,
	)
}

func (mw *MainWindow) Show() {
	mw.window.SetMainMenu(mw.setupMenu())

	// Initialize with empty table
	mw.table = mw.createManufacturersTable()
	scroll := container.NewScroll(mw.table)

	mw.mainContainer = container.NewBorder(
		nil, nil, nil, nil,
		scroll,
	)

	mw.window.SetContent(mw.mainContainer)
	mw.window.Resize(fyne.NewSize(1000, 600))
	mw.window.ShowAndRun()
}

func (mw *MainWindow) onNew() {
	mw.checkUnsavedChanges(func() {
		// Создаем новую пустую базу
		mw.controller.NewDatabase()
		mw.currentFile = ""
		mw.unsavedChanges = false
		mw.refreshTable()
		mw.updateWindowTitle()
		mw.showNotification("Создана новая база данных")
	})
}

func (mw *MainWindow) onOpen() {
	mw.checkUnsavedChanges(func() {
		fileDialog := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil {
				dialog.ShowError(fmt.Errorf("ошибка при выборе файла: %v", err), mw.window)
				return
			}
			if reader == nil {
				return // Пользователь отменил
			}
			defer reader.Close()

			filePath := uriToPath(reader.URI())

			if !strings.HasSuffix(strings.ToLower(filePath), ".csv") {
				dialog.ShowError(errors.New("выберите файл с расширением .csv"), mw.window)
				return
			}

			if err := mw.controller.LoadFromFile(filePath); err != nil {
				dialog.ShowError(fmt.Errorf("ошибка загрузки файла:\n%v", err), mw.window)
				return
			}

			mw.currentFile = filePath
			mw.unsavedChanges = false
			mw.refreshTable()
			mw.updateWindowTitle()
			mw.showNotification("Файл успешно открыт")
		}, mw.window)

		fileDialog.SetFilter(storage.NewExtensionFileFilter([]string{".csv"}))

		// Устанавливаем начальную директорию
		if mw.currentFile != "" {
			dirURI := storage.NewFileURI(filepath.Dir(mw.currentFile))
			listableURI, _ := storage.ListerForURI(dirURI)
			fileDialog.SetLocation(listableURI)
		} else {
			// Директория по умолчанию - домашняя
			homeDir, _ := os.UserHomeDir()
			homeURI := storage.NewFileURI(homeDir)
			listableURI, _ := storage.ListerForURI(homeURI)
			fileDialog.SetLocation(listableURI)
		}

		fileDialog.Show()
	})
}

func (mw *MainWindow) onSave() {
	if mw.currentFile == "" {
		mw.onSaveAs()
		return
	}

	// Проверяем, существует ли файл
	if _, err := os.Stat(mw.currentFile); os.IsNotExist(err) {
		dialog.ShowConfirm(
			mw.locale.Translate("File not found"),
			mw.locale.Translate("The file does not exist. Save as new file?"),
			func(ok bool) {
				if ok {
					mw.onSaveAs()
				}
			},
			mw.window,
		)
		return
	}

	if err := mw.controller.SaveToFile(mw.currentFile); err != nil {
		dialog.ShowError(err, mw.window)
		return
	}

	mw.unsavedChanges = false
	mw.showNotification(mw.locale.Translate("File saved successfully"))
}

func (mw *MainWindow) onSaveAs() {
	saveDialog := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
		if err != nil {
			dialog.ShowError(err, mw.window)
			return
		}
		if writer == nil {
			return // Пользователь отменил
		}
		defer writer.Close()

		filePath := uriToPath(writer.URI())
		if !strings.HasSuffix(strings.ToLower(filePath), ".csv") {
			filePath += ".csv"
		}

		if err := mw.controller.SaveToFile(filePath); err != nil {
			dialog.ShowError(err, mw.window)
			return
		}

		mw.currentFile = filePath
		mw.unsavedChanges = false
		mw.window.SetTitle(mw.locale.Translate("Manufacturers Database") + " - " + filepath.Base(filePath))
		mw.showNotification(mw.locale.Translate("File saved successfully"))
	}, mw.window)

	// Устанавливаем фильтр для CSV файлов
	saveDialog.SetFilter(storage.NewExtensionFileFilter([]string{".csv"}))

	// Устанавливаем начальное расположение
	if mw.currentFile != "" {
		fileURI := storage.NewFileURI(mw.currentFile)
		listableURI, _ := storage.ListerForURI(fileURI)
		saveDialog.SetLocation(listableURI)
	} else {
		// Установка расположения по умолчанию
		homeDir, _ := os.UserHomeDir()
		defaultPath := filepath.Join(homeDir, "manufacturers.csv")
		defaultURI := storage.NewFileURI(defaultPath)
		listableURI, _ := storage.ListerForURI(defaultURI)
		saveDialog.SetLocation(listableURI)
	}

	saveDialog.Show()
}

func (mw *MainWindow) onCreateNewFile() {
	// Сначала создаем новую базу данных
	mw.controller.NewDatabase()
	mw.currentFile = ""
	mw.unsavedChanges = false
	mw.refreshTable()

	// Затем сразу предлагаем сохранить
	mw.onSaveAsWithPrompt()
}

func (mw *MainWindow) onSaveAsWithPrompt() {
	saveDialog := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
		if err != nil {
			dialog.ShowError(fmt.Errorf("ошибка сохранения: %v", err), mw.window)
			return
		}
		if writer == nil {
			return // Пользователь отменил
		}
		defer writer.Close()

		// Получаем путь к файлу
		filePath := uriToPath(writer.URI())
		if filePath == "" {
			dialog.ShowError(errors.New("не удалось получить путь к файлу"), mw.window)
			return
		}

		// Добавляем расширение .csv если его нет
		if !strings.HasSuffix(strings.ToLower(filePath), ".csv") {
			filePath += ".csv"
		}

		// Сохраняем данные
		if err := mw.controller.SaveToFile(filePath); err != nil {
			dialog.ShowError(fmt.Errorf("не удалось сохранить файл: %v", err), mw.window)
			return
		}

		mw.currentFile = filePath
		mw.unsavedChanges = false
		mw.updateWindowTitle()
		mw.showNotification("Файл успешно сохранён")
	}, mw.window)

	// Настраиваем фильтр для CSV файлов
	saveDialog.SetFilter(storage.NewExtensionFileFilter([]string{".csv"}))

	// Устанавливаем начальную директорию
	if mw.currentFile != "" {
		dirPath := filepath.Dir(mw.currentFile)
		dirURI := storage.NewFileURI(dirPath)
		listableURI, _ := storage.ListerForURI(dirURI)
		saveDialog.SetLocation(listableURI)
	} else {
		// Предлагаем стандартное имя файла в домашней директории
		homeDir, _ := os.UserHomeDir()
		defaultName := fmt.Sprintf("производители_%s.csv", time.Now().Format("2006-01-02"))
		defaultPath := filepath.Join(homeDir, defaultName)
		defaultURI := storage.NewFileURI(defaultPath)
		listableURI, _ := storage.ListerForURI(defaultURI)
		saveDialog.SetLocation(listableURI)
	}

	saveDialog.Show()
}

func (mw *MainWindow) onExportPDF() {
	// Здесь должна быть реализация экспорта в PDF
	dialog.ShowInformation("Export PDF", "PDF export will be implemented here", mw.window)
}

func (mw *MainWindow) onPrint() {
	if len(mw.controller.GetManufacturers()) == 0 {
		dialog.ShowInformation(
			mw.locale.Translate("No Data"),
			mw.locale.Translate("No manufacturers to print. Please load data first."),
			mw.window,
		)
		return
	}

	// Create a print dialog
	printDialog := dialog.NewCustomConfirm(
		mw.locale.Translate("Print"),
		mw.locale.Translate("Print"),
		mw.locale.Translate("Cancel"),
		widget.NewLabel(mw.locale.Translate("Print current manufacturer list?")),
		func(print bool) {
			if print {
				// Actual printing implementation would go here
				// For now just show a message
				dialog.ShowInformation(
					mw.locale.Translate("Print"),
					mw.locale.Translate("Printing functionality would be implemented here"),
					mw.window,
				)
			}
		},
		mw.window,
	)
	printDialog.Show()
}

func (mw *MainWindow) onAdd() {
	newManufacturer := model.Manufacturer{
		// Инициализируем ID (можно добавить логику генерации ID)
		ID: mw.controller.GetNextID(),
	}
	mw.showEditDialog(&newManufacturer, true)
}

func (mw *MainWindow) onEdit(row int) {
	if row == -1 {
		row = mw.selectedRow
	}

	if row <= 0 {
		dialog.ShowInformation(
			mw.locale.Translate("No Selection"),
			mw.locale.Translate("Please select a manufacturer first"),
			mw.window,
		)
		return
	}

	if len(mw.controller.GetManufacturers()) == 0 {
		dialog.ShowInformation(
			mw.locale.Translate("No Data"),
			mw.locale.Translate("No manufacturers available. Please load data first."),
			mw.window,
		)
		return
	}

	manufacturer, err := mw.controller.GetManufacturerByRow(row - 1)
	if err != nil {
		dialog.ShowError(err, mw.window)
		return
	}

	mw.showEditDialog(manufacturer, false)
}

func (mw *MainWindow) onDelete(row int) {
	if row == -1 {
		row = mw.selectedRow
	}

	if row <= 0 {
		dialog.ShowInformation(
			mw.locale.Translate("No Selection"),
			mw.locale.Translate("Please select a manufacturer first"),
			mw.window,
		)
		return
	}

	manufacturer, err := mw.controller.GetManufacturerByIndex(row - 1)
	if err != nil {
		dialog.ShowError(err, mw.window)
		return
	}

	mw.onDeleteWithConfirmation(manufacturer.ID)
}

func (mw *MainWindow) showEditDialog(manufacturer *model.Manufacturer, isNew bool) {
	nameEntry := widget.NewEntry()
	nameEntry.SetText(manufacturer.Name)

	countryEntry := widget.NewEntry()
	countryEntry.SetText(manufacturer.Country)

	addressEntry := widget.NewEntry()
	addressEntry.SetText(manufacturer.Address)

	phoneEntry := widget.NewEntry()
	phoneEntry.SetText(manufacturer.Phone)

	emailEntry := widget.NewEntry()
	emailEntry.SetText(manufacturer.Email)

	productTypeEntry := widget.NewEntry()
	productTypeEntry.SetText(manufacturer.ProductType)

	foundedYearEntry := widget.NewEntry()
	foundedYearEntry.SetText(fmt.Sprintf("%d", manufacturer.FoundedYear))

	revenueEntry := widget.NewEntry()
	revenueEntry.SetText(fmt.Sprintf("%.2f", manufacturer.Revenue))

	form := &widget.Form{
		Items: []*widget.FormItem{
			{Text: mw.locale.Translate("Name"), Widget: nameEntry},
			{Text: mw.locale.Translate("Country"), Widget: countryEntry},
			{Text: mw.locale.Translate("Address"), Widget: addressEntry},
			{Text: mw.locale.Translate("Phone"), Widget: phoneEntry},
			{Text: mw.locale.Translate("Email"), Widget: emailEntry},
			{Text: mw.locale.Translate("Product Type"), Widget: productTypeEntry},
			{Text: mw.locale.Translate("Founded Year"), Widget: foundedYearEntry},
			{Text: mw.locale.Translate("Revenue"), Widget: revenueEntry},
		},
		OnSubmit: func() {
			// Валидация данных
			if nameEntry.Text == "" {
				dialog.ShowError(errors.New(mw.locale.Translate("Name cannot be empty")), mw.window)
				return
			}

			var firstErr error

			year, firstErr := strconv.Atoi(foundedYearEntry.Text)
			if firstErr != nil {
				dialog.ShowError(errors.New(mw.locale.Translate("Invalid year format")), mw.window)
				return
			}

			revenue, firstErr := strconv.ParseFloat(revenueEntry.Text, 64)
			if firstErr != nil {
				dialog.ShowError(errors.New(mw.locale.Translate("Invalid revenue format")), mw.window)
				return
			}

			// Обновляем данные производителя
			manufacturer.Name = nameEntry.Text
			manufacturer.Country = countryEntry.Text
			manufacturer.Address = addressEntry.Text
			manufacturer.Phone = phoneEntry.Text
			manufacturer.Email = emailEntry.Text
			manufacturer.ProductType = productTypeEntry.Text
			manufacturer.FoundedYear = year
			manufacturer.Revenue = revenue

			var err error
			if isNew {
				err = mw.controller.AddManufacturer(manufacturer)
			} else {
				err = mw.controller.UpdateManufacturer(manufacturer)
			}

			if err != nil {
				dialog.ShowError(err, mw.window)
				return
			}

			mw.unsavedChanges = true
			mw.refreshTable()
			dialog.ShowInformation(
				mw.locale.Translate("Success"),
				mw.locale.Translate("Manufacturer saved successfully"),
				mw.window,
			)
		},
	}

	dialogTitle := mw.locale.Translate("Edit Manufacturer")
	if isNew {
		dialogTitle = mw.locale.Translate("Add New Manufacturer")
	}

	dialog.ShowCustomConfirm(
		dialogTitle,
		mw.locale.Translate("Save"),
		mw.locale.Translate("Cancel"),
		form,
		func(b bool) {
			if b {
				form.OnSubmit()
			}
		},
		mw.window,
	)
}

func (mw *MainWindow) onDeleteWithConfirmation(id int) {
	manufacturer, err := mw.controller.GetManufacturerByID(id)
	if err != nil {
		dialog.ShowError(err, mw.window)
		return
	}

	dialog.ShowConfirm(
		mw.locale.Translate("Delete Manufacturer"),
		fmt.Sprintf(mw.locale.Translate("Are you sure you want to delete %s?"), manufacturer.Name),
		func(confirm bool) {
			if confirm {
				if err := mw.controller.DeleteManufacturer(id); err != nil {
					dialog.ShowError(err, mw.window)
					return
				}
				mw.unsavedChanges = true
				mw.refreshTable()
				mw.showNotification(mw.locale.Translate("Manufacturer deleted successfully"))
			}
		},
		mw.window,
	)
}

func (mw *MainWindow) onShowChart() {
	// Здесь должна быть реализация отображения графика
	dialog.ShowInformation("Chart", "Chart visualization will be implemented here", mw.window)
}

func (mw *MainWindow) changeLanguage(lang string) {
	localesDir := filepath.Join("assets", "locales") // Укажите правильный путь
	if err := mw.locale.SetLanguage(lang, localesDir); err != nil {
		log.Printf("Ошибка смены языка: %v", err)
		return
	}

	// Обновляем меню
	mw.window.SetMainMenu(mw.setupMenu())

	// Обновляем таблицу
	mw.refreshTable()

	// Обновляем заголовок окна
	mw.window.SetTitle(mw.locale.Translate("Manufacturers Database"))
}

func (mw *MainWindow) showAboutDialog() {
	aboutText := fmt.Sprintf(`%s
%s 1.0.0

%s: Поляков Кирилл Дмитриевич
%s: ИЦТМС-2-2
НИУ МГСУ`,
		mw.locale.Translate("Construction Materials Manufacturers"),
		mw.locale.Translate("Version"),
		mw.locale.Translate("Author"),
		mw.locale.Translate("Group"))

	dialog.ShowCustom(
		mw.locale.Translate("About"),
		mw.locale.Translate("Close"),
		container.NewVBox(
			widget.NewLabel(aboutText),
			widget.NewLabel("© 2025"),
		),
		mw.window,
	)
}

func (mw *MainWindow) showNotification(message string) {
	notification := fyne.NewNotification(
		mw.locale.Translate("Notification"),
		message,
	)
	mw.app.SendNotification(notification)
}

func (mw *MainWindow) refreshTable() {
	if mw.mainContainer == nil {
		return
	}

	// Получаем текущий Scroll контейнер
	oldScroll, ok := mw.mainContainer.Objects[0].(*container.Scroll)
	if !ok {
		return
	}

	// Сохраняем позицию прокрутки
	scrollOffset := int(oldScroll.Offset.Y)

	// Создаем новую таблицу
	newTable := mw.createManufacturersTable()
	mw.table = newTable

	// Создаем новый Scroll контейнер
	newScroll := container.NewScroll(newTable)
	newScroll.Offset.Y = scrollOffset

	// Обновляем главный контейнер
	mw.mainContainer.Objects[0] = newScroll
	mw.mainContainer.Refresh()
}

func (mw *MainWindow) createManufacturersTable() *widget.Table {
	manufacturers, err := mw.controller.GetAllManufacturers()
	if err != nil {
		dialog.ShowError(err, mw.window)
		return widget.NewTable(nil, nil, nil)
	}

	table := widget.NewTable(
		func() (int, int) {
			return len(manufacturers) + 1, 8 // +1 для заголовков
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("template")
		},
		func(tci widget.TableCellID, co fyne.CanvasObject) {
			label := co.(*widget.Label)
			label.Wrapping = fyne.TextTruncate

			if tci.Row == 0 {
				switch tci.Col {
				case 0:
					label.SetText(mw.locale.Translate("ID"))
				case 1:
					label.SetText(mw.locale.Translate("Name"))
				case 2:
					label.SetText(mw.locale.Translate("Country"))
				case 3:
					label.SetText(mw.locale.Translate("Address"))
				case 4:
					label.SetText(mw.locale.Translate("Phone"))
				case 5:
					label.SetText(mw.locale.Translate("Email"))
				case 6:
					label.SetText(mw.locale.Translate("Product Type"))
				case 7:
					label.SetText(mw.locale.Translate("Revenue"))
				}
				label.TextStyle.Bold = true
				return
			}

			if tci.Row-1 < len(manufacturers) {
				m := manufacturers[tci.Row-1]
				switch tci.Col {
				case 0:
					label.SetText(fmt.Sprintf("%d", m.ID))
				case 1:
					label.SetText(m.Name)
				case 2:
					label.SetText(m.Country)
				case 3:
					label.SetText(m.Address)
				case 4:
					label.SetText(m.Phone)
				case 5:
					label.SetText(m.Email)
				case 6:
					label.SetText(m.ProductType)
				case 7:
					label.SetText(fmt.Sprintf("%.2f", m.Revenue))
				}
			}
		},
	)

	// Настройка ширины столбцов
	table.SetColumnWidth(0, 80)  // ID - фиксированная
	table.SetColumnWidth(1, 200) // Name - растягивается
	table.SetColumnWidth(2, 150) // Country - фиксированная
	table.SetColumnWidth(3, 250) // Address - растягивается
	table.SetColumnWidth(4, 150) // Phone - фиксированная
	table.SetColumnWidth(5, 200) // Email - растягивается
	table.SetColumnWidth(6, 200) // Product Type - растягивается
	table.SetColumnWidth(7, 120) // Revenue - фиксированная

	table.OnSelected = func(id widget.TableCellID) {
		if id.Row > 0 { // Игнорируем заголовки
			mw.selectedRow = id.Row
		}
	}

	return table
}

func uriToPath(uri fyne.URI) string {
	if uri == nil {
		return ""
	}
	// Для file:// URI просто убираем префикс
	path := uri.String()
	if uri.Scheme() == "file" {
		path = strings.TrimPrefix(path, "file://")
		// На Windows убираем лишний слеш
		if runtime.GOOS == "windows" && strings.HasPrefix(path, "/") {
			path = path[1:]
		}
	}
	return path
}

func (mw *MainWindow) updateWindowTitle() {
	title := mw.locale.Translate("База данных производителей")
	if mw.currentFile != "" {
		title += " - " + filepath.Base(mw.currentFile)
	}
	mw.window.SetTitle(title)
}
