package downloader

import (
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

var DefaultCategories = map[string]string{
	"Images":     ".jpg,.jpeg,.png,.gif,.svg",
	"Videos":     ".mp4,.mkv,.avi,.mov,.flv",
	"Music":      ".mp3,.wav,.flac,.m4a",
	"Documents":  ".pdf,.docx,.doc,.xlsx,.pptx,.txt",
	"Compressed": ".zip,.rar,.7z,.tar,.gz",
	"Programs":   ".deb,.rpm,.sh,.exe",
}

// Menentukan nama folder berdasarkan ekstensi file dan aturan kustom
func GetCategoryFolder(filename string, customRules map[string]string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == "" {
		return "Others"
	}

	for folderName, extensions := range customRules {
		extList := strings.Split(extensions, ",")
		for _, e := range extList {
			trimmedExt := strings.TrimSpace(e)
			if !strings.HasPrefix(trimmedExt, ".") {
				trimmedExt = "." + trimmedExt
			}
			if ext == strings.ToLower(trimmedExt) {
				return folderName
			}
		}
	}

	return "Others"
}

// Mendapatkan nama file cadangan dari URL mentah
func GetFilenameFromURLAndHeader(url string) string {
	cleanURL := url
	if idx := strings.Index(url, "?"); idx != -1 {
		cleanURL = url[:idx]
	}

	tokens := strings.Split(cleanURL, "/")
	filename := tokens[len(tokens)-1]

	if filename == "" {
		filename = "download_file"
	}
	return filename
}

// DownloadFile yang mendeteksi nama file asli secara presisi
func DownloadFile(url string, baseDir string, categories map[string]string, onFilenameDetermined func(string), onProgress func(float64, int64, int64)) error {
	// 1. Lakukan HTTP GET Request
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	filename := ""

	// STRATEGI A: Cari di header Content-Disposition
	if contentDisp := resp.Header.Get("Content-Disposition"); contentDisp != "" {
		_, params, err := mime.ParseMediaType(contentDisp)
		if err == nil {
			if actualFilename, ok := params["filename"]; ok && actualFilename != "" {
				filename = actualFilename
			}
		}
	}

	// STRATEGI B: Jika tidak ada di header, ambil dari URL final (setelah redirect)
	if filename == "" {
		finalURLPath := resp.Request.URL.Path
		tokens := strings.Split(finalURLPath, "/")
		potentialName := tokens[len(tokens)-1]
		if potentialName != "" && strings.Contains(potentialName, ".") {
			filename = potentialName
		}
	}

	// STRATEGI C: Jika masih tidak ketemu, ambil dari URL awal yang dibersihkan
	if filename == "" {
		cleanURL := url
		if idx := strings.Index(url, "?"); idx != -1 {
			cleanURL = url[:idx]
		}
		tokens := strings.Split(cleanURL, "/")
		filename = tokens[len(tokens)-1]
	}

	// STRATEGI D: Kebalikan Terakhir (Tebak ekstensi dari Content-Type)
	if filename == "" || !strings.Contains(filename, ".") {
		baseName := "download"
		if filename != "" {
			baseName = filename
		}

		ext := ".bin" // default jika tidak diketahui
		contentType := resp.Header.Get("Content-Type")
		if contentType != "" {
			// Menghilangkan bagian parameter seperti charset=utf-8
			mediaType, _, err := mime.ParseMediaType(contentType)
			if err == nil {
				extensions, err := mime.ExtensionsByType(mediaType)
				if err == nil && len(extensions) > 0 {
					ext = extensions[0]
				}
			}
		}
		filename = baseName + ext
	}

	// Kirim nama file asli yang bersih ke UI
	onFilenameDetermined(filename)

	// 2. Tentukan jalur simpan berdasarkan kategori
	category := GetCategoryFolder(filename, categories)
	targetDir := filepath.Join(baseDir, category)

	err = os.MkdirAll(targetDir, 0755)
	if err != nil {
		return err
	}

	finalPath := filepath.Join(targetDir, filename)

	// 3. Buat file tujuan di disk
	out, err := os.Create(finalPath)
	if err != nil {
		return err
	}
	defer out.Close()

	// 4. Salin data sambil menghitung progress
	size := resp.ContentLength
	buffer := make([]byte, 32*1024)
	var downloaded int64

	for {
		n, err := resp.Body.Read(buffer)
		if n > 0 {
			_, writeErr := out.Write(buffer[:n])
			if writeErr != nil {
				return writeErr
			}
			downloaded += int64(n)

			if size > 0 {
				progress := float64(downloaded) / float64(size)
				onProgress(progress, downloaded, size)
			} else {
				onProgress(-1, downloaded, -1)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	onProgress(1.0, size, size)
	return nil
}
