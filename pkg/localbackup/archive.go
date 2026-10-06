// Package localbackup implements the mobile archive format without depending on
// the UI or a running service. Live SQLite files are never copied for backup.
package localbackup

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

const Format = "cyledger-mobile-backup-v1"
const maxTotal = int64(2 * 1024 * 1024 * 1024)

type Manifest struct {
	Format    string            `json:"format"`
	CreatedAt int64             `json:"createdAt"`
	Files     map[string]string `json:"files"`
}

func allowed(name string) bool {
	return name == "cyledger.ini" || name == "data/cyledger.db" || name == "settings.json" || strings.HasPrefix(name, "storage/")
}
func safe(name string) bool {
	return name != "" && name == path.Clean(name) && !strings.ContainsAny(name, "\\:\x00") && !strings.HasPrefix(name, "/") && name != ".." && !strings.HasPrefix(name, "../")
}
func ValidateSettings(raw []byte) error {
	if len(raw) > 2*1024*1024 {
		return errors.New("settings too large")
	}
	var values map[string]string
	if json.Unmarshal(raw, &values) != nil || values == nil {
		return errors.New("invalid settings")
	}
	for key, value := range values {
		if key != "ebk_app_settings" && !strings.HasPrefix(key, "cy_ledger_experience_") {
			return errors.New("unsupported setting")
		}
		if len(key) > 160 || !json.Valid([]byte(value)) {
			return errors.New("invalid setting value")
		}
	}
	return nil
}
func Create(root, destination string, settings []byte, snapshot func(string) error) error {
	if err := ValidateSettings(settings); err != nil {
		return err
	}
	temp, err := os.MkdirTemp(filepath.Dir(destination), ".snapshot-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)
	db := filepath.Join(temp, "cyledger.db")
	if err = snapshot(db); err != nil {
		return err
	}
	files := map[string]string{"data/cyledger.db": db, "cyledger.ini": filepath.Join(root, "cyledger.ini")}
	err = filepath.WalkDir(filepath.Join(root, "storage"), func(name string, entry os.DirEntry, err error) error {
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return errors.New("symlink not allowed")
		}
		if entry.IsDir() {
			return nil
		}
		rel, e := filepath.Rel(root, name)
		if e != nil {
			return e
		}
		files[filepath.ToSlash(rel)] = name
		return nil
	})
	if err != nil {
		return err
	}
	if len(files) > 50000 {
		return errors.New("too many files")
	}
	out, err := os.OpenFile(destination+".part", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer func() { out.Close(); os.Remove(destination + ".part") }()
	writer := zip.NewWriter(out)
	manifest := Manifest{Format: Format, CreatedAt: time.Now().Unix(), Files: map[string]string{}}
	total := int64(0)
	add := func(name string, input io.Reader) error {
		if !safe(name) || !allowed(name) {
			return errors.New("invalid archive path")
		}
		item, e := writer.Create(name)
		if e != nil {
			return e
		}
		hash := sha256.New()
		n, e := io.Copy(io.MultiWriter(item, hash), io.LimitReader(input, maxTotal-total+1))
		total += n
		if e != nil {
			return e
		}
		if total > maxTotal {
			return errors.New("archive too large")
		}
		manifest.Files[name] = hex.EncodeToString(hash.Sum(nil))
		return nil
	}
	for name, file := range files {
		input, e := os.Open(file)
		if e != nil {
			return e
		}
		e = add(name, input)
		input.Close()
		if e != nil {
			return e
		}
	}
	if err = add("settings.json", strings.NewReader(string(settings))); err != nil {
		return err
	}
	raw, _ := json.Marshal(manifest)
	entry, err := writer.Create("manifest.json")
	if err != nil {
		return err
	}
	if _, err = entry.Write(raw); err != nil {
		return err
	}
	if err = writer.Close(); err != nil {
		return err
	}
	if err = out.Sync(); err != nil {
		return err
	}
	if err = out.Close(); err != nil {
		return err
	}
	return os.Rename(destination+".part", destination)
}

// Stage validates paths, sizes and every hash before returning a restore tree.
// The caller validates SQLite and swaps the tree only after the service stops.
func Stage(archive, parent string) (string, error) {
	reader, err := zip.OpenReader(archive)
	if err != nil {
		return "", err
	}
	defer reader.Close()
	if len(reader.File) > 50001 {
		return "", errors.New("too many files")
	}
	var manifest Manifest
	seen := map[string]bool{}
	found := false
	var total uint64
	for _, file := range reader.File {
		if !safe(file.Name) || seen[file.Name] || file.Mode()&os.ModeSymlink != 0 || file.FileInfo().IsDir() {
			return "", errors.New("unsafe archive entry")
		}
		seen[file.Name] = true
		total += file.UncompressedSize64
		if file.UncompressedSize64 > uint64(maxTotal) || total > uint64(maxTotal) {
			return "", errors.New("archive too large")
		}
		if file.Name == "manifest.json" {
			if file.UncompressedSize64 > 8*1024*1024 {
				return "", errors.New("manifest too large")
			}
			in, e := file.Open()
			if e != nil {
				return "", e
			}
			raw, e := io.ReadAll(io.LimitReader(in, 8*1024*1024+1))
			in.Close()
			if e != nil || json.Unmarshal(raw, &manifest) != nil {
				return "", errors.New("invalid manifest")
			}
			found = true
		} else if !allowed(file.Name) {
			return "", errors.New("unexpected file")
		}
	}
	if !found || manifest.Format != Format || len(manifest.Files) != len(reader.File)-1 || manifest.Files["data/cyledger.db"] == "" || manifest.Files["cyledger.ini"] == "" || manifest.Files["settings.json"] == "" {
		return "", errors.New("incomplete backup")
	}
	stage, err := os.MkdirTemp(parent, "restore-")
	if err != nil {
		return "", err
	}
	ok := false
	defer func() {
		if !ok {
			os.RemoveAll(stage)
		}
	}()
	for _, file := range reader.File {
		if file.Name == "manifest.json" {
			continue
		}
		expected := manifest.Files[file.Name]
		if len(expected) != 64 {
			return "", errors.New("invalid file hash")
		}
		target := filepath.Join(stage, filepath.FromSlash(file.Name))
		if err = os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			return "", err
		}
		in, e := file.Open()
		if e != nil {
			return "", e
		}
		out, e := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			in.Close()
			return "", e
		}
		hash := sha256.New()
		n, e := io.Copy(io.MultiWriter(out, hash), io.LimitReader(in, int64(file.UncompressedSize64)+1))
		in.Close()
		syncErr := out.Sync()
		out.Close()
		if e != nil || syncErr != nil || uint64(n) != file.UncompressedSize64 || hex.EncodeToString(hash.Sum(nil)) != expected {
			return "", errors.New("backup checksum failed")
		}
	}
	settings, err := os.ReadFile(filepath.Join(stage, "settings.json"))
	if err != nil {
		return "", err
	}
	if err = ValidateSettings(settings); err != nil {
		return "", err
	}
	ok = true
	return stage, nil
}
func Failure(err error) string {
	if err == nil {
		return ""
	}
	return fmt.Sprint("备份文件无效或操作未完成；现有账本保持不变。")
}
