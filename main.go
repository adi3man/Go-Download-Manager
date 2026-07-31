package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"go-download-manager/downloader" // Pastikan modul internal Anda sesuai

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// --- STATE BAHASA ---
var currentLang = "en"

var T = map[string]map[string]string{
	"en": {
		"title":            "Go Download Manager",
		"tab_tasks":        "Tasks",
		"tab_settings":     "Settings",
		"btn_add":          "Add New Download",
		"btn_clear_all":    "Clear Finished",
		"status_ready":     "Ready",
		"status_conn":      "Connecting",
		"status_down":      "Downloading",
		"status_paused":    "Paused",
		"status_fail":      "Failed",
		"status_done":      "Finished",
		"setting_dir":      "Main Download Directory",
		"setting_cat":      "File Extension Categories (Comma separated)",
		"btn_browse":       "Choose Folder",
		"btn_save_cat":     "Save Category Settings",
		"dialog_saved":     "Saved",
		"dialog_saved_m":   "Category settings updated successfully!",
		"dialog_add_t":     "Add Download",
		"dialog_add_b":     "Start",
		"dialog_cancel":    "Cancel",
		"lang_label":       "Language / Bahasa:",
		"unknown_size":     "Unknown size",
		"col_name":         "Name",
		"col_size":         "Size",
		"col_status":       "Status / Speed",
		"col_date":         "Date Added",
		"menu_pause":       "Pause",
		"menu_resume":      "Resume / Start",
		"menu_refresh":     "Refresh Download Address",
		"menu_delete":      "Delete Task",
		"menu_open_file":   "Open File",
		"menu_open_folder": "Open Containing Folder",
		"menu_delete_file": "Delete File from Disk",
		"dialog_del_title": "Delete File",
		"dialog_del_msg":   "Are you sure you want to delete this file from your disk?",
	},
	"id": {
		"title":            "Go Download Manager",
		"tab_tasks":        "Tugas",
		"tab_settings":     "Pengaturan",
		"btn_add":          "Tambah Baru",
		"btn_clear_all":    "Bersihkan Selesai",
		"status_ready":     "Siap",
		"status_conn":      "Menghubungkan",
		"status_down":      "Mengunduh",
		"status_paused":    "Ditangguhkan",
		"status_fail":      "Gagal",
		"status_done":      "Selesai",
		"setting_dir":      "Direktori Utama Unduhan",
		"setting_cat":      "Ekstensi File Kategori (Pisahkan dengan koma)",
		"btn_browse":       "Pilih Folder",
		"btn_save_cat":     "Simpan Pengaturan Kategori",
		"dialog_saved":     "Tersimpan",
		"dialog_saved_m":   "Pengaturan kategori berhasil diperbarui!",
		"dialog_add_t":     "Tambah Unduhan",
		"dialog_add_b":     "Mulai",
		"dialog_cancel":    "Batal",
		"lang_label":       "Language / Bahasa:",
		"unknown_size":     "Ukuran tidak diketahui",
		"col_name":         "Nama",
		"col_size":         "Ukuran",
		"col_status":       "Status / Kecepatan",
		"col_date":         "Tanggal Dibuat",
		"menu_pause":       "Jeda",
		"menu_resume":      "Lanjutkan / Mulai",
		"menu_refresh":     "Perbarui Alamat Unduhan (URL)",
		"menu_delete":      "Hapus Tugas",
		"menu_open_file":   "Buka File",
		"menu_open_folder": "Buka Folder Lokasi",
		"menu_delete_file": "Hapus File dari Disk",
		"dialog_del_title": "Hapus File",
		"dialog_del_msg":   "Apakah Anda yakin ingin menghapus file ini dari penyimpanan disk?",
	},
}

func getText(key string) string {
	return T[currentLang][key]
}

type DownloadRequest struct {
	URL string `json:"url"`
}

type DownloadItem struct {
	ID         int
	Filename   string
	URL        string
	Progress   float64
	Status     string
	Downloaded int64
	TotalSize  int64
	Speed      string
	DateAdded  string
	IsFinished bool
	IsPaused   bool
	CancelFunc context.CancelFunc
	Context    context.Context
	Task       *downloader.DownloadTask
}

var (
	myApp         fyne.App
	myWindow      fyne.Window
	downloads     []*DownloadItem
	downloadMutex sync.Mutex
	downloadList  *widget.List
	nextID        = 1

	// UI Elements Header & Control
	addBtn       *widget.Button
	clearAllBtn  *widget.Button
	colNameLabel *widget.Label
	colSizeLabel *widget.Label
	colStatLabel *widget.Label
	colDateLabel *widget.Label
	folderCard   *widget.Card
	categoryCard *widget.Card
	saveCatBtn   *widget.Button
	langLabel    *widget.Label
	tabs         *container.AppTabs
)

func main() {
	myApp = app.NewWithID("com.godownloadmanager.app")
	myWindow = myApp.NewWindow(getText("title"))
	myWindow.Resize(fyne.NewSize(950, 600))

	prefs := myApp.Preferences()

	if prefs.String("app_lang") != "" {
		currentLang = prefs.String("app_lang")
		myWindow.SetTitle(getText("title"))
	}

	// Gunakan folder Downloads di home user sebagai path default
	defaultPath := ""
	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		defaultPath = filepath.Join(home, "Downloads")
	} else {
		defaultPath = "Downloads"
	}

	if prefs.String("download_path") == "" {
		prefs.SetString("download_path", defaultPath)
	}

	loadCategoryRules(prefs)
	// Pemanggilan loadDownloadsJSON hanya dilakukan sekali di sini
	loadDownloadsJSON()

	// --- ROW HEADER DI ATAS DAFTAR DOWNLOAD ---
	colNameLabel = widget.NewLabelWithStyle(getText("col_name"), fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	colSizeLabel = widget.NewLabelWithStyle(getText("col_size"), fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	colStatLabel = widget.NewLabelWithStyle(getText("col_status"), fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	colDateLabel = widget.NewLabelWithStyle(getText("col_date"), fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	listHeader := container.NewGridWithColumns(4,
		colNameLabel,
		colSizeLabel,
		colStatLabel,
		colDateLabel,
	)

	// --- LIST TASKS ---
	downloadList = widget.NewList(
		func() int {
			downloadMutex.Lock()
			defer downloadMutex.Unlock()
			return len(downloads)
		},
		func() fyne.CanvasObject {
			nameLabel := widget.NewLabel("file.zip")
			sizeLabel := widget.NewLabel("0.00 MB / 0.00 MB")

			statusLabel := widget.NewLabel("Status: Ready")
			progressBar := widget.NewProgressBar()
			statusBox := container.NewVBox(statusLabel, progressBar)

			dateLabel := widget.NewLabel("2026-04-12 10:00")

			rowGrid := container.NewGridWithColumns(4,
				nameLabel,
				sizeLabel,
				statusBox,
				dateLabel,
			)

			clickableRow := &rightClickableContainer{
				Container: container.NewMax(rowGrid),
			}
			clickableRow.ExtendBaseWidget(clickableRow)

			return clickableRow
		},
		func(id widget.ListItemID, object fyne.CanvasObject) {
			downloadMutex.Lock()
			if id >= len(downloads) {
				downloadMutex.Unlock()
				return
			}
			item := downloads[id]
			downloadMutex.Unlock()

			customRow := object.(*rightClickableContainer)
			maxContainer := customRow.Container.(*fyne.Container)
			rowGrid := maxContainer.Objects[0].(*fyne.Container)

			nameLabel := rowGrid.Objects[0].(*widget.Label)
			sizeLabel := rowGrid.Objects[1].(*widget.Label)

			statusBox := rowGrid.Objects[2].(*fyne.Container)
			statusLabel := statusBox.Objects[0].(*widget.Label)
			progressBar := statusBox.Objects[1].(*widget.ProgressBar)

			dateLabel := rowGrid.Objects[3].(*widget.Label)

			nameLabel.SetText(item.Filename)
			dateLabel.SetText(item.DateAdded)

			customRow.itemID = id

			if item.TotalSize > 0 {
				sizeLabel.SetText(fmt.Sprintf("%.2f MB / %.2f MB", float64(item.Downloaded)/(1024*1024), float64(item.TotalSize)/(1024*1024)))
			} else {
				sizeLabel.SetText(fmt.Sprintf("%.2f MB (%s)", float64(item.Downloaded)/(1024*1024), getText("unknown_size")))
			}

			translatedStatus := getText(item.Status)
			if translatedStatus == "" {
				translatedStatus = item.Status
			}

			if item.Status == "status_down" && item.Speed != "" {
				statusLabel.SetText(fmt.Sprintf("%s (%s)", translatedStatus, item.Speed))
			} else {
				statusLabel.SetText(translatedStatus)
			}

			if item.Progress < 0 {
				progressBar.SetValue(0)
			} else {
				progressBar.SetValue(item.Progress)
			}
		},
	)

	downloadList.OnSelected = func(id widget.ListItemID) {
		downloadList.Unselect(id)
	}

	// --- TOMBOL CONTROL ---
	addBtn = widget.NewButtonWithIcon(getText("btn_add"), theme.ContentAddIcon(), func() {
		showAddDownloadDialog()
	})

	clearAllBtn = widget.NewButtonWithIcon(getText("btn_clear_all"), theme.DeleteIcon(), func() {
		clearFinishedDownloads()
	})
	clearAllBtn.Importance = widget.WarningImportance

	topControl := container.NewHBox(addBtn, clearAllBtn)
	headerArea := container.NewVBox(topControl, listHeader)
	downloadTab := container.NewBorder(headerArea, nil, nil, nil, downloadList)

	// --- TAB 2: SETTINGS ---
	pathEntry := widget.NewEntry()
	pathEntry.SetText(prefs.String("download_path"))
	pathEntry.Disable()

	browseBtn := widget.NewButtonWithIcon(getText("btn_browse"), theme.FolderOpenIcon(), func() {
		folderDialog := dialog.NewFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil || uri == nil {
				return
			}
			selectedPath := uri.Path()
			pathEntry.SetText(selectedPath)
			prefs.SetString("download_path", selectedPath)
		}, myWindow)

		folderDialog.Show()
		folderDialog.Resize(fyne.NewSize(750, 500))
	})

	folderCard = widget.NewCard(getText("setting_dir"), "", container.NewBorder(nil, nil, nil, browseBtn, pathEntry))

	categoryForm := widget.NewForm()
	categoryEntries := make(map[string]*widget.Entry)

	for cat := range downloader.DefaultCategories {
		savedRules := prefs.String("category_" + cat)
		entry := widget.NewEntry()
		entry.SetText(savedRules)
		categoryEntries[cat] = entry

		categoryForm.Append(cat+":", entry)
	}

	saveCatBtn = widget.NewButtonWithIcon(getText("btn_save_cat"), theme.DocumentSaveIcon(), func() {
		for cat, entry := range categoryEntries {
			prefs.SetString("category_"+cat, entry.Text)
		}
		dialog.ShowInformation(getText("dialog_saved"), getText("dialog_saved_m"), myWindow)
	})

	categoryCard = widget.NewCard(getText("setting_cat"), "", categoryForm)

	langLabel = widget.NewLabel(getText("lang_label"))
	langSelect := widget.NewSelect([]string{"English", "Bahasa Indonesia"}, func(selected string) {
		if selected == "English" {
			currentLang = "en"
		} else {
			currentLang = "id"
		}
		prefs.SetString("app_lang", currentLang)
		refreshLanguage(langLabel, browseBtn)
	})

	langContainer := container.NewHBox(langLabel, langSelect)

	minimizeToTrayCheck := widget.NewCheck("Minimize to System Tray on Close", func(checked bool) {
		prefs.SetBool("MinimizeToTray", checked)
	})
	minimizeToTrayCheck.SetChecked(prefs.BoolWithFallback("MinimizeToTray", true))

	settingsTab := container.NewVScroll(container.NewVBox(
		langContainer,
		widget.NewSeparator(),
		minimizeToTrayCheck,
		widget.NewSeparator(),
		folderCard,
		categoryCard,
		saveCatBtn,
	))

	tabs = container.NewAppTabs(
		container.NewTabItemWithIcon(getText("tab_tasks"), theme.ListIcon(), downloadTab),
		container.NewTabItemWithIcon(getText("tab_settings"), theme.SettingsIcon(), settingsTab),
	)

	if currentLang == "en" {
		langSelect.SetSelected("English")
	} else {
		langSelect.SetSelected("Bahasa Indonesia")
	}

	// --- SYSTEM TRAY ---
	if desk, ok := myApp.(desktop.App); ok {
		icon := myApp.Metadata().Icon
		if icon == nil {
			icon = theme.DownloadIcon()
		}

		menu := fyne.NewMenu("GDM",
			fyne.NewMenuItem("Show Application", func() { myWindow.Show() }),
			fyne.NewMenuItemSeparator(),
			fyne.NewMenuItem("Exit", func() { myApp.Quit() }),
		)
		desk.SetSystemTrayMenu(menu)
		desk.SetSystemTrayIcon(icon)
	}

	myWindow.SetCloseIntercept(func() {
		shouldMinimize := prefs.BoolWithFallback("MinimizeToTray", true)
		_, isDesktop := myApp.(desktop.App)

		if shouldMinimize && isDesktop {
			myWindow.Hide()
		} else {
			myApp.Quit()
		}
	})

	go startBackgroundServer()

	myWindow.SetContent(tabs)
	myWindow.ShowAndRun()
}

func refreshLanguage(langLabel *widget.Label, browseBtn *widget.Button) {
	myWindow.SetTitle(getText("title"))
	addBtn.SetText(getText("btn_add"))
	clearAllBtn.SetText(getText("btn_clear_all"))
	browseBtn.SetText(getText("btn_browse"))
	folderCard.SetTitle(getText("setting_dir"))
	categoryCard.SetTitle(getText("setting_cat"))
	saveCatBtn.SetText(getText("btn_save_cat"))
	langLabel.SetText(getText("lang_label"))

	colNameLabel.SetText(getText("col_name"))
	colSizeLabel.SetText(getText("col_size"))
	colStatLabel.SetText(getText("col_status"))
	colDateLabel.SetText(getText("col_date"))

	tabs.Items[0].Text = getText("tab_tasks")
	tabs.Items[1].Text = getText("tab_settings")
	tabs.Refresh()

	downloadList.Refresh()
}

func showAddDownloadDialog() {
	urlEntry := widget.NewEntry()
	urlEntry.SetPlaceHolder("https://example.com/file.zip")

	form := widget.NewForm(
		widget.NewFormItem("URL File", urlEntry),
	)
	scrollContainer := container.NewScroll(form)

	customDialog := dialog.NewCustomConfirm(
		getText("dialog_add_t"),
		getText("dialog_add_b"),
		getText("dialog_cancel"),
		scrollContainer,
		func(confirmed bool) {
			if confirmed && strings.TrimSpace(urlEntry.Text) != "" {
				addNewDownload(strings.TrimSpace(urlEntry.Text))
			}
		},
		myWindow,
	)

	customDialog.Resize(fyne.NewSize(750, 400))
	customDialog.Show()
}

// --- ENGINE LOGIKA DOWNLOAD ---
func addNewDownload(url string) {
	filename := downloader.GetFilenameFromURLAndHeader(url)

	ctx, cancel := context.WithCancel(context.Background())
	prefs := myApp.Preferences()
	baseDir := prefs.String("download_path")
	categoryRules := getActiveCategoryMap()
	task := downloader.NewDownloadTask(url, baseDir, categoryRules)

	downloadMutex.Lock()
	item := &DownloadItem{
		ID:         nextID,
		Filename:   filename,
		URL:        url,
		Progress:   0,
		Status:     "status_conn",
		Downloaded: 0,
		TotalSize:  0,
		Speed:      "",
		DateAdded:  time.Now().Format("2006-01-02 15:04:05"),
		IsFinished: false,
		IsPaused:   false,
		CancelFunc: cancel,
		Context:    ctx,
		Task:       task,
	}
	nextID++
	downloads = append(downloads, item)
	downloadMutex.Unlock()

	saveDownloadsJSON()
	downloadList.Refresh()
	go runDownloadRoutine(item)
}

func runDownloadRoutine(item *DownloadItem) {
	if item.Task == nil {
		return
	}

	var lastDownloaded int64
	var lastTime = time.Now()

	err := item.Task.Start(
		item.Context,
		func(actualFilename string) {
			downloadMutex.Lock()
			item.Filename = actualFilename
			downloadMutex.Unlock()

			// BUNGKUS DENGAN fyne.Do
			fyne.Do(func() {
				downloadList.Refresh()
			})
		},
		func(progress float64, currentDownloaded int64, size int64) {
			downloadMutex.Lock()
			item.Progress = progress
			item.Status = "status_down"
			item.Downloaded = currentDownloaded
			item.TotalSize = size

			now := time.Now()
			duration := now.Sub(lastTime).Seconds()
			if duration >= 1.0 {
				bytesSec := float64(currentDownloaded-lastDownloaded) / duration
				item.Speed = formatSpeed(bytesSec)
				lastDownloaded = currentDownloaded
				lastTime = now
			}
			downloadMutex.Unlock()

			saveDownloadsJSON()

			// BUNGKUS DENGAN fyne.Do
			fyne.Do(func() {
				downloadList.Refresh()
			})
		},
	)

	downloadMutex.Lock()
	if err != nil {
		if item.IsPaused || errors.Is(err, context.Canceled) {
			item.Status = "status_paused"
			item.Speed = ""
		} else {
			item.Status = "status_fail"
			item.Speed = err.Error()
			item.IsFinished = true
		}
	} else {
		item.Status = "status_done"
		item.Progress = 1.0
		item.Speed = ""
		item.IsFinished = true
	}
	downloadMutex.Unlock()

	saveDownloadsJSON()

	// BUNGKUS DENGAN fyne.Do KETIKA SELESAI/GAGAL
	fyne.Do(func() {
		downloadList.Refresh()
	})
}

func pauseDownload(item *DownloadItem) {
	downloadMutex.Lock()
	if !item.IsFinished && !item.IsPaused {
		item.IsPaused = true
		if item.CancelFunc != nil {
			item.CancelFunc()
		}
	}
	downloadMutex.Unlock()

	saveDownloadsJSON()
	downloadList.Refresh()
}

func resumeDownload(item *DownloadItem) {
	downloadMutex.Lock()
	if item.IsPaused {
		item.IsPaused = false
		item.Status = "status_conn"
		ctx, cancel := context.WithCancel(context.Background())
		item.Context = ctx
		item.CancelFunc = cancel
		go runDownloadRoutine(item)
	}
	downloadMutex.Unlock()

	saveDownloadsJSON()
	downloadList.Refresh()
}

func showRefreshAddressDialog(item *DownloadItem) {
	urlEntry := widget.NewEntry()
	urlEntry.SetText(item.URL)

	form := widget.NewForm(
		widget.NewFormItem("New URL Address", urlEntry),
	)

	customDialog := dialog.NewCustomConfirm(
		"Refresh Download Address",
		"Save & Start",
		"Cancel",
		container.NewScroll(form),
		func(confirmed bool) {
			if confirmed && strings.TrimSpace(urlEntry.Text) != "" {
				downloadMutex.Lock()
				item.URL = strings.TrimSpace(urlEntry.Text)
				downloadMutex.Unlock()
				resumeDownload(item)
			}
		},
		myWindow,
	)
	customDialog.Resize(fyne.NewSize(750, 250))
	customDialog.Show()
}

func formatSpeed(bytesPerSec float64) string {
	if bytesPerSec >= 1024*1024 {
		return fmt.Sprintf("%.2f MB/s", bytesPerSec/(1024*1024))
	} else if bytesPerSec >= 1024 {
		return fmt.Sprintf("%.2f KB/s", bytesPerSec/1024)
	}
	return fmt.Sprintf("%.0f B/s", bytesPerSec)
}

func clearFinishedDownloads() {
	downloadMutex.Lock()
	var activeDownloads []*DownloadItem
	for _, item := range downloads {
		if !item.IsFinished {
			activeDownloads = append(activeDownloads, item)
		}
	}
	downloads = activeDownloads
	downloadMutex.Unlock()

	saveDownloadsJSON()
	downloadList.Refresh()
}

func removeDownloadItem(id int) {
	downloadMutex.Lock()
	indexToHapus := -1
	for i, item := range downloads {
		if item.ID == id {
			indexToHapus = i
			if !item.IsFinished && item.CancelFunc != nil {
				item.CancelFunc()
			}
			break
		}
	}

	if indexToHapus != -1 {
		downloads = append(downloads[:indexToHapus], downloads[indexToHapus+1:]...)
	}
	downloadMutex.Unlock()

	saveDownloadsJSON()
	downloadList.Refresh()
}

func loadCategoryRules(prefs fyne.Preferences) {
	for cat, exts := range downloader.DefaultCategories {
		if prefs.String("category_"+cat) == "" {
			prefs.SetString("category_"+cat, exts)
		}
	}
}

func getActiveCategoryMap() map[string]string {
	rules := make(map[string]string)
	prefs := myApp.Preferences()
	for cat := range downloader.DefaultCategories {
		rules[cat] = prefs.String("category_" + cat)
	}
	return rules
}

func startBackgroundServer() {
	http.HandleFunc("/download", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		if r.Method == "POST" {
			var req DownloadRequest
			err := json.NewDecoder(r.Body).Decode(&req)
			if err != nil || strings.TrimSpace(req.URL) == "" {
				http.Error(w, "Bad Request", http.StatusBadRequest)
				return
			}

			// BUNGKUS OPERASI WINDOW & ADD DOWNLOAD DENGAN fyne.Do
			fyne.Do(func() {
				myWindow.Show()
				myWindow.RequestFocus()
				addNewDownload(strings.TrimSpace(req.URL))
			})

			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status": "queued"}`))
			return
		}
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	})

	if err := http.ListenAndServe("localhost:18080", nil); err != nil {
		fmt.Println("Server Error:", err)
	}
}

// Right Clickable Container
type rightClickableContainer struct {
	widget.BaseWidget
	Container fyne.CanvasObject
	itemID    widget.ListItemID
}

func (r *rightClickableContainer) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(r.Container)
}

func (r *rightClickableContainer) Tapped(event *fyne.PointEvent) {}

func (r *rightClickableContainer) TappedSecondary(event *fyne.PointEvent) {
	downloadMutex.Lock()
	if int(r.itemID) >= len(downloads) {
		downloadMutex.Unlock()
		return
	}
	item := downloads[r.itemID]
	downloadMutex.Unlock()

	showRowContextMenuAt(item, event.AbsolutePosition)
}

func showRowContextMenuAt(item *DownloadItem, pos fyne.Position) {
	var menuItems []*fyne.MenuItem
	fullPath := resolveFilePath(item)

	if item.IsFinished {
		openFileItem := fyne.NewMenuItem(getText("menu_open_file"), func() {
			openFile(fullPath)
		})
		menuItems = append(menuItems, openFileItem)

		openFolderItem := fyne.NewMenuItem(getText("menu_open_folder"), func() {
			openContainingFolder(fullPath)
		})
		menuItems = append(menuItems, openFolderItem)

		menuItems = append(menuItems, fyne.NewMenuItemSeparator())
	} else {
		if item.IsPaused {
			resumeItem := fyne.NewMenuItem(getText("menu_resume"), func() {
				resumeDownload(item)
			})
			menuItems = append(menuItems, resumeItem)
		} else {
			pauseItem := fyne.NewMenuItem(getText("menu_pause"), func() {
				pauseDownload(item)
			})
			menuItems = append(menuItems, pauseItem)
		}

		refreshAddressItem := fyne.NewMenuItem(getText("menu_refresh"), func() {
			showRefreshAddressDialog(item)
		})
		menuItems = append(menuItems, refreshAddressItem)

		menuItems = append(menuItems, fyne.NewMenuItemSeparator())
	}

	deleteTaskItem := fyne.NewMenuItem(getText("menu_delete"), func() {
		removeDownloadItem(item.ID)
	})
	menuItems = append(menuItems, deleteTaskItem)

	deleteFileItem := fyne.NewMenuItem(getText("menu_delete_file"), func() {
		dialog.ShowConfirm(
			getText("dialog_del_title"),
			getText("dialog_del_msg")+"\n\n"+item.Filename,
			func(confirmed bool) {
				if confirmed {
					err := os.Remove(fullPath)
					if err != nil {
						fmt.Println("Gagal menghapus file dari disk:", err)
					}
					removeDownloadItem(item.ID)
				}
			},
			myWindow,
		)
	})
	menuItems = append(menuItems, deleteFileItem)

	menu := fyne.NewMenu("", menuItems...)
	popupMenu := widget.NewPopUpMenu(menu, myWindow.Canvas())
	popupMenu.ShowAtPosition(pos)
}

func openFile(filePath string) {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", filePath)
	case "darwin":
		cmd = exec.Command("open", filePath)
	default:
		cmd = exec.Command("xdg-open", filePath)
	}

	_ = cmd.Start()
}

func openContainingFolder(filePath string) {
	folder := filepath.Dir(filePath)
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", "/select,", filePath)
	case "darwin":
		cmd = exec.Command("open", "-R", filePath)
	default:
		cmd = exec.Command("xdg-open", folder)
	}

	_ = cmd.Start()
}

const dataFileName = "downloads.json"

type DownloadItemSave struct {
	ID         int     `json:"id"`
	Filename   string  `json:"filename"`
	URL        string  `json:"url"`
	Progress   float64 `json:"progress"`
	Status     string  `json:"status"`
	Downloaded int64   `json:"downloaded"`
	TotalSize  int64   `json:"total_size"`
	Speed      string  `json:"speed"`
	DateAdded  string  `json:"date_added"`
	IsFinished bool    `json:"is_finished"`
	IsPaused   bool    `json:"is_paused"`
}

func getStorageFilePath() string {
	if myApp != nil && myApp.Storage() != nil && myApp.Storage().RootURI() != nil {
		path := myApp.Storage().RootURI().Path()
		if path != "" {
			_ = os.MkdirAll(path, 0755)
			return filepath.Join(path, dataFileName)
		}
	}
	return dataFileName
}

func saveDownloadsJSON() {
	downloadMutex.Lock()
	defer downloadMutex.Unlock()

	var saveList []DownloadItemSave
	for _, item := range downloads {
		saveList = append(saveList, DownloadItemSave{
			ID:         item.ID,
			Filename:   item.Filename,
			URL:        item.URL,
			Progress:   item.Progress,
			Status:     item.Status,
			Downloaded: item.Downloaded,
			TotalSize:  item.TotalSize,
			Speed:      item.Speed,
			DateAdded:  item.DateAdded,
			IsFinished: item.IsFinished,
			IsPaused:   item.IsPaused,
		})
	}

	data, err := json.MarshalIndent(saveList, "", "  ")
	if err != nil {
		fmt.Println("Error Marshal JSON:", err)
		return
	}

	filePath := getStorageFilePath()
	err = os.WriteFile(filePath, data, 0644)
	if err != nil {
		fmt.Println("Error Write JSON File:", err)
	}
}

func loadDownloadsJSON() {
	downloadMutex.Lock()
	defer downloadMutex.Unlock()

	filePath := getStorageFilePath()
	data, err := os.ReadFile(filePath)
	if err != nil {
		return
	}

	var loadedData []DownloadItemSave
	if err := json.Unmarshal(data, &loadedData); err != nil {
		fmt.Println("Error Unmarshal JSON:", err)
		return
	}

	downloads = nil
	maxID := 0

	for _, s := range loadedData {
		if s.ID > maxID {
			maxID = s.ID
		}

		status := s.Status
		isPaused := s.IsPaused
		if status == "status_down" || status == "status_conn" {
			status = "status_paused"
			isPaused = true
		}

		ctx, cancel := context.WithCancel(context.Background())
		prefs := myApp.Preferences()
		baseDir := prefs.String("download_path")
		categoryRules := getActiveCategoryMap()
		task := downloader.NewDownloadTask(s.URL, baseDir, categoryRules)

		item := &DownloadItem{
			ID:         s.ID,
			Filename:   s.Filename,
			URL:        s.URL,
			Progress:   s.Progress,
			Status:     status,
			Downloaded: s.Downloaded,
			TotalSize:  s.TotalSize,
			Speed:      s.Speed,
			DateAdded:  s.DateAdded,
			IsFinished: s.IsFinished,
			IsPaused:   isPaused,
			CancelFunc: cancel,
			Context:    ctx,
			Task:       task,
		}
		downloads = append(downloads, item)
	}
	nextID = maxID + 1
}

func resolveFilePath(item *DownloadItem) string {
	prefs := myApp.Preferences()
	baseDir := prefs.String("download_path")

	// 1. Cek langsung di folder utama Downloads
	directPath := filepath.Join(baseDir, item.Filename)
	if _, err := os.Stat(directPath); err == nil {
		return directPath
	}

	// 2. Cek berdasarkan ekstensi di daftar kategori
	categoryRules := getActiveCategoryMap()
	ext := strings.ToLower(filepath.Ext(item.Filename))

	for cat, exts := range categoryRules {
		rawExtList := strings.Split(exts, ",")
		for _, e := range rawExtList {
			cleanExt := strings.ToLower(strings.TrimSpace(e))
			if cleanExt == "" {
				continue
			}
			if !strings.HasPrefix(cleanExt, ".") {
				cleanExt = "." + cleanExt
			}

			if cleanExt == ext {
				catPath := filepath.Join(baseDir, cat, item.Filename)
				if _, err := os.Stat(catPath); err == nil {
					return catPath
				}
			}
		}
	}

	// 3. Fallback scan semua subfolder
	entries, err := os.ReadDir(baseDir)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				subPath := filepath.Join(baseDir, entry.Name(), item.Filename)
				if _, err := os.Stat(subPath); err == nil {
					return subPath
				}
			}
		}
	}

	return directPath
}
