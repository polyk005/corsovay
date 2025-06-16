package view

import (
	"bytes"
	"cursovay/internal/controller"
	"cursovay/internal/model"
	"cursovay/pkg/localization"
	"errors"
	"fmt"
	"image"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne"
	"fyne.io/fyne/canvas"
	"fyne.io/fyne/container"
	"fyne.io/fyne/dialog"
	"fyne.io/fyne/storage"
	"fyne.io/fyne/widget"
	"fyne.io/fyne/theme"
)

type MainWindow struct {
	app           fyne.App
	window        fyne.Window
	table         *widget.Table
	chartWindow   fyne.Window
	searchEntry   *widget.Entry
	searchWindow  fyne.Window // Новое поле для окна поиска
	searchResults []model.Manufacturer
	isSearching   bool
	recentFiles   *RecentFiles
	currentSort   struct {
		column    string
		ascending bool
	}
	mainContainer *fyne.Container
	// tableContainer *fyne.Container
	controller     *controller.ManufacturerController
	locale         *localization.Locale
	currentFile    string
	unsavedChanges bool
	selectedRow    int
}

type RecentFiles struct {
	files []string
	max   int
}

func NewRecentFiles(max int) *RecentFiles {
	return &RecentFiles{
		max: max,
	}
}

func NewMainWindow(app fyne.App, controller *controller.ManufacturerController, locale *localization.Locale) *MainWindow {
	if controller == nil {
		log.Fatal("Контроллер не инициализирован")
	}

	w := app.NewWindow(locale.Translate("База данных производителей"))
	w.Resize(fyne.NewSize(1400, 800)) // Начальный размер окна

	// Инициализация пустой базы данных
	controller.NewDatabase()

	mw := &MainWindow{
		app:            app,
		window:         w,
		controller:     controller,
		locale:         locale,
		currentFile:    "",
		unsavedChanges: false,
		recentFiles:    NewRecentFiles(10),
	}

	// Загружаем недавние файлы
	mw.loadRecentFiles()

	return mw
}

func (mw *MainWindow) setupMenu() *fyne.MainMenu {
	fileMenu := fyne.NewMenu(mw.locale.Translate("File"),
		fyne.NewMenuItem(mw.locale.Translate("New"), mw.onCreateNewFile),
		fyne.NewMenuItem(mw.locale.Translate("Open"), mw.onOpen),
		fyne.NewMenuItem(mw.locale.Translate("Save"), mw.onSave),
		fyne.NewMenuItem(mw.locale.Translate("Save As"), mw.onSaveAsWithPrompt),
		fyne.NewMenuItem(mw.locale.Translate("Export to PDF"), mw.onExportPDF),
		fyne.NewMenuItem(mw.locale.Translate("Export to JSON"), mw.onExportJSON),
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

func (mw *MainWindow) createManufacturersTab() *container.TabItem {
	// Создание таблицы производителей
	table := mw.createManufacturersTable()

	return container.NewTabItem(
		mw.locale.Translate("Manufacturers"),
		container.NewScroll(table), // Обернем таблицу в прокручиваемый контейнер
	)
}

func (mw *MainWindow) createRecentFilesTab() *widget.TabItem {
	list := widget.NewList(
		func() int {
			return len(mw.recentFiles.Get())
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("template")
		},
		func(id widget.ListItemID, item fyne.CanvasObject) {
			item.(*widget.Label).SetText(filepath.Base(mw.recentFiles.Get()[id]))
		},
	)

	list.OnSelected = func(id widget.ListItemID) {
		filePath := mw.recentFiles.Get()[id]
		mw.checkUnsavedChanges(func() {
			// Показываем индикатор загрузки
			loading := dialog.NewProgress("Загрузка", "Открытие файла...", mw.window)
			loading.Show()

			// В Fyne v1 используем time.AfterFunc для обновления UI
			go func() {
				// Загрузка в фоновом потоке
				_, err := mw.controller.LoadFromFile(filePath)

				// Обновляем UI через time.AfterFunc
				time.AfterFunc(100*time.Millisecond, func() {
					loading.Hide()
					if err != nil {
						dialog.ShowError(err, mw.window)
						return
					}

					mw.currentFile = filePath
					mw.unsavedChanges = false

					// Полностью пересоздаем таблицу
					mw.table = mw.createManufacturersTable()

					// Обновляем содержимое главного окна
					mw.refreshMainContent()

					mw.window.SetTitle("База производителей - " + filepath.Base(filePath))
					mw.window.Content().Refresh()
				})
			}()
		})
	}

	return widget.NewTabItem(
		mw.locale.Translate("Recent Files"),
		container.NewBorder(
			widget.NewLabel(mw.locale.Translate("Select a recent file:")),
			nil, nil, nil,
			list,
		),
	)
}

// Добавляем новый метод для обновления основного содержимого
func (mw *MainWindow) refreshMainContent() {
	searchBox := container.NewVBox(
		mw.setupSearch(),
		widget.NewSeparator(),
	)

	// Создаем новое содержимое
	newContent := container.NewBorder(
		searchBox, nil, nil, nil,
		container.NewScroll(mw.table),
	)

	// В Fyne v1 используем widget.TabContainer
	if tabs, ok := mw.mainContainer.Objects[0].(*widget.TabContainer); ok {
		if len(tabs.Items) > 0 {
			tabs.Items[0].Content = newContent
			tabs.SelectTab(tabs.Items[0]) // Выбираем первую вкладку
			tabs.Refresh()
		}
	}
}

func (mw *MainWindow) Show() {
	mw.window.SetMainMenu(mw.setupMenu())
	mw.table = mw.createManufacturersTable()

	// Создаем поисковую панель
	searchBox := container.NewVBox(
		mw.setupSearch(),
		widget.NewSeparator(),
	)

	// Основное содержимое вкладки Database
	databaseContent := container.NewBorder(
		searchBox, nil, nil, nil,
		container.NewScroll(mw.table),
	)

	// В Fyne v1 используем widget.NewTabContainer
	tabs := widget.NewTabContainer(
		widget.NewTabItem(mw.locale.Translate("Database"), databaseContent),
		mw.createRecentFilesTab(),
	)

	mw.mainContainer = container.NewMax(tabs)
	mw.window.SetContent(mw.mainContainer)
	mw.window.Resize(fyne.NewSize(1400, 800))
	mw.window.ShowAndRun()
}

func (mw *MainWindow) loadRecentFiles() {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return
	}

	appDir := filepath.Join(configDir, "ManufacturersDB")
	os.MkdirAll(appDir, 0755)

	filePath := filepath.Join(appDir, "recent_files.txt")
	data, err := os.ReadFile(filePath)
	if err != nil {
		return
	}

	files := strings.Split(string(data), "\n")
	for _, file := range files {
		if file != "" {
			mw.recentFiles.Add(file)
		}
	}
}

func (mw *MainWindow) saveRecentFiles() {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return
	}

	appDir := filepath.Join(configDir, "ManufacturersDB")
	os.MkdirAll(appDir, 0755)

	filePath := filepath.Join(appDir, "recent_files.txt")
	data := strings.Join(mw.recentFiles.Get(), "\n")
	os.WriteFile(filePath, []byte(data), 0644)
}

// func (mw *MainWindow) onNew() {
// 	mw.checkUnsavedChanges(func() {
// 		// Создаем новую пустую базу
// 		mw.controller.NewDatabase()
// 		mw.currentFile = ""
// 		mw.unsavedChanges = false
// 		mw.refreshTable()
// 		mw.updateWindowTitle()
// 		mw.showNotification("Создана новая база данных")
// 	})
// }

func (mw *MainWindow) onOpen() {
	fileDialog := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil {
			dialog.ShowError(err, mw.window)
			return
		}
		if reader == nil {
			return // Пользователь отменил
		}
		defer reader.Close()

		filePath := uriToPath(reader.URI())
		mw.recentFiles.Add(filePath)
		if !strings.HasSuffix(strings.ToLower(filePath), ".csv") {
			dialog.ShowError(errors.New("выберите CSV файл"), mw.window)
			return
		}

		// Сбрасываем текущее состояние
		mw.controller.NewDatabase()

		// Изменено здесь - игнорируем первый возвращаемый параметр
		if _, err := mw.controller.LoadFromFile(filePath); err != nil {
			dialog.ShowError(err, mw.window)
			return
		}

		mw.currentFile = filePath
		mw.unsavedChanges = false
		mw.refreshTable()
		mw.refreshSearchStyle() // Обновляем стиль поиска после загрузки файла
		mw.window.SetTitle("База производителей - " + filepath.Base(filePath))
	}, mw.window)

	fileDialog.SetFilter(storage.NewExtensionFileFilter([]string{".csv"}))
	fileDialog.Show()
}

func (mw *MainWindow) onSave() {
	if mw.currentFile == "" {
		mw.onSaveAsWithPrompt()
		return
	}

	loading := dialog.NewProgress("Сохранение", "Идет сохранение...", mw.window)
	loading.Show()

	go func() {
		err := mw.controller.SaveToFile(mw.currentFile)

		mw.runInUI(func() {
			loading.Hide()
			if err != nil {
				dialog.ShowError(fmt.Errorf("Ошибка сохранения: %v", err), mw.window)
				return
			}
			mw.unsavedChanges = false
			mw.showNotification("Файл успешно сохранен")
			mw.refreshTable() // Обновляем таблицу после сохранения
		})
	}()
}

func (mw *MainWindow) checkLoadedData() {
	manufacturers, err := mw.controller.GetAllManufacturers()
	if err != nil {
		dialog.ShowError(err, mw.window)
		return
	}

	fmt.Println("Loaded manufacturers:")
	for _, m := range manufacturers {
		fmt.Printf("%+v\n", m)
	}
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

	// Получаем список существующих типов продукции
	productTypes := mw.controller.GetUniqueProductTypes()
	
	// Создаем элементы формы
	formItems := []*widget.FormItem{
		{Text: mw.locale.Translate("Name"), Widget: nameEntry},
		{Text: mw.locale.Translate("Country"), Widget: countryEntry},
		{Text: mw.locale.Translate("Address"), Widget: addressEntry},
		{Text: mw.locale.Translate("Phone"), Widget: phoneEntry},
		{Text: mw.locale.Translate("Email"), Widget: emailEntry},
	}

	// Создаем виджеты для типа продукции
	productTypeEntry := widget.NewEntry()
	productTypeEntry.SetText(manufacturer.ProductType)
	
	var productTypeSelect *widget.Select
	var productTypeContainer *fyne.Container
	var useExistingType bool

	if len(productTypes) > 0 {
		// Создаем выпадающий список с существующими типами
		productTypeSelect = widget.NewSelect(productTypes, nil)
		if manufacturer.ProductType != "" && contains(productTypes, manufacturer.ProductType) {
			productTypeSelect.SetSelected(manufacturer.ProductType)
			useExistingType = true
		}

		// Создаем радио-кнопки для выбора режима
		radioGroup := widget.NewRadioGroup(
			[]string{
				mw.locale.Translate("Use Existing Type"),
				mw.locale.Translate("Add New Type"),
			},
			func(selected string) {
				useExistingType = selected == mw.locale.Translate("Use Existing Type")
				if useExistingType {
					productTypeContainer.Objects[0] = productTypeSelect
				} else {
					productTypeContainer.Objects[0] = productTypeEntry
				}
				productTypeContainer.Refresh()
			},
		)

		// Добавляем радио-группу в форму
		formItems = append(formItems, &widget.FormItem{
			Text:   mw.locale.Translate("Product Type Selection"),
			Widget: radioGroup,
		})

		// Инициализируем контейнер с правильным виджетом
		if useExistingType {
			productTypeContainer = container.NewHBox(productTypeSelect)
			radioGroup.SetSelected(mw.locale.Translate("Use Existing Type"))
		} else {
			productTypeContainer = container.NewHBox(productTypeEntry)
			radioGroup.SetSelected(mw.locale.Translate("Add New Type"))
		}
	} else {
		// Если нет существующих типов, показываем только поле ввода
		productTypeContainer = container.NewHBox(productTypeEntry)
	}

	foundedYearEntry := widget.NewEntry()
	foundedYearEntry.SetText(fmt.Sprintf("%d", manufacturer.FoundedYear))

	revenueEntry := widget.NewEntry()
	revenueEntry.SetText(fmt.Sprintf("%.2f", manufacturer.Revenue))

	// Добавляем оставшиеся поля формы
	formItems = append(formItems,
		&widget.FormItem{Text: mw.locale.Translate("Product Type"), Widget: productTypeContainer},
		&widget.FormItem{Text: mw.locale.Translate("Founded Year"), Widget: foundedYearEntry},
		&widget.FormItem{Text: mw.locale.Translate("Revenue"), Widget: revenueEntry},
	)

	form := &widget.Form{
		Items: formItems,
		OnSubmit: func() {
			// Валидация данных
			if nameEntry.Text == "" {
				dialog.ShowError(errors.New(mw.locale.Translate("Name cannot be empty")), mw.window)
				return
			}

			year, yearErr := strconv.Atoi(foundedYearEntry.Text)
			if yearErr != nil {
				dialog.ShowError(errors.New(mw.locale.Translate("Invalid year format")), mw.window)
				return
			}

			revenue, revenueErr := strconv.ParseFloat(revenueEntry.Text, 64)
			if revenueErr != nil {
				dialog.ShowError(errors.New(mw.locale.Translate("Invalid revenue format")), mw.window)
				return
			}

			// Получаем значение типа продукции
			var productType string
			if len(productTypes) > 0 && useExistingType && productTypeSelect.Selected != "" {
				productType = productTypeSelect.Selected
			} else {
				productType = productTypeEntry.Text
			}

			if productType == "" {
				dialog.ShowError(errors.New(mw.locale.Translate("Product type cannot be empty")), mw.window)
				return
			}

			// Обновляем данные производителя
			manufacturer.Name = nameEntry.Text
			manufacturer.Country = countryEntry.Text
			manufacturer.Address = addressEntry.Text
			manufacturer.Phone = phoneEntry.Text
			manufacturer.Email = emailEntry.Text
			manufacturer.ProductType = productType
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

// Вспомогательная функция для проверки наличия строки в слайсе
func contains(slice []string, str string) bool {
	for _, v := range slice {
		if v == str {
			return true
		}
	}
	return false
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
	newTable := mw.createManufacturersTable()

	time.AfterFunc(5*time.Millisecond, func() {
		if tabs, ok := mw.mainContainer.Objects[0].(*widget.TabContainer); ok {
			if len(tabs.Items) > 0 {
				// Обновляем содержимое вкладки Database
				if scroll, ok := tabs.Items[0].Content.(*container.Scroll); ok {
					scroll.Content = newTable
					mw.table = newTable
					scroll.Refresh()
				}
				tabs.Refresh()
			}
		}
		
		// Принудительно обновляем поле поиска и его контейнер
		if mw.searchEntry != nil {
			mw.window.Canvas().Refresh(mw.searchEntry)
		}
		
		mw.window.Content().Refresh()
	})
	
	// Важно для работы сортировки
	mw.table = newTable
	mw.refreshMainContent()
}

func (mw *MainWindow) createManufacturersTable() *widget.Table {
	var manufacturers []model.Manufacturer
	if mw.isSearching {
		manufacturers = mw.searchResults
	} else {
		manufacturers = mw.controller.GetCurrentData()
	}

	table := widget.NewTable(
		func() (int, int) {
			return len(manufacturers) + 1, 9
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("template")
		},
		func(tci widget.TableCellID, co fyne.CanvasObject) {
			label := co.(*widget.Label)
			if tci.Row == 0 {
				// Заполняем заголовки
				headers := []string{"Id", "Name", "Country", "Address", "Phone",
					"Email", "Product Type", "Founded Year", "Revenue"}
				if tci.Col < len(headers) {
					label.SetText(headers[tci.Col])
				}
			} else if tci.Row-1 < len(manufacturers) {
				// Заполняем данные
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
					label.SetText(fmt.Sprintf("%d", m.FoundedYear))
				case 8:
					label.SetText(fmt.Sprintf("%.2f", m.Revenue))
				}
			}
		},
	)

	// Настраиваем размеры столбцов
	table.SetColumnWidth(0, 60)   // ID
	table.SetColumnWidth(1, 180)  // Name
	table.SetColumnWidth(2, 150)  // Country
	table.SetColumnWidth(3, 200)  // Address
	table.SetColumnWidth(4, 120)  // Phone
	table.SetColumnWidth(5, 180)  // Email
	table.SetColumnWidth(6, 150)  // Product Type
	table.SetColumnWidth(7, 120)  // Founded Year
	table.SetColumnWidth(8, 150)  // Revenue

	table.OnSelected = func(id widget.TableCellID) {
		if id.Row == 0 { // Сортировка по заголовку
			columns := []string{"id", "name", "country", "address", "phone",
				"email", "productType", "foundedYear", "revenue"}

			if id.Col < len(columns) {
				ascending := !mw.currentSort.ascending
				var dataToSort []model.Manufacturer

				if mw.isSearching {
					dataToSort = mw.searchResults
				} else {
					dataToSort = mw.controller.GetCurrentData()
				}

				sorted, err := mw.controller.Sort(dataToSort, columns[id.Col], ascending)
				if err != nil {
					dialog.ShowError(err, mw.window)
					return
				}

				if mw.isSearching {
					mw.searchResults = sorted
				} else {
					// Для основного набора нужно обновить данные в контроллере
					mw.controller.UpdateManufacturers(sorted)
				}

				mw.currentSort.column = columns[id.Col]
				mw.currentSort.ascending = ascending
				mw.refreshTable()
			}
		} else {
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

// func (mw *MainWindow) updateUI(f func()) {
// 	// Используем AfterFunc как самый надежный способ в Fyne 1.x
// 	time.AfterFunc(10*time.Millisecond, func() {
// 		f()
// 	})
// }

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

func (mw *MainWindow) onExportJSON() {
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
		if !strings.HasSuffix(strings.ToLower(filePath), ".json") {
			filePath += ".json"
		}

		progress := dialog.NewProgress(mw.locale.Translate("Export"), mw.locale.Translate("Exporting to JSON..."), mw.window)
		progress.Show()

		go func() {
			defer progress.Hide()

			if err := mw.controller.ExportToJSON(filePath); err != nil {
				mw.runInUI(func() {
					dialog.ShowError(fmt.Errorf("export failed: %v", err), mw.window)
				})
				return
			}

			mw.runInUI(func() {
				mw.showNotification(mw.locale.Translate("JSON exported successfully"))
			})
		}()
	}, mw.window)

	saveDialog.SetFilter(storage.NewExtensionFileFilter([]string{".json"}))
	saveDialog.Show()
}

func (mw *MainWindow) onShowChart() {
	// Создаем селектор типа графика
	chartTypeSelect := widget.NewSelect([]string{
		mw.locale.Translate("Bar Chart - Revenue"),
		mw.locale.Translate("Bar Chart - Founded Year"),
		mw.locale.Translate("Pie Chart - Product Types"),
		mw.locale.Translate("Line Chart - Revenue Trend"),
	}, nil)
	chartTypeSelect.SetSelected(mw.locale.Translate("Bar Chart - Revenue"))

	// Создаем селектор цветовой схемы
	colorSchemeSelect := widget.NewSelect([]string{
		mw.locale.Translate("Default"),
		mw.locale.Translate("Blue Theme"),
		mw.locale.Translate("Green Theme"),
		mw.locale.Translate("Rainbow"),
	}, nil)
	colorSchemeSelect.SetSelected(mw.locale.Translate("Default"))

	// Создаем чекбокс для отображения значений
	showValuesCheck := widget.NewCheck(mw.locale.Translate("Show Values"), nil)
	showValuesCheck.SetChecked(true)

	// Создаем чекбокс для сортировки данных
	sortDataCheck := widget.NewCheck(mw.locale.Translate("Sort Data"), nil)
	sortDataCheck.SetChecked(true)

	// Кнопка генерации графика
	generateBtn := widget.NewButton(mw.locale.Translate("Generate Chart"), func() {
		if chartTypeSelect.Selected == "" {
			dialog.ShowInformation(
				mw.locale.Translate("Error"),
				mw.locale.Translate("Please select chart type"),
				mw.window,
			)
			return
		}

		// Определяем тип данных для графика
		var chartType string
		switch chartTypeSelect.Selected {
		case mw.locale.Translate("Bar Chart - Revenue"):
			chartType = "revenue_bar"
		case mw.locale.Translate("Bar Chart - Founded Year"):
			chartType = "founded_bar"
		case mw.locale.Translate("Pie Chart - Product Types"):
			chartType = "product_pie"
		case mw.locale.Translate("Line Chart - Revenue Trend"):
			chartType = "revenue_line"
		}

		// Создаем параметры для графика
		chartParams := map[string]interface{}{
			"type":        chartType,
			"colorScheme": colorSchemeSelect.Selected,
			"showValues":  showValuesCheck.Checked,
			"sortData":    sortDataCheck.Checked,
		}

		// Генерируем график
		imgData, err := mw.controller.GenerateChart(chartParams)
		if err != nil {
			dialog.ShowError(err, mw.window)
			return
		}

		// Конвертируем PNG данные в изображение
		img, _, err := image.Decode(bytes.NewReader(imgData))
		if err != nil {
			dialog.ShowError(fmt.Errorf("ошибка декодирования изображения: %v", err), mw.window)
			return
		}

		// Создаем новое окно для графика
		chartWindow := mw.app.NewWindow(mw.locale.Translate("Chart View"))
		
		// Создаем и настраиваем изображение
		chartImg := canvas.NewImageFromImage(img)
		chartImg.FillMode = canvas.ImageFillContain
		chartImg.SetMinSize(fyne.NewSize(900, 600))

		// Создаем скролл-контейнер и устанавливаем его размер
		scrollContainer := container.NewScroll(chartImg)
		scrollContainer.Resize(fyne.NewSize(20000, 20000))

		// Создаем кнопку сохранения
		saveButton := widget.NewButton(mw.locale.Translate("Save Chart"), func() {
			saveDialog := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
				if err != nil {
					dialog.ShowError(err, chartWindow)
					return
				}
				if writer == nil {
					return
				}
				defer writer.Close()

				filePath := uriToPath(writer.URI())
				if !strings.HasSuffix(strings.ToLower(filePath), ".png") {
					filePath += ".png"
				}

				if err := os.WriteFile(filePath, imgData, 0644); err != nil {
					dialog.ShowError(err, chartWindow)
					return
				}

				dialog.ShowInformation(
					mw.locale.Translate("Success"),
					mw.locale.Translate("Chart saved successfully"),
					chartWindow,
				)
			}, chartWindow)

			saveDialog.SetFilter(storage.NewExtensionFileFilter([]string{".png"}))
			saveDialog.Show()
		})

		// Создаем контейнер для графика и кнопки
		content := container.NewVBox(
			scrollContainer,
			saveButton,
		)
		content.Resize(fyne.NewSize(900, 650)) // Дополнительная высота для кнопки

		chartWindow.SetContent(content)
		chartWindow.Resize(fyne.NewSize(920, 700)) // Немного больше для рамок окна
		chartWindow.Show()
		chartWindow.CenterOnScreen()
	})

	// Создаем контейнер с элементами управления
	controls := container.NewVBox(
		widget.NewLabel(mw.locale.Translate("Chart Settings")),
		container.NewGridWithColumns(2,
			widget.NewLabel(mw.locale.Translate("Chart Type:")),
			chartTypeSelect,
			widget.NewLabel(mw.locale.Translate("Color Scheme:")),
			colorSchemeSelect,
		),
		container.NewHBox(
			showValuesCheck,
			sortDataCheck,
		),
		generateBtn,
	)

	// Создаем окно для настроек графика (если еще не создано)
	if mw.chartWindow == nil {
		mw.chartWindow = mw.app.NewWindow(mw.locale.Translate("Chart Settings"))
		mw.chartWindow.Resize(fyne.NewSize(400, 300))
		mw.chartWindow.SetContent(controls)
		mw.chartWindow.SetOnClosed(func() {
			mw.chartWindow = nil
		})
	}
	
	mw.chartWindow.Show()
	mw.chartWindow.CenterOnScreen()
}

func (mw *MainWindow) setupSearch() fyne.CanvasObject {
	// Создаем кнопку поиска
	searchButton := widget.NewButtonWithIcon(mw.locale.Translate("Search"), theme.SearchIcon(), func() {
		mw.showSearchWindow()
	})
	searchButton.Resize(fyne.NewSize(120, 40))

	return container.NewPadded(searchButton)
}

func (mw *MainWindow) showSearchWindow() {
	// Если окно уже существует, показываем его
	if mw.searchWindow != nil {
		mw.searchWindow.Show()
		return
	}

	// Создаем новое окно для поиска
	mw.searchWindow = mw.app.NewWindow(mw.locale.Translate("Search Manufacturers"))
	mw.searchWindow.Resize(fyne.NewSize(400, 500))

	// Создаем поле поиска
	searchEntry := widget.NewEntry()
	searchEntry.SetPlaceHolder(mw.locale.Translate("Enter search text..."))
	mw.searchEntry = searchEntry

	// Создаем метку для отображения текущего поискового запроса
	searchLabel := widget.NewLabel("")
	
	// Создаем список для отображения результатов
	resultsList := widget.NewTextGrid()

	// Обработчик изменения текста
	searchEntry.OnChanged = func(query string) {
		// Обновляем метку с текущим поисковым запросом
		searchLabel.SetText(mw.locale.Translate("Current search: ") + query)
		searchLabel.Refresh()

		if query == "" {
			mw.isSearching = false
			mw.searchResults = nil
			resultsList.SetText("")
			mw.refreshTable()
			return
		}

		// Получаем текущие данные
		currentData := mw.controller.GetCurrentData()
		if len(currentData) == 0 {
			return
		}

		// Выполняем поиск
		var results []model.Manufacturer
		query = strings.ToLower(query)
		for _, m := range currentData {
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

		// Формируем текст для отображения результатов
		var resultsText strings.Builder
		for _, m := range results {
			resultsText.WriteString(fmt.Sprintf("Name: %s\nCountry: %s\nProduct: %s\n\n",
				m.Name, m.Country, m.ProductType))
		}
		
		// Обновляем список результатов
		resultsList.SetText(resultsText.String())
		resultsList.Refresh()

		mw.searchResults = results
		mw.isSearching = true
		mw.refreshTable()
	}

	// Создаем кнопку закрытия
	closeButton := widget.NewButton(mw.locale.Translate("Close"), func() {
		mw.searchWindow.Hide()
	})

	// Создаем кнопку очистки
	clearButton := widget.NewButton(mw.locale.Translate("Clear"), func() {
		searchEntry.SetText("")
		searchLabel.SetText("")
		resultsList.SetText("")
		mw.isSearching = false
		mw.searchResults = nil
		mw.refreshTable()
	})

	// Создаем контейнер с элементами
	content := container.NewVBox(
		searchEntry,
		searchLabel,
		container.NewHBox(clearButton, closeButton),
		widget.NewSeparator(),
		container.NewScroll(resultsList),
	)

	mw.searchWindow.SetContent(content)
	mw.searchWindow.Show()

	// Устанавливаем обработчик закрытия окна
	mw.searchWindow.SetOnClosed(func() {
		mw.searchWindow = nil
	})
}

// Обновляем метод refreshSearchStyle без использования TextStyle
func (mw *MainWindow) refreshSearchStyle() {
	if mw.searchEntry != nil {
		mw.searchEntry.Refresh()
	}
}

func getPrintCommand(filename string) (*exec.Cmd, error) {
	switch runtime.GOOS {
	case "windows":
		// Попробуем разные варианты для Windows
		if path, err := exec.LookPath("AcroRd32.exe"); err == nil {
			return exec.Command(path, "/t", filename), nil
		}
		if path, err := exec.LookPath("SumatraPDF.exe"); err == nil {
			return exec.Command(path, "-print-to-default", filename), nil
		}
		return nil, errors.New("не найдена программа для печати PDF (установите Adobe Reader или SumatraPDF)")
	case "darwin":
		if path, err := exec.LookPath("lp"); err == nil {
			return exec.Command(path, filename), nil
		}
		return nil, errors.New("команда 'lp' не найдена")
	default: // Linux и другие UNIX
		if path, err := exec.LookPath("lp"); err == nil {
			return exec.Command(path, filename), nil
		}
		if path, err := exec.LookPath("evince"); err == nil {
			return exec.Command(path, "-p", filename), nil
		}
		return nil, errors.New("не найдены команды 'lp' или 'evince'")
	}
}

func (rf *RecentFiles) Add(file string) {
	// Удаляем дубликаты
	for i, f := range rf.files {
		if f == file {
			rf.files = append(rf.files[:i], rf.files[i+1:]...)
			break
		}
	}

	// Добавляем в начало
	rf.files = append([]string{file}, rf.files...)

	// Обрезаем если превышен лимит
	if len(rf.files) > rf.max {
		rf.files = rf.files[:rf.max]
	}
}

func (rf *RecentFiles) Get() []string {
	return rf.files
}
