package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type uploadResponse struct {
	Message  string `json:"message"`
	FileURL  string `json:"file_url"`
	Filename string `json:"filename"`
}

func UploadPhoto(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		http.Error(
			w,
			"Method not allowed",
			http.StatusMethodNotAllowed,
		)
		return
	}

	file, header, err := r.FormFile("photo")

	if err != nil {
		http.Error(
			w,
			"Failed to read file",
			http.StatusBadRequest,
		)
		return
	}

	defer file.Close()

	err = os.MkdirAll("./uploads", 0755)
	if err != nil {
		http.Error(
			w,
			"Failed to prepare upload directory",
			http.StatusInternalServerError,
		)
		return
	}

	filename := safeUploadFilename(header.Filename)

	dst, err := os.Create(filepath.Join("./uploads", filename))

	if err != nil {
		http.Error(
			w,
			"Failed to save file",
			http.StatusInternalServerError,
		)
		return
	}

	defer dst.Close()

	_, err = io.Copy(dst, file)
	if err != nil {
		http.Error(
			w,
			"Failed to save file",
			http.StatusInternalServerError,
		)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(uploadResponse{
		Message:  "Photo uploaded successfully",
		FileURL:  "/uploads/" + filename,
		Filename: filename,
	})
}

func safeUploadFilename(original string) string {
	ext := filepath.Ext(original)
	name := strings.TrimSuffix(filepath.Base(original), ext)
	name = strings.ToLower(name)
	name = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' {
			return r
		}
		if r >= '0' && r <= '9' {
			return r
		}
		if r == '-' || r == '_' {
			return r
		}
		return '-'
	}, name)
	name = strings.Trim(name, "-_")

	if name == "" {
		name = "wedding-photo"
	}

	return name + "-" + time.Now().Format("20060102150405") + ext
}
