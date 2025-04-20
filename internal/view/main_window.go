package view

import (
	"bytes"
	"cursovay/internal/connect"
	"cursovay/internal/controller"
	"cursovay/internal/model"
	"cursovay/pkg/localization"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne"
	"fyne.io/fyne/container"
	"fyne.io/fyne/dialog"
	"fyne.io/fyne/layout"
	"fyne.io/fyne/storage"
	"fyne.io/fyne/theme"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/widget"
	"github.com/google/uuid"
)

type MainWindow struct {
	app             fyne.App
	window          fyne.Window
	currentTabIndex int
	table           *widget.Table
	currentDoc      *connect.Document
	searchEntry     *widget.Entry
	searchResults   []model.Manufacturer
	tabs            *container.AppTabs
	isSearching     bool
	currentSort     struct {
		column    string
		ascending bool
	}
	controller     *controller.ManufacturerController
	locale         *localization.Locale
	currentFile    string
	unsavedChanges bool
	selectedRow    int
}

type document struct {
	ID            string
	FilePath      string
	Controller    *controller.ManufacturerController
	Manufacturers []model.Manufacturer
	Unsaved       bool
	LastModified  time.Time
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
		tabs:           container.NewAppTabs(),
	}
}

func newDocument(filePath string, ctrl *controller.ManufacturerController) *document {
	return &document{
		ID:           uuid.New().String(),
		FilePath:     filePath,
		Controller:   ctrl,
		Unsaved:      filePath == "",
		LastModified: time.Now(),
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

	helpMenu := fyne.NewMenu(mw.locale.Translate("About Program"),
		fyne.NewMenuItem(mw.locale.Translate("Help"), mw.helpMenu),
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
		helpMenu,
	)
}

func (mw *MainWindow) setupTabs() {
	mw.tabs = container.NewAppTabs()
	mw.tabs.OnChanged = func(tab *container.TabItem) {
		// При переключении вкладки обновляем поиск
		if searchContainer, ok := mw.window.Content().(*fyne.Container); ok {
			if searchBox, ok := searchContainer.Objects[0].(*fyne.Container); ok {
				if searchEntry, ok := searchBox.Objects[0].(*widget.Entry); ok {
					if doc := mw.controller.GetActiveDocument(); doc != nil {
						searchEntry.SetText(doc.SearchQuery)
					}
				}
			}
		}
		mw.refreshTable()
	}
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

func (mw *MainWindow) createTabForDocument(doc *connect.Document) *container.TabItem {
	// Создаем таблицу с данными производителей
	table := widget.NewTable(
		func() (int, int) {
			return len(doc.Manufacturers) + 1, 8 // +1 для заголовков
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("template")
		},
		func(tci widget.TableCellID, co fyne.CanvasObject) {
			label := co.(*widget.Label)
			if tci.Row == 0 {
				// Заголовки столбцов
				headers := []string{
					mw.locale.Translate("ID"),
					mw.locale.Translate("Name"),
					mw.locale.Translate("Country"),
					mw.locale.Translate("Address"),
					mw.locale.Translate("Phone"),
					mw.locale.Translate("Email"),
					mw.locale.Translate("Product Type"),
					mw.locale.Translate("Revenue"),
				}
				label.SetText(headers[tci.Col])
				label.TextStyle.Bold = true
			} else {
				// Данные производителей
				m := doc.Manufacturers[tci.Row-1]
				values := []string{
					fmt.Sprintf("%d", m.ID),
					m.Name,
					m.Country,
					m.Address,
					m.Phone,
					m.Email,
					m.ProductType,
					fmt.Sprintf("%.2f", m.Revenue),
				}
				label.SetText(values[tci.Col])
			}
		},
	)

	// Устанавливаем размеры для всех 8 столбцов
	table.SetColumnWidth(0, 60)  // ID
	table.SetColumnWidth(1, 180) // Name
	table.SetColumnWidth(2, 120) // Country
	table.SetColumnWidth(3, 250) // Address
	table.SetColumnWidth(4, 120) // Phone
	table.SetColumnWidth(5, 200) // Email
	table.SetColumnWidth(6, 150) // Product Type
	table.SetColumnWidth(7, 100) // Revenue

	tableScroll := container.NewScroll(table)
	tableScroll.SetMinSize(fyne.NewSize(800, 400))

	// 4. Создаем панель информации о файле
	fileInfo := widget.NewLabel("")
	if doc.FilePath != "" {
		fileInfo.SetText(fmt.Sprintf("Файл: %s | Изменен: %s",
			filepath.Base(doc.FilePath),
			doc.LastModified.Format("2006-01-02 15:04:05")))
	} else {
		fileInfo.SetText("Новый файл (не сохранен)")
	}

	// 5. Создаем кнопку закрытия вкладки
	closeBtn := widget.NewButtonWithIcon("", theme.CancelIcon(), func() {
		mw.closeTabByDocument(doc)
	})
	closeBtn.Importance = widget.LowImportance

	// 6. Собираем нижнюю панель
	bottomPanel := container.NewHBox(
		fileInfo,
		layout.NewSpacer(),
		closeBtn,
	)

	// 7. Создаем основной контейнер
	content := container.NewBorder(
		nil,         // Верх - ничего
		bottomPanel, // Низ - наша панель с информацией и кнопкой
		nil,         // Лево - ничего
		nil,         // Право - ничего
		tableScroll, // Центр - таблица
	)

	// 8. Создаем вкладку
	tabTitle := mw.getTabTitle(doc)
	tab := container.NewTabItem(tabTitle, content)

	return tab
}

func (mw *MainWindow) closeTabByDocument(doc *connect.Document) {
	for i, t := range mw.tabs.Items {
		if t.Text == mw.getTabTitle(doc) {
			mw.tabs.Items = append(mw.tabs.Items[:i], mw.tabs.Items[i+1:]...)
			mw.controller.CloseDocument(doc.ID)

			doc.SearchQuery = ""
			doc.IsSearching = false
			doc.SearchResults = nil

			if len(mw.tabs.Items) == 0 {
				mw.window.Close() // или другое действие по вашему выбору
			}
			break
		}
	}
}

func (mw *MainWindow) refreshTabs() {
	if len(mw.tabs.Items) == 0 {
		mw.onCreateNewFile()
	}
}

func (mw *MainWindow) getTabTitle(doc *connect.Document) string {
	if doc.FilePath == "" {
		return mw.locale.Translate("New Document") + "*"
	}
	return filepath.Base(doc.FilePath) + "*"
}

func (mw *MainWindow) Show() {
	mw.window.SetMainMenu(mw.setupMenu())
	mw.table = mw.createManufacturersTable()
	// 1. Создаем поисковую панель
	searchPanel := mw.setupSearch()

	// 2. Создаем разделитель
	separator := widget.NewSeparator()

	// 3. Собираем верхнюю часть
	top := container.NewVBox(
		searchPanel,
		separator,
	)

	// 4. Собираем основной интерфейс
	mainContent := container.NewBorder(
		top,     // Верх - поиск и разделитель
		nil,     // Низ - ничего
		nil,     // Лево - ничего
		nil,     // Право - ничего
		mw.tabs, // Центр - вкладки
	)

	mw.window.SetContent(mainContent)
	mw.window.Resize(fyne.NewSize(1000, 600))
	mw.window.ShowAndRun()
}

func (mw *MainWindow) onOpen() {
	fileDialog := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil {
			dialog.ShowError(err, mw.window)
			return
		}
		if reader == nil {
			return
		}
		defer reader.Close()

		filePath := uriToPath(reader.URI())

		manufacturers, err := mw.controller.LoadDataFromFile(filePath)
		if err != nil {
			dialog.ShowError(err, mw.window)
			return
		}

		doc := connect.NewDocument(filePath, manufacturers)
		mw.controller.AddDocument(doc)

		tab := mw.createTabForDocument(doc)
		mw.tabs.Append(tab)
		mw.currentTabIndex = len(mw.tabs.Items) - 1

	}, mw.window)

	fileDialog.SetFilter(storage.NewExtensionFileFilter([]string{".csv"}))
	fileDialog.Show()
}

func (mw *MainWindow) onSave() {
	doc := mw.controller.GetActiveDocument()
	if doc == nil {
		return
	}

	if doc.FilePath == "" {
		mw.onSaveAs()
		return
	}

	// Сохраняем текущие данные
	err := mw.controller.SaveToFile(doc.FilePath)
	if err != nil {
		dialog.ShowError(err, mw.window)
		return
	}

	doc.Unsaved = false
	doc.LastModified = time.Now()
	mw.updateTabTitle(doc)
	mw.showNotification("Файл сохранен")
}

func (mw *MainWindow) onSaveAs() {
	doc := mw.controller.GetActiveDocument()
	if doc == nil {
		return
	}

	saveDialog := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
		if err != nil {
			dialog.ShowError(err, mw.window)
			return
		}
		if writer == nil {
			return
		}
		defer writer.Close()

		filePath := uriToPath(writer.URI())
		if !strings.HasSuffix(strings.ToLower(filePath), ".csv") {
			filePath += ".csv"
		}

		// Устанавливаем новый путь и сохраняем
		doc.FilePath = filePath
		err = mw.controller.SaveToFile(filePath)
		if err != nil {
			dialog.ShowError(err, mw.window)
			return
		}

		doc.Unsaved = false
		doc.LastModified = time.Now()
		mw.updateTabTitle(doc)
		mw.showNotification("Файл сохранен")
	}, mw.window)

	saveDialog.SetFilter(storage.NewExtensionFileFilter([]string{".csv"}))
	saveDialog.Show()
}

func (mw *MainWindow) onCreateNewFile() {
	if mw.controller == nil {
		return
	}

	// Создаем новый документ
	newDoc := connect.NewDocument("", []model.Manufacturer{})
	mw.controller.AddDocument(newDoc)

	// Создаем вкладку для нового документа
	tab := mw.createTabForDocument(newDoc)
	if tab != nil && mw.tabs != nil {
		mw.tabs.Append(tab)
	}
}

func (mw *MainWindow) onSaveAsWithPrompt() {
	saveDialog := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
		if err != nil {
			mw.runInUI(func() {
				dialog.ShowError(fmt.Errorf("ошибка сохранения: %v", err), mw.window)
			})
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

		mw.runInUI(func() {
			loading := dialog.NewProgress("Сохранение", "Идет сохранение...", mw.window)
			loading.Show()

			go func() {
				err := mw.controller.SaveToFile(filePath)

				mw.runInUI(func() {
					loading.Hide()
					if err != nil {
						dialog.ShowError(fmt.Errorf("не удалось сохранить файл: %v", err), mw.window)
						return
					}
					mw.currentFile = filePath
					mw.unsavedChanges = false
					mw.updateWindowTitle()
					mw.showNotification("Файл успешно сохранён")
					mw.refreshTable()
				})
			}()
		})
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

func (mw *MainWindow) onAdd() {
	newManufacturer := model.Manufacturer{}
	mw.showEditDialog(&newManufacturer, true)
}

func (mw *MainWindow) onEdit(row int) {
	if row == -1 {
		row = mw.selectedRow
	}

	if row <= 0 {
		dialog.ShowInformation(
			"Требуется выбор",
			"Пожалуйста, выберите производителя, щелкнув по строке в таблице",
			mw.window,
		)
		return
	}

	activeDoc := mw.controller.GetActiveDocument()
	if activeDoc == nil {
		return
	}

	var manufacturers []model.Manufacturer
	if activeDoc.IsSearching {
		manufacturers = activeDoc.SearchResults
	} else {
		manufacturers = activeDoc.Manufacturers
	}

	if row-1 >= len(manufacturers) {
		dialog.ShowError(errors.New("неверный выбор строки"), mw.window)
		return
	}

	manufacturer := manufacturers[row-1]
	mw.showEditDialog(&manufacturer, false)
}

func (mw *MainWindow) onDelete(row int) {
	if row == -1 {
		// Если вызвано из меню, используем сохраненную строку
		row = mw.selectedRow
	}

	if row <= 0 {
		dialog.ShowInformation("No Selection", "Please select a manufacturer first", mw.window)
		return
	}

	manufacturers, err := mw.controller.GetAllManufacturers()
	if err != nil || row-1 >= len(manufacturers) {
		dialog.ShowError(errors.New("invalid selection"), mw.window)
		return
	}

	manufacturer := manufacturers[row-1]
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

	// Показываем диалог подтверждения с дополнительной информацией
	confirmText := fmt.Sprintf("%s\n%s: %s\nID: %d",
		mw.locale.Translate("Are you sure you want to delete this manufacturer?"),
		mw.locale.Translate("Name"),
		manufacturer.Name,
		manufacturer.ID)

	dialog.ShowConfirm(
		mw.locale.Translate("Confirm Deletion"),
		confirmText,
		func(confirmed bool) {
			if !confirmed {
				return
			}

			// Показываем индикатор прогресса
			progress := widget.NewProgressBarInfinite()
			content := container.NewVBox(
				widget.NewLabel(mw.locale.Translate("Deleting manufacturer...")),
				progress,
			)
			dlg := dialog.NewCustom(
				mw.locale.Translate("Deleting"),
				mw.locale.Translate("Cancel"),
				content,
				mw.window)
			dlg.Show()

			go func() {
				// Выполняем удаление
				err := mw.controller.DeleteManufacturer(id)

				mw.runInUI(func() {
					dlg.Hide()

					if err != nil {
						dialog.ShowError(fmt.Errorf("%s: %v",
							mw.locale.Translate("Deletion failed"), err), mw.window)
						return
					}

					// Обновляем интерфейс
					mw.unsavedChanges = false
					mw.refreshTable()

					// Показываем уведомление
					dialog.ShowInformation(
						mw.locale.Translate("Success"),
						mw.locale.Translate("Manufacturer deleted successfully"),
						mw.window)
				})
			}()
		},
		mw.window,
	)
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

func (mw *MainWindow) helpMenu() {
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
	activeDoc := mw.controller.GetActiveDocument()
	if activeDoc == nil {
		return
	}

	var manufacturers []model.Manufacturer
	if activeDoc.IsSearching {
		manufacturers = activeDoc.SearchResults
	} else {
		manufacturers = activeDoc.Manufacturers
	}

	newTable := mw.createTableWithData(manufacturers)

	// Получаем индекс выбранной вкладки
	selectedIndex := mw.tabs.CurrentTabIndex()
	if selectedIndex >= 0 && selectedIndex < len(mw.tabs.Items) {
		if content, ok := mw.tabs.Items[selectedIndex].Content.(*fyne.Container); ok {
			if scroll, ok := content.Objects[0].(*container.Scroll); ok {
				scroll.Content = newTable
				scroll.Refresh()
			}
		}
	}
}

func (mw *MainWindow) createTableWithData(manufacturers []model.Manufacturer) *widget.Table {
	table := widget.NewTable(
		func() (int, int) {
			return len(manufacturers) + 1, 10 // +1 для заголовков, 10 столбцов
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("template") // Здесь будет шаблон для ячейки
		},
		func(tci widget.TableCellID, co fyne.CanvasObject) {
			label := co.(*widget.Label)

			if tci.Row == 0 { // Заголовок
				switch tci.Col {
				case 0:
					label.SetText("ID")
				case 1:
					label.SetText("Название")
				case 2:
					label.SetText("Страна")
				case 3:
					label.SetText("Адрес")
				case 4:
					label.SetText("Телефон")
				case 5:
					label.SetText("Email")
				case 6:
					label.SetText("Тип продукта")
				case 7:
					label.SetText("Год основания")
				case 8:
					label.SetText("Выручка")
				case 9:
					label.SetText("Количество сотрудников")
				}
			} else { // Данные производителей
				manufacturer := manufacturers[tci.Row-1] // -1 для пропуска заголовка
				switch tci.Col {
				case 0:
					label.SetText(fmt.Sprintf("%d", manufacturer.ID))
				case 1:
					label.SetText(manufacturer.Name)
				case 2:
					label.SetText(manufacturer.Country)
				case 3:
					label.SetText(manufacturer.Address)
				case 4:
					label.SetText(manufacturer.Phone)
				case 5:
					label.SetText(manufacturer.Email)
				case 6:
					label.SetText(manufacturer.ProductType)
				case 7:
					label.SetText(fmt.Sprintf("%d", manufacturer.FoundedYear))
				case 8:
					label.SetText(fmt.Sprintf("%.2f", manufacturer.Revenue))
				case 9:
					label.SetText(fmt.Sprintf("%d", manufacturer.Employees))
				}
			}
		},
	)

	// Добавляем обработчик события выбора строки
	table.OnSelected = func(id widget.TableCellID) {
		if id.Row > 0 { // Игнорируем заголовки
			mw.selectedRow = id.Row
			// Для отладки можно добавить:
			fmt.Printf("Выбрана строка: %d\n", mw.selectedRow)
		}
	}

	return table
}

func (mw *MainWindow) createManufacturersTable() *widget.Table {
	var manufacturers []model.Manufacturer
	var err error

	// Получаем данные в зависимости от состояния (поиск/обычный режим)
	if mw.isSearching {
		manufacturers = mw.searchResults
	} else if mw.currentDoc != nil {
		manufacturers = mw.currentDoc.Manufacturers
	} else {
		manufacturers, err = mw.controller.GetAllManufacturers()
		if err != nil {
			mw.runOnMainThread(func() {
				dialog.ShowError(err, mw.window)
			})
			return widget.NewTable(nil, nil, nil)
		}
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

			if tci.Row == 0 { // Отрисовка заголовков
				switch tci.Col {
				case 0:
					text := mw.locale.Translate("ID")
					if mw.currentSort.column == "id" {
						text += mw.getSortIcon()
					}
					label.SetText(text)
				case 1:
					text := mw.locale.Translate("Name")
					if mw.currentSort.column == "name" {
						text += mw.getSortIcon()
					}
					label.SetText(text)
				case 2:
					text := mw.locale.Translate("Country")
					if mw.currentSort.column == "country" {
						text += mw.getSortIcon()
					}
					label.SetText(text)
				case 3:
					text := mw.locale.Translate("Address")
					if mw.currentSort.column == "address" {
						text += mw.getSortIcon()
					}
					label.SetText(text)
				case 4:
					text := mw.locale.Translate("Phone")
					if mw.currentSort.column == "phone" {
						text += mw.getSortIcon()
					}
					label.SetText(text)
				case 5:
					text := mw.locale.Translate("Email")
					if mw.currentSort.column == "email" {
						text += mw.getSortIcon()
					}
					label.SetText(text)
				case 6:
					text := mw.locale.Translate("Product Type")
					if mw.currentSort.column == "productType" {
						text += mw.getSortIcon()
					}
					label.SetText(text)
				case 7:
					text := mw.locale.Translate("Revenue")
					if mw.currentSort.column == "revenue" {
						text += mw.getSortIcon()
					}
					label.SetText(text)
				}
				label.TextStyle.Bold = true
				return
			}

			// Отрисовка данных
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
	table.SetColumnWidth(0, 80)  // ID
	table.SetColumnWidth(1, 200) // Name
	table.SetColumnWidth(2, 150) // Country
	table.SetColumnWidth(3, 250) // Address
	table.SetColumnWidth(4, 150) // Phone
	table.SetColumnWidth(5, 200) // Email
	table.SetColumnWidth(6, 200) // Product Type
	table.SetColumnWidth(7, 120) // Revenue

	// Обработчик кликов по заголовкам для сортировки
	table.OnSelected = func(id widget.TableCellID) {
		if id.Row == 0 { // Клик по заголовку
			column := ""
			switch id.Col {
			case 0:
				column = "id"
			case 1:
				column = "name"
			case 2:
				column = "country"
			case 3:
				column = "address"
			case 4:
				column = "phone"
			case 5:
				column = "email"
			case 6:
				column = "productType"
			case 7:
				column = "revenue"
			}

			if column != "" {
				var err error
				ascending := !mw.currentSort.ascending

				if mw.isSearching {
					// Сортируем результаты поиска
					mw.searchResults, err = mw.controller.Sort(mw.searchResults, column, ascending)
				} else if mw.currentDoc != nil {
					// Сортируем данные текущего документа
					mw.currentDoc.Manufacturers, err = mw.controller.Sort(mw.currentDoc.Manufacturers, column, ascending)
				} else {
					// Сортируем все данные
					var allManufacturers []model.Manufacturer
					allManufacturers, err = mw.controller.GetAllManufacturers()
					if err == nil {
						allManufacturers, err = mw.controller.Sort(allManufacturers, column, ascending)
						if err == nil {
							mw.controller.SetManufacturers(allManufacturers)
						}
					}
				}

				if err != nil {
					dialog.ShowError(err, mw.window)
					return
				}

				mw.currentSort.column = column
				mw.currentSort.ascending = ascending
				mw.refreshTable()
			}
		} else {
			// Клик по данным - запоминаем выбранную строку
			mw.selectedRow = id.Row
		}
	}

	return table
}

// Вспомогательная функция для отображения иконки сортировки
func (mw *MainWindow) getSortIcon() string {
	if mw.currentSort.ascending {
		return "^"
	}
	return "!^!"
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

func (mw *MainWindow) runInUI(f func()) {
	// Самый надежный способ выполнить код в UI-потоке в Fyne 1.x
	time.AfterFunc(10*time.Millisecond, func() {
		f()
		mw.window.Content().Refresh()
	})
}

func (mw *MainWindow) runOnMainThread(f func()) {
	log.Println("Запланировано выполнение в главном потоке")
	time.AfterFunc(10*time.Millisecond, func() {
		log.Println("Выполнение в главном потоке")
		f()
		mw.window.Content().Refresh()
	})
}

func (mw *MainWindow) onExportPDF() {
	saveDialog := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
		if err != nil {
			dialog.ShowError(err, mw.window)
			return
		}
		if writer == nil {
			return
		}
		defer writer.Close()

		filePath := uriToPath(writer.URI())
		if !strings.HasSuffix(strings.ToLower(filePath), ".pdf") {
			filePath += ".pdf"
		}

		if err := mw.controller.ExportToPDF(filePath); err != nil {
			dialog.ShowError(fmt.Errorf("export failed: %v", err), mw.window)
			return
		}

		mw.showNotification("PDF exported successfully")
	}, mw.window)

	saveDialog.SetFilter(storage.NewExtensionFileFilter([]string{".pdf"}))
	saveDialog.Show()
}

func (mw *MainWindow) onPrint() {
	progress := dialog.NewProgress("Подготовка к печати", "Формирование документа...", mw.window)
	progress.Show()

	go func() {
		defer progress.Hide()

		pdfData, err := mw.controller.GetPrintableData()
		if err != nil {
			mw.runInUI(func() {
				dialog.ShowError(err, mw.window)
			})
			return
		}

		tmpFile, err := os.CreateTemp("", "print_*.pdf")
		if err != nil {
			mw.runInUI(func() {
				dialog.ShowError(err, mw.window)
			})
			return
		}
		defer os.Remove(tmpFile.Name())

		if _, err := tmpFile.Write(pdfData); err != nil {
			mw.runInUI(func() {
				dialog.ShowError(err, mw.window)
			})
			return
		}
		tmpFile.Close()

		mw.runInUI(func() {
			// Предложим выбор: печать или просмотр
			dialog.ShowCustomConfirm(
				"Печать документа",
				"Печать",
				"Просмотр",
				container.NewVBox(
					widget.NewLabel("Выберите действие с документом:"),
					widget.NewLabel(tmpFile.Name()),
				),
				func(print bool) {
					if print {
						mw.printPDF(tmpFile.Name())
					} else {
						mw.openPDF(tmpFile.Name())
					}
				},
				mw.window,
			)
		})
	}()
}

func (mw *MainWindow) printPDF(filename string) {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "mshtml.dll", "PrintHTML", "file:"+filename)
	case "darwin":
		cmd = exec.Command("lp", filename)
	default: // linux и другие unix-системы
		cmd = exec.Command("lp", filename)
	}

	if err := cmd.Run(); err != nil {
		dialog.ShowError(fmt.Errorf("ошибка печати: %v", err), mw.window)
		return
	}

	dialog.ShowInformation(
		"Успешно",
		"Документ отправлен на печать",
		mw.window,
	)
}

func (mw *MainWindow) openPDF(filename string) {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", filename)
	case "darwin":
		cmd = exec.Command("open", filename)
	default:
		cmd = exec.Command("xdg-open", filename)
	}

	if err := cmd.Run(); err != nil {
		dialog.ShowError(fmt.Errorf("не удалось открыть PDF: %v", err), mw.window)
	}
}

func (mw *MainWindow) onShowChart() {
	// Диалог выбора колонки для графика
	columnSelect := widget.NewSelect([]string{"revenue", "foundedYear"}, nil)
	columnSelect.SetSelected("revenue")

	dialog.ShowCustomConfirm(
		"Generate Chart",
		"Generate",
		"Cancel",
		container.NewVBox(
			widget.NewLabel("Select data column:"),
			columnSelect,
		),
		func(confirm bool) {
			if !confirm {
				return
			}

			loading := dialog.NewProgress("Generating Chart", "Please wait...", mw.window)
			loading.Show()

			go func() {
				imgData, err := mw.controller.GenerateChart(columnSelect.Selected)

				mw.runInUI(func() {
					loading.Hide()
					if err != nil {
						dialog.ShowError(err, mw.window)
						return
					}

					// Показываем изображение в новом окне
					img := canvas.NewImageFromReader(bytes.NewReader(imgData), "chart.png")
					img.FillMode = canvas.ImageFillOriginal

					chartWindow := mw.app.NewWindow("Manufacturers Chart")
					chartWindow.SetContent(container.NewScroll(&widget.Icon{}))
					chartWindow.Resize(fyne.NewSize(800, 600))
					chartWindow.Show()
				})
			}()
		},
		mw.window,
	)
}

func (mw *MainWindow) updateTabTitle(doc *connect.Document) {
	for _, tab := range mw.tabs.Items {
		if mw.getTabTitle(doc) == tab.Text {
			tab.Text = mw.getTabTitle(doc)
			tab.Content.Refresh()
			break
		}
	}
}

func (mw *MainWindow) setupSearch() *fyne.Container {
	searchEntry := widget.NewEntry()
	searchEntry.SetPlaceHolder(mw.locale.Translate("Search..."))

	// При изменении вкладки обновляем поле поиска
	mw.tabs.OnChanged = func(tab *container.TabItem) {
		if doc := mw.controller.GetActiveDocument(); doc != nil {
			searchEntry.SetText(doc.SearchQuery)
		}
	}

	searchEntry.OnChanged = func(query string) {
		activeDoc := mw.controller.GetActiveDocument()
		if activeDoc == nil {
			return
		}

		// Сохраняем запрос в документ
		activeDoc.SearchQuery = query

		if query == "" {
			activeDoc.IsSearching = false
			mw.refreshTable()
			return
		}

		// Ищем в данных текущего документа
		results, err := mw.controller.SearchInManufacturers(activeDoc.Manufacturers, query)
		if err != nil {
			dialog.ShowError(err, mw.window)
			return
		}

		activeDoc.SearchResults = results
		activeDoc.IsSearching = true
		mw.refreshTable()
	}

	clearBtn := widget.NewButtonWithIcon("", theme.ContentClearIcon(), func() {
		searchEntry.SetText("")
	})
	clearBtn.Importance = widget.LowImportance

	return container.NewBorder(
		nil,
		nil,
		nil,
		clearBtn,
		searchEntry,
	)
}
