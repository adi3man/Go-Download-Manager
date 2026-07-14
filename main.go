package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go-download-manager/downloader" // Pastikan modul internal Anda masih sesuai

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
		"title":          "Go Download Manager",
		"tab_tasks":      "Tasks",
		"tab_settings":   "Settings",
		"btn_add":        "Add New Download",
		"btn_clear_all":  "Clear Finished",
		"status_ready":   "Ready",
		"status_conn":    "Connecting",
		"status_down":    "Downloading",
		"status_paused":  "Paused",
		"status_fail":    "Failed",
		"status_done":    "Finished",
		"setting_dir":    "Main Download Directory",
		"setting_cat":    "File Extension Categories (Comma separated)",
		"btn_browse":     "Choose Folder",
		"btn_save_cat":   "Save Category Settings",
		"dialog_saved":   "Saved",
		"dialog_saved_m": "Category settings updated successfully!",
		"dialog_add_t":   "Add Download",
		"dialog_add_b":   "Start",
		"dialog_cancel":  "Cancel",
		"lang_label":     "Language / Bahasa:",
		"unknown_size":   "Unknown size",
		"col_name":       "Name",
		"col_size":       "Size",
		"col_status":     "Status / Speed",
		"col_date":       "Date Added",
		"menu_pause":     "Pause",
		"menu_resume":    "Resume / Start",
		"menu_refresh":   "Refresh Download Address",
		"menu_delete":    "Delete Task",
	},
	"id": {
		"title":          "Go Download Manager",
		"tab_tasks":      "Tugas",
		"tab_settings":   "Pengaturan",
		"btn_add":        "Tambah Baru",
		"btn_clear_all":  "Bersihkan Selesai",
		"status_ready":   "Siap",
		"status_conn":    "Menghubungkan",
		"status_down":    "Mengunduh",
		"status_paused":  "Ditangguhkan",
		"status_fail":    "Gagal",
		"status_done":    "Selesai",
		"setting_dir":    "Direktori Utama Unduhan",
		"setting_cat":    "Ekstensi File Kategori (Pisahkan dengan koma)",
		"btn_browse":     "Pilih Folder",
		"btn_save_cat":   "Simpan Pengaturan Kategori",
		"dialog_saved":   "Tersimpan",
		"dialog_saved_m": "Pengaturan kategori berhasil diperbarui!",
		"dialog_add_t":   "Tambah Unduhan",
		"dialog_add_b":   "Mulai",
		"dialog_cancel":  "Batal",
		"lang_label":     "Language / Bahasa:",
		"unknown_size":   "Ukuran tidak diketahui",
		"col_name":       "Nama",
		"col_size":       "Ukuran",
		"col_status":     "Status / Kecepatan",
		"col_date":       "Tanggal Dibuat",
		"menu_pause":     "Jeda",
		"menu_resume":    "Lanjutkan / Mulai",
		"menu_refresh":   "Perbarui Alamat Unduhan (URL)",
		"menu_delete":    "Hapus Tugas",
	},
}

func getText(key string) string {
	return T[currentLang][key]
}

type DownloadRequest struct {
	URL string `json:"url"`
}

type DownloadItem struct {
	ID          int
	Filename    string
	URL         string
	Progress    float64
	Status      string // Menyimpan key translasi statis ("status_conn", dll)
	Downloaded  int64
	TotalSize   int64
	Speed       string
	DateAdded   string
	IsFinished  bool
	IsPaused    bool
	CancelFunc  context.CancelFunc
	Context     context.Context
}

var (
	myApp         fyne.App
	myWindow      fyne.Window
	downloads     []*DownloadItem
	downloadMutex sync.Mutex
	downloadList  *widget.List
	nextID        = 1

	// UI Elements Header & Control
	addBtn         *widget.Button
	clearAllBtn    *widget.Button
	colNameLabel   *widget.Label
	colSizeLabel   *widget.Label
	colStatLabel   *widget.Label
	colDateLabel   *widget.Label
	folderCard     *widget.Card
	categoryCard   *widget.Card
	saveCatBtn     *widget.Button
	langLabel      *widget.Label
	tabs           *container.AppTabs
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

	home, _ := os.UserHomeDir()
	defaultPath := filepath.Join(home, "Downloads")
		if prefs.String("download_path") == "" {
			prefs.SetString("download_path", defaultPath)
		}
		loadCategoryRules(prefs)

		// --- ROW HEADER DI ATAS DAFTAR DOWNLOAD ---
		colNameLabel = widget.NewLabelWithStyle(getText("col_name"), fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
		colSizeLabel = widget.NewLabelWithStyle(getText("col_size"), fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
		colStatLabel = widget.NewLabelWithStyle(getText("col_status"), fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
		colDateLabel = widget.NewLabelWithStyle(getText("col_date"), fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

		// Header Grid Layout untuk menyelaraskan kolom secara horizontal
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
				// Membuat layout 4 kolom
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

				// KUNCI PERBAIKAN: Gunakan widget kustom tappable untuk mendeteksi klik kanan
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

				// Ambil container kustom kita, lalu masuk ke grid di dalamnya
				customRow := object.(*rightClickableContainer)
				maxContainer := customRow.Container.(*fyne.Container)
				rowGrid := maxContainer.Objects[0].(*fyne.Container)

				nameLabel := rowGrid.Objects[0].(*widget.Label)
				sizeLabel := rowGrid.Objects[1].(*widget.Label)

				statusBox := rowGrid.Objects[2].(*fyne.Container)
				statusLabel := statusBox.Objects[0].(*widget.Label)
				progressBar := statusBox.Objects[1].(*widget.ProgressBar)

				dateLabel := rowGrid.Objects[3].(*widget.Label)

				// Masukkan data download ke baris terkait
				nameLabel.SetText(item.Filename)
				dateLabel.SetText(item.DateAdded)

				// Simpan ID baris saat ini ke dalam widget kustom agar tahu baris mana yang diklik kanan
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

		// Interaksi Klik Kanan pada Baris List
		downloadList.OnSelected = func(id widget.ListItemID) {
			downloadList.Unselect(id) // Hanya hilangkan seleksi biru tanpa aksi apa-apa
		}

		// --- TOMBOL-TOMBOL DI ATAS LIST ---
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

		categoryForm := container.NewVBox()
		categoryEntries := make(map[string]*widget.Entry)

		for cat := range downloader.DefaultCategories {
			savedRules := prefs.String("category_" + cat)
			entry := widget.NewEntry()
			entry.SetText(savedRules)
			categoryEntries[cat] = entry

			categoryForm.Add(container.NewBorder(
				nil, nil,
				widget.NewLabelWithStyle(cat+":", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
							     nil,
					entry,
			))
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

		settingsTab := container.NewVScroll(container.NewVBox(
			langContainer,
			folderCard,
			categoryCard,
			saveCatBtn,
		))

		// --- SUSUN TABS ---
		tabs = container.NewAppTabs(
			container.NewTabItemWithIcon(getText("tab_tasks"), theme.ListIcon(), downloadTab),
					    container.NewTabItemWithIcon(getText("tab_settings"), theme.SettingsIcon(), settingsTab),
		)

		if desk, ok := myApp.(desktop.App); ok {
			menu := fyne.NewMenu("GDM",
					     fyne.NewMenuItem("Tampilkan", func() { myWindow.Show() }),
					     fyne.NewMenuItem("Keluar", func() { myApp.Quit() }),
			)
			desk.SetSystemTrayMenu(menu)
			myWindow.SetCloseIntercept(func() { myWindow.Hide() })
		}

		// Set bahasa terpilih saat baru berjalan
		if currentLang == "en" {
			langSelect.SetSelected("English")
		} else {
			langSelect.SetSelected("Bahasa Indonesia")
		}

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
						  if confirmed && urlEntry.Text != "" {
							  addNewDownload(urlEntry.Text)
						  }
					  },
					  myWindow,
		)

		customDialog.Resize(fyne.NewSize(750, 400))
		customDialog.Show()
}

// --- POPUP MENU KLIK KANAN ---
// --- POPUP MENU KLIK KANAN ---
// --- POPUP MENU KLIK KANAN ---
func showRowContextMenu(item *DownloadItem) {
	var menuItems []*fyne.MenuItem

	// Menu Pause / Resume dinamis berdasarkan kondisi task
	if !item.IsFinished {
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

		// Menu ganti URL / Refresh download address
		refreshAddressItem := fyne.NewMenuItem(getText("menu_refresh"), func() {
			showRefreshAddressDialog(item)
		})
		menuItems = append(menuItems, refreshAddressItem)
	}

	// Menu Hapus
	deleteItem := fyne.NewMenuItem(getText("menu_delete"), func() {
		removeDownloadItem(item.ID)
	})
	menuItems = append(menuItems, deleteItem)

	menu := fyne.NewMenu("", menuItems...)
	popupMenu := widget.NewPopUpMenu(menu, myWindow.Canvas())

	// Solusi Terbaik & Aman: Ambil ukuran konten jendela aktif saat ini,
	// lalu posisikan popup tepat di tengah-tengah layar agar mudah dijangkau.
	contentSize := myWindow.Content().Size()
	centerPosition := fyne.NewPos(contentSize.Width/2 - 50, contentSize.Height/2 - 50)

	popupMenu.ShowAtPosition(centerPosition)
}

// --- ENGINE LOGIKA DOWNLOAD ---
func addNewDownload(url string) {
	filename := downloader.GetFilenameFromURLAndHeader(url)

	ctx, cancel := context.WithCancel(context.Background())

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
	}
	nextID++
	downloads = append(downloads, item)
	downloadMutex.Unlock()

	downloadList.Refresh()
	go runDownloadRoutine(item)
}

func runDownloadRoutine(item *DownloadItem) {
	prefs := myApp.Preferences()
	baseDir := prefs.String("download_path")
	categoryRules := getActiveCategoryMap()

	// Menghitung kecepatan download secara real-time
	var lastDownloaded int64
	var lastTime = time.Now()

	err := DownloadFileWithContext(
		item.Context,
		item.URL,
		baseDir,
		categoryRules,
		item.Downloaded, // offset untuk resume
		func(actualFilename string) {
			downloadMutex.Lock()
			item.Filename = actualFilename
			downloadMutex.Unlock()
			downloadList.Refresh()
		},
		func(progress float64, currentDownloaded int64, size int64) {
			downloadMutex.Lock()
			item.Progress = progress
			item.Status = "status_down"
			item.Downloaded = currentDownloaded
			if size > 0 {
				item.TotalSize = size
			}

			// Mengukur Kecepatan Download Per Detik
			now := time.Now()
			duration := now.Sub(lastTime).Seconds()
			if duration >= 1.0 {
				bytesSec := float64(currentDownloaded-lastDownloaded) / duration
				item.Speed = formatSpeed(bytesSec)
				lastDownloaded = currentDownloaded
				lastTime = now
			}
			downloadMutex.Unlock()

			downloadList.Refresh()
		},
	)

	downloadMutex.Lock()
	if err != nil {
		if item.IsPaused {
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
	downloadList.Refresh()
}

func pauseDownload(item *DownloadItem) {
	downloadMutex.Lock()
	if !item.IsFinished && !item.IsPaused {
		item.IsPaused = true
		item.CancelFunc() // Memutus request HTTP yang sedang berjalan
	}
	downloadMutex.Unlock()
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
								if confirmed && urlEntry.Text != "" {
									downloadMutex.Lock()
									item.URL = urlEntry.Text
									downloadMutex.Unlock()
									resumeDownload(item) // Langsung mulai menggunakan alamat baru
								}
							},
					  myWindow,
		)
		customDialog.Resize(fyne.NewSize(750, 250))
		customDialog.Show()
}

// Format byte/sec ke KB/s atau MB/s secara rapi
func formatSpeed(bytesPerSec float64) string {
	if bytesPerSec >= 1024*1024 {
		return fmt.Sprintf("%.2f MB/s", bytesPerSec/(1024*1024))
	} else if bytesPerSec >= 1024 {
		return fmt.Sprintf("%.2f KB/s", bytesPerSec/1024)
	}
	return fmt.Sprintf("%.0f B/s", bytesPerSec)
}

// Fungsi Clear All Finished: Menghapus semua yang berstatus sukses / gagal
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
	downloadList.Refresh()
}

func removeDownloadItem(id int) {
	downloadMutex.Lock()
	indexToHapus := -1
	for i, item := range downloads {
		if item.ID == id {
			indexToHapus = i
			// Jika sedang berjalan, matikan terlebih dahulu koneksinya
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
	downloadList.Refresh()
}

// --- ADAPTER DOWNLOADING FILE MENDUKUNG CONTEXT ---
func DownloadFileWithContext(
	ctx context.Context,
	url string,
	baseDir string,
	categoryRules map[string]string,
	offset int64,
	onFilenameSelected func(string),
			     onProgress func(float64, int64, int64),
) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	// Jika ada offset sebelumnya, tambahkan Range Header untuk melanjutkan (Resume)
	if offset > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("server returned status: %s", resp.Status)
	}

	filename := downloader.GetFilenameFromURLAndHeader(url)
	onFilenameSelected(filename)

	savePath := filepath.Join(baseDir, filename)

	var file *os.File
	if offset > 0 {
		// Buka mode append untuk melanjutkan penulisan berkas
		file, err = os.OpenFile(savePath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
	} else {
		file, err = os.Create(savePath)
	}
	if err != nil {
		return err
	}
	defer file.Close()

	totalSize := resp.ContentLength
	if offset > 0 {
		totalSize += offset
	}

	buffer := make([]byte, 32*1024)
	var accumulated = offset

	for {
		select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				n, err := resp.Body.Read(buffer)
				if n > 0 {
					_, writeErr := file.Write(buffer[:n])
					if writeErr != nil {
						return writeErr
					}
					accumulated += int64(n)

					var progress float64 = -1
					if totalSize > 0 {
						progress = float64(accumulated) / float64(totalSize)
					}
					onProgress(progress, accumulated, totalSize)
				}
				if err != nil {
					if err == io.EOF {
						return nil
					}
					return err
				}
		}
	}
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
			if err != nil || req.URL == "" {
				http.Error(w, "Bad Request", http.StatusBadRequest)
				return
			}

			myWindow.Show()
			myWindow.RequestFocus()

			addNewDownload(req.URL)

			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status": "queued"}`))
			return
		}
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	})

	if err := http.ListenAndServe("localhost:18080", nil); err != nil {
		panic(err)
	}
}
// rightClickableContainer adalah widget kustom untuk mendeteksi klik kanan (TappedSecondary)
type rightClickableContainer struct {
	widget.BaseWidget
	Container fyne.CanvasObject
	itemID    widget.ListItemID
}

func (r *rightClickableContainer) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(r.Container)
}

// Tapped menangani klik kiri biasa (tidak melakukan apa-apa)
func (r *rightClickableContainer) Tapped(event *fyne.PointEvent) {}

// TappedSecondary menangani klik kanan secara spesifik
func (r *rightClickableContainer) TappedSecondary(event *fyne.PointEvent) {
	downloadMutex.Lock()
	if int(r.itemID) >= len(downloads) {
		downloadMutex.Unlock()
		return
	}
	item := downloads[r.itemID]
	downloadMutex.Unlock()

	// Munculkan menu aksi langsung di titik kursor mouse saat diklik kanan!
	showRowContextMenuAt(item, event.AbsolutePosition)
}

// Modifikasi fungsi showRowContextMenu agar muncul presisi di lokasi kursor klik kanan
func showRowContextMenuAt(item *DownloadItem, pos fyne.Position) {
	var menuItems []*fyne.MenuItem

	if !item.IsFinished {
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
	}

	deleteItem := fyne.NewMenuItem(getText("menu_delete"), func() {
		removeDownloadItem(item.ID)
	})
	menuItems = append(menuItems, deleteItem)

	menu := fyne.NewMenu("", menuItems...)
	popupMenu := widget.NewPopUpMenu(menu, myWindow.Canvas())

	// Tampilkan tepat di posisi koordinat klik kanan mouse Anda
	popupMenu.ShowAtPosition(pos)
}
