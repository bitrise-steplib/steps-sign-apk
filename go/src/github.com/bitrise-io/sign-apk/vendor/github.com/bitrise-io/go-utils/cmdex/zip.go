package cmdex

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/bitrise-io/go-utils/pathutil"
)

// UnZIP ...
func UnZIP(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer func() {
		if err := r.Close(); err != nil {
			log.Fatal(err)
		}
	}()

	if err := os.MkdirAll(dest, 0755); err != nil {
		return err
	}

	// Closure to address file descriptors issue with all the deferred .Close() methods
	extractAndWriteFile := func(f *zip.File) error {
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer func() {
			if err := rc.Close(); err != nil {
				log.Fatal(err)
			}
		}()

		path := filepath.Join(dest, f.Name)

		if f.FileInfo().IsDir() {
			fmt.Printf("*** dir path: %s\n", path)
			if err := os.MkdirAll(path, f.Mode()); err != nil {
				return err
			}
			fmt.Printf("*** ok dir path: %s\n", path)
		} else {
			fmt.Printf("*** filr path: %s\n", path)
			f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return err
			}
			fmt.Printf("*** ok filr path: %s\n", path)
			defer func() {
				if err := f.Close(); err != nil {
					log.Fatal(err)
				}
			}()

			if _, err = io.Copy(f, rc); err != nil {
				return err
			}
		}
		return nil
	}

	for _, f := range r.File {
		if err := extractAndWriteFile(f); err != nil {
			return err
		}
	}
	return nil
}

// DownloadAndUnZIP ...
func DownloadAndUnZIP(url, pth string) error {
	tmpDir, err := pathutil.NormalizedOSTempDirPath("")
	if err != nil {
		return err
	}
	srcFilePath := tmpDir + "/target.zip"
	srcFile, err := os.Create(srcFilePath)
	if err != nil {
		return err
	}
	defer func() {
		if err := srcFile.Close(); err != nil {
			log.Fatal("Failed to close srcFile:", err)
		}
		if err := os.Remove(srcFilePath); err != nil {
			log.Fatal("Failed to remove srcFile:", err)
		}
	}()

	response, err := http.Get(url)
	if err != nil {
		return err
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			log.Fatal("Failed to close response body:", err)
		}
	}()

	if response.StatusCode != http.StatusOK {
		errorMsg := "Failed to download target from: " + url
		return errors.New(errorMsg)
	}

	if _, err := io.Copy(srcFile, response.Body); err != nil {
		return err
	}

	return UnZIP(srcFilePath, pth)
}

// ZIP ...
func ZIP(source, target string) error {
	zipfile, err := os.Create(target)
	if err != nil {
		return err
	}
	defer func() {
		if err := zipfile.Close(); err != nil {
			log.Fatal(err)
		}
	}()

	archive := zip.NewWriter(zipfile)
	defer func() {
		if err := archive.Close(); err != nil {
			log.Fatal(err)
		}
	}()

	info, err := os.Stat(source)
	if err != nil {
		return nil
	}

	var baseDir string
	if info.IsDir() {
		baseDir = filepath.Base(source)
	}

	filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		if baseDir != "" {
			header.Name = filepath.Join(baseDir, strings.TrimPrefix(path, source))
		}

		if info.IsDir() {
			header.Name += "/"
		} else {
			header.Method = zip.Deflate
		}

		writer, err := archive.CreateHeader(header)
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer func() {
			if err := file.Close(); err != nil {
				log.Fatal(err)
			}
		}()

		_, err = io.Copy(writer, file)
		return err
	})

	return err
}
