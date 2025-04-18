package view

import (
	"cursovay/internal/controller"
	"cursovay/internal/model"
	"cursovay/pkg/localization"
	"errors"
	"fmt"
	"log"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"fyne.io/fyne"
	"fyne.io/fyne/app"
	"fyne.io/fyne/container"
	"fyne.io/fyne/dialog"
	"fyne.io/fyne/widget"
)

type MainWindow struct {
	app            fyne.App
	window         fyne.Window
	table          *widget.Table
	tableContainer *fyne.Container
	controller     *controller.ManufacturerController
	locale         *localization.Locale
	currentFile    string
	unsavedChanges bool
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

func (mw *MainWindow) setupMenu() *fyne.MainMenu {
	fileMenu := fyne.NewMenu(mw.locale.Translate("File"),
		fyne.NewMenuItem(mw.locale.Translate("New"), mw.onNew),
		fyne.NewMenuItem(mw.locale.Translate("Open"), mw.onOpen),
		fyne.NewMenuItem(mw.locale.Translate("Save"), mw.onSave),
		fyne.NewMenuItem(mw.locale.Translate("Save As"), mw.onSaveAs),
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
	mw.table = mw.createManufacturersTable()

	mw.window.SetContent(container.NewBorder(nil, nil, nil, nil, mw.table))
	mw.window.Resize(fyne.NewSize(1000, 600))

	// Обработка закрытия окна
	mw.window.SetCloseIntercept(func() {
		mw.checkUnsavedChanges(func() {
			mw.window.Close()
		})
	})

	mw.window.ShowAndRun()
}

func (mw *MainWindow) onNew() {
	mw.checkUnsavedChanges(func() {
		mw.controller.NewDatabase()
		mw.currentFile = ""
		mw.unsavedChanges = false
		mw.refreshTable()
		mw.window.SetTitle(mw.locale.Translate("Manufacturers Database"))
	})
}

func (mw *MainWindow) onOpen() {
	mw.checkUnsavedChanges(func() {
		dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
			// Убрали объявление fileFilter, так как оно не используется
			if err != nil {
				dialog.ShowError(err, mw.window)
				return
			}
			if reader == nil {
				return // Пользователь отменил
			}
			defer reader.Close()

			filePath := path.Clean(strings.TrimPrefix(reader.URI().String(), "file://"))
			if err := mw.controller.LoadFromFile(filePath); err != nil {
				dialog.ShowError(err, mw.window)
				return
			}

			mw.currentFile = filePath
			mw.unsavedChanges = false
			mw.refreshTable()
			mw.window.SetTitle(mw.locale.Translate("Manufacturers Database") + " - " + filepath.Base(filePath))
			mw.showNotification(mw.locale.Translate("File loaded successfully"))
		}, mw.window)
	})
}

func (mw *MainWindow) onSave() {
	if mw.currentFile == "" {
		mw.onSaveAs()
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
	dialog.ShowFileSave(func(writer fyne.URIWriteCloser, err error) {
		if err != nil {
			dialog.ShowError(err, mw.window)
			return
		}
		if writer == nil {
			return // Пользователь отменил
		}
		defer writer.Close()

		filePath := path.Clean(strings.TrimPrefix(writer.URI().String(), "file://"))
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
}

func (mw *MainWindow) onExportPDF() {
	// Здесь должна быть реализация экспорта в PDF
	dialog.ShowInformation("Export PDF", "PDF export will be implemented here", mw.window)
}

func (mw *MainWindow) onPrint() {
	// Здесь должна быть реализация печати
	dialog.ShowInformation("Print", "Print functionality will be implemented here", mw.window)
}

func (mw *MainWindow) onAdd() {
	newManufacturer := model.Manufacturer{}
	mw.showEditDialog(&newManufacturer, true)
}

func (mw *MainWindow) onEdit(row int) {
	if row <= 0 {
		return
	}

	manufacturers, err := mw.controller.GetAllManufacturers()
	if err != nil || row-1 >= len(manufacturers) {
		return
	}

	manufacturer := manufacturers[row-1]
	mw.showEditDialog(&manufacturer, false)
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
			mw.refreshTable() // Обязательно обновляем таблицу
			mw.showNotification(mw.locale.Translate("Manufacturer saved successfully"))
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

func (mw *MainWindow) onDelete(row int) {
	if row <= 0 {
		return
	}

	manufacturers, err := mw.controller.GetAllManufacturers()
	if err != nil || row-1 >= len(manufacturers) {
		return
	}

	manufacturer := manufacturers[row-1]
	mw.onDeleteWithConfirmation(manufacturer.ID)
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
	// Здесь должна быть реализация смены языка
	dialog.ShowInformation("Language", fmt.Sprintf("Language changed to %s", lang), mw.window)
	// В реальной реализации здесь нужно:
	// 1. Обновить локаль
	// 2. Перезагрузить все тексты в интерфейсе
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
	// Полная перерисовка содержимого окна
	content := container.NewBorder(nil, nil, nil, nil, mw.createManufacturersTable())
	mw.window.SetContent(content)
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

			if tci.Row-1 < len(manufacturers) {
				m := manufacturers[tci.Row-1]
				switch tci.Col {
				case 0:
					label.SetText(m.Name)
				case 1:
					label.SetText(m.Country)
				case 2:
					label.SetText(m.Address)
				case 3:
					label.SetText(m.Phone)
				case 4:
					label.SetText(m.Email)
				case 5:
					label.SetText(m.ProductType)
				case 6:
					label.SetText(fmt.Sprintf("%d", m.FoundedYear))
				case 7:
					label.SetText(fmt.Sprintf("%.2f", m.Revenue))
				}

			}

		},
	)

	// Настройка ширины столбцов
	table.SetColumnWidth(0, 200) // Name
	table.SetColumnWidth(1, 150) // Country
	table.SetColumnWidth(2, 250) // Address
	table.SetColumnWidth(3, 150) // Phone
	table.SetColumnWidth(4, 200) // Email
	table.SetColumnWidth(5, 200) // Product Type
	table.SetColumnWidth(6, 120) // Founded Year
	table.SetColumnWidth(7, 150) // Revenue

	// Добавляем обработчик кликов для контекстного меню
	table.OnSelected = func(id widget.TableCellID) {
		if id.Row == 0 {
			return // Заголовки
		}

		// Показываем контекстное меню по правому клику
		// (реализация требует дополнительной работы с обработкой событий мыши)
	}

	return table
}
