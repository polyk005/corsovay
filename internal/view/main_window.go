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
)

type MainWindow struct {
	app           fyne.App
	window        fyne.Window
	table         *widget.Table
	chartWindow   fyne.Window
	searchEntry   *widget.Entry
	searchResults []model.Manufacturer
	isSearching   bool
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

	content := container.NewMax(mw.mainContainer)

	// Настраиваем окно
	mw.window.SetContent(mw.mainContainer)
	mw.window.SetContent(content)
	mw.window.Resize(fyne.NewSize(1000, 600))
	mw.window.SetFixedSize(false)

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
	var manufacturers []model.Manufacturer
	var err error

	if mw.isSearching {
		manufacturers = mw.searchResults
	} else {
		manufacturers, err = mw.controller.GetAllManufacturers()
		if err != nil {
			dialog.ShowError(err, mw.window)
			return widget.NewTable(nil, nil, nil)
		}
	}

	table := widget.NewTable(
		func() (int, int) {
			return len(manufacturers) + 1, 9 // +1 для заголовков, 9 столбцов
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("template")
		},
		func(tci widget.TableCellID, co fyne.CanvasObject) {
			label := co.(*widget.Label)
			if tci.Row == 0 { // Заголовки
				headers := []string{
					mw.locale.Translate("ID"),
					mw.locale.Translate("Name"),
					mw.locale.Translate("Country"),
					mw.locale.Translate("Address"),
					mw.locale.Translate("Phone"),
					mw.locale.Translate("Email"),
					mw.locale.Translate("Product Type"),
					mw.locale.Translate("Founded Year"),
					mw.locale.Translate("Revenue"),
				}
				label.SetText(headers[tci.Col])
				label.TextStyle.Bold = true
			} else if tci.Row-1 < len(manufacturers) {
				m := manufacturers[tci.Row-1]
				values := []string{
					fmt.Sprintf("%d", m.ID),
					m.Name,
					m.Country,
					m.Address,
					m.Phone,
					m.Email,
					m.ProductType,
					fmt.Sprintf("%d", m.FoundedYear),
					fmt.Sprintf("%.2f", m.Revenue),
				}
				label.SetText(values[tci.Col])
			}
		},
	)

	// Настройка ширины столбцов
	table.SetColumnWidth(0, 50)  // ID
	table.SetColumnWidth(1, 150) // Name
	table.SetColumnWidth(2, 260) // Country
	table.SetColumnWidth(3, 200) // Address
	table.SetColumnWidth(4, 120) // Phone
	table.SetColumnWidth(5, 180) // Email
	table.SetColumnWidth(6, 180) // Product Type
	table.SetColumnWidth(7, 180) // Founded Year
	table.SetColumnWidth(8, 180) // Revenue

	// Обработчик сортировки
	table.OnSelected = func(id widget.TableCellID) {
		if id.Row == 0 {
			columns := []string{"id", "name", "country", "address", "phone",
				"email", "productType", "foundedYear", "revenue"}
			if id.Col < len(columns) {
				ascending := !mw.currentSort.ascending
				var err error

				if mw.isSearching {
					mw.searchResults, err = mw.controller.Sort(mw.searchResults, columns[id.Col], ascending)
				} else {
					var all []model.Manufacturer
					all, err = mw.controller.GetAllManufacturers()
					if err == nil {
						all, err = mw.controller.Sort(all, columns[id.Col], ascending)
						if err == nil {
							mw.controller.SetManufacturers(all)
						}
					}
				}

				if err == nil {
					mw.currentSort.column = columns[id.Col]
					mw.currentSort.ascending = ascending
					mw.refreshTable()
				} else {
					dialog.ShowError(err, mw.window)
				}
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

	// Кнопка генерации графика
	generateBtn := widget.NewButton("Сгенерировать график", func() {
		if columnSelect.Selected == "" {
			dialog.ShowInformation("Ошибка", "Выберите колонку для графика", mw.window)
			return
		}

		imgData, err := mw.controller.GenerateChart(columnSelect.Selected)
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

		// Создаем и настраиваем изображение
		chartImg := canvas.NewImageFromImage(img)
		chartImg.FillMode = canvas.ImageFillOriginal
		chartImg.SetMinSize(fyne.NewSize(10, 20))

		// Обновляем содержимое окна
		if mw.chartWindow != nil {
			mw.chartWindow.SetContent(container.NewScroll(chartImg))
			mw.chartWindow.Canvas().Refresh(chartImg)
		}

	})

	// Создаем контейнер с элементами управления
	controls := container.NewVBox(
		widget.NewLabel("Выберите данные для графика:"),
		columnSelect,
		generateBtn,
	)

	// Создаем окно для графика (если еще не создано)
	if mw.chartWindow == nil {
		mw.chartWindow = mw.app.NewWindow("График производителей")
		mw.chartWindow.SetContent(controls)
		mw.chartWindow.Resize(fyne.NewSize(850, 650)) // Используем стандартный NewSize из первой версии
		mw.chartWindow.SetOnClosed(func() {
			mw.chartWindow = nil
		})
	}

	mw.chartWindow.Show()
}

func (mw *MainWindow) setupSearch() *widget.Entry {
	mw.searchEntry = widget.NewEntry()
	mw.searchEntry.SetPlaceHolder("Поиск...")
	mw.searchEntry.OnChanged = func(query string) {
		if query == "" {
			mw.isSearching = false
			mw.refreshTable()
			return
		}

		results, err := mw.controller.Search(query)
		if err != nil {
			dialog.ShowError(err, mw.window)
			return
		}

		mw.searchResults = results
		mw.isSearching = true
		mw.refreshTable()
	}
	return mw.searchEntry
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
