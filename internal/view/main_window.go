package view

import (
	"bytes"
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
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/widget"
)

type MainWindow struct {
	app         fyne.App
	window      fyne.Window
	table       *widget.Table
	searchEntry *widget.Entry
	currentSort struct {
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

	aboutMenu := fyne.NewMenu("Справка",
		fyne.NewMenuItem("О программе", mw.Show))

	return fyne.NewMainMenu(
		fileMenu,
		editMenu,
		viewMenu,
		langMenu,
		aboutMenu,
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
	// Устанавливаем меню
	mw.window.SetMainMenu(mw.setupMenu())

	// Инициализируем таблицу
	mw.table = mw.createManufacturersTable()

	// Создаем поисковую панель
	searchBox := container.NewVBox(
		mw.setupSearch(),
		widget.NewSeparator(),
	)

	// Собираем основной интерфейс
	mw.mainContainer = container.NewBorder(
		searchBox,                     // Верх - панель поиска
		nil,                           // Низ (можно добавить статус бар)
		nil,                           // Левая панель
		nil,                           // Правая панель
		container.NewScroll(mw.table), // Центр - таблица с прокруткой
	)

	// Настраиваем окно
	mw.window.SetContent(mw.mainContainer)
	mw.window.Resize(fyne.NewSize(1000, 600))

	// Показываем окно (НЕ используем ShowAndRun!)
	mw.window.ShowAndRun()
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
		if !strings.HasSuffix(strings.ToLower(filePath), ".csv") {
			dialog.ShowError(errors.New("выберите CSV файл"), mw.window)
			return
		}

		// Сбрасываем текущее состояние
		mw.controller.NewDatabase()

		if err := mw.controller.LoadFromFile(filePath); err != nil {
			dialog.ShowError(err, mw.window)
			return
		}

		mw.currentFile = filePath
		mw.unsavedChanges = false
		mw.refreshTable()
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

// func (mw *MainWindow) onSaveAs() {
// 	saveDialog := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
// 		if err != nil {
// 			dialog.ShowError(err, mw.window)
// 			return
// 		}
// 		if writer == nil {
// 			return // Пользователь отменил
// 		}
// 		defer writer.Close()

// 		filePath := uriToPath(writer.URI())
// 		if !strings.HasSuffix(strings.ToLower(filePath), ".csv") {
// 			filePath += ".csv"
// 		}

// 		if err := mw.controller.SaveToFile(filePath); err != nil {
// 			dialog.ShowError(err, mw.window)
// 			return
// 		}

// 		mw.currentFile = filePath
// 		mw.unsavedChanges = false
// 		mw.window.SetTitle(mw.locale.Translate("Manufacturers Database") + " - " + filepath.Base(filePath))
// 		mw.showNotification(mw.locale.Translate("File saved successfully"))
// 	}, mw.window)

// 	// Устанавливаем фильтр для CSV файлов
// 	saveDialog.SetFilter(storage.NewExtensionFileFilter([]string{".csv"}))

// 	// Устанавливаем начальное расположение
// 	if mw.currentFile != "" {
// 		fileURI := storage.NewFileURI(mw.currentFile)
// 		listableURI, _ := storage.ListerForURI(fileURI)
// 		saveDialog.SetLocation(listableURI)
// 	} else {
// 		// Установка расположения по умолчанию
// 		homeDir, _ := os.UserHomeDir()
// 		defaultPath := filepath.Join(homeDir, "manufacturers.csv")
// 		defaultURI := storage.NewFileURI(defaultPath)
// 		listableURI, _ := storage.ListerForURI(defaultURI)
// 		saveDialog.SetLocation(listableURI)
// 	}

// 	saveDialog.Show()
// }

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

// func (mw *MainWindow) aboutMenu() {
// 	aboutText := fmt.Sprintf(`%s
// %s 1.0.0

// %s: Поляков Кирилл Дмитриевич
// %s: ИЦТМС-2-2
// НИУ МГСУ`,
// 		mw.locale.Translate("Construction Materials Manufacturers"),
// 		mw.locale.Translate("Version"),
// 		mw.locale.Translate("Author"),
// 		mw.locale.Translate("Group"))

// 	dialog.ShowCustom(
// 		mw.locale.Translate("About"),
// 		mw.locale.Translate("Close"),
// 		container.NewVBox(
// 			widget.NewLabel(aboutText),
// 			widget.NewLabel("© 2025"),
// 		),
// 		mw.window,
// 	)
// }

func (mw *MainWindow) showNotification(message string) {
	notification := fyne.NewNotification(
		mw.locale.Translate("Notification"),
		message,
	)
	mw.app.SendNotification(notification)
}

func (mw *MainWindow) refreshTable() {
	// Убрали неиспользуемую переменную manufacturers
	newTable := mw.createManufacturersTable()

	// Используем time.AfterFunc для безопасного обновления UI
	time.AfterFunc(50*time.Millisecond, func() {
		if scroll, ok := mw.mainContainer.Objects[0].(*container.Scroll); ok {
			scroll.Content = newTable
			mw.table = newTable
			scroll.Refresh()
		}
		mw.window.Content().Refresh()
	})
}

func (mw *MainWindow) createManufacturersTable() *widget.Table {
	// Получаем данные с учетом текущей сортировки
	manufacturers, err := mw.controller.GetAllManufacturers()
	if err != nil {
		mw.runOnMainThread(func() {
			dialog.ShowError(err, mw.window)
		})
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
				// Если кликнули по той же колонке - меняем направление сортировки
				if mw.currentSort.column == column {
					mw.currentSort.ascending = !mw.currentSort.ascending
				} else {
					// Иначе сортируем по новой колонке по возрастанию
					mw.currentSort.column = column
					mw.currentSort.ascending = true
				}

				// Применяем сортировку
				if err := mw.controller.Sort(column, mw.currentSort.ascending); err != nil {
					dialog.ShowError(err, mw.window)
					return
				}

				// Обновляем таблицу
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

func (mw *MainWindow) onPrint() {
	loading := dialog.NewProgress("Printing", "Preparing document...", mw.window)
	loading.Show()

	go func() {
		err := mw.controller.Print()

		mw.runInUI(func() {
			loading.Hide()
			if err != nil {
				dialog.ShowError(fmt.Errorf("printing failed: %v", err), mw.window)
				return
			}
			mw.showNotification("Document sent to printer")
		})
	}()
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

func (mw *MainWindow) setupSearch() *widget.Entry {
	mw.searchEntry = widget.NewEntry()
	mw.searchEntry.SetPlaceHolder("Поиск...")
	mw.searchEntry.OnChanged = func(query string) {
		if query == "" {
			mw.refreshTable()
			return
		}

		results, err := mw.controller.Search(query)
		if err != nil {
			dialog.ShowError(err, mw.window)
			return
		}

		// Получаем текущие данные через метод
		oldData := mw.controller.GetManufacturers()
		defer mw.controller.SetManufacturers(oldData) // Восстановим после отрисовки

		mw.controller.SetManufacturers(results)
		mw.refreshTable()
	}
	return mw.searchEntry
}
