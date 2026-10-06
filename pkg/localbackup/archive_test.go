package localbackup

import (
	"archive/zip"
	"bytes"
	"github.com/stretchr/testify/require"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestArchiveRoundTripAndCorruption(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, "storage"), 0700))
	require.NoError(t, os.WriteFile(filepath.Join(root, "storage", "receipt.png"), []byte("fictional receipt"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "cyledger.ini"), []byte("[database]\ntype=sqlite3\n"), 0600))
	destination := filepath.Join(t.TempDir(), "backup.zip")
	settings := []byte(`{"ebk_app_settings":"{\"theme\":\"light\"}"}`)
	require.NoError(t, Create(root, destination, settings, func(path string) error { return os.WriteFile(path, []byte("consistent sqlite snapshot"), 0600) }))
	stage, err := Stage(destination, t.TempDir())
	require.NoError(t, err)
	snapshot, err := os.ReadFile(filepath.Join(stage, "data", "cyledger.db"))
	require.NoError(t, err)
	require.Equal(t, "consistent sqlite snapshot", string(snapshot))
	restored, err := os.ReadFile(filepath.Join(stage, "settings.json"))
	require.NoError(t, err)
	require.Equal(t, settings, restored)
	original, err := zip.OpenReader(destination)
	require.NoError(t, err)
	defer original.Close()
	var damaged bytes.Buffer
	writer := zip.NewWriter(&damaged)
	for _, file := range original.File {
		entry, err := writer.Create(file.Name)
		require.NoError(t, err)
		if file.Name == "data/cyledger.db" {
			_, err = entry.Write([]byte("tampered"))
		} else {
			in, e := file.Open()
			require.NoError(t, e)
			_, err = io.Copy(entry, in)
			in.Close()
		}
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())
	bad := filepath.Join(t.TempDir(), "bad.zip")
	require.NoError(t, os.WriteFile(bad, damaged.Bytes(), 0600))
	_, err = Stage(bad, t.TempDir())
	require.Error(t, err)
}
func TestArchiveRejectsTraversalDuplicateAndSecrets(t *testing.T) {
	for _, names := range [][]string{{"../escape"}, {"/absolute"}, {"storage\\escape"}, {"storage/image", "storage/image"}, {"unexpected"}} {
		var raw bytes.Buffer
		writer := zip.NewWriter(&raw)
		for _, name := range names {
			item, err := writer.Create(name)
			require.NoError(t, err)
			_, err = item.Write([]byte("x"))
			require.NoError(t, err)
		}
		require.NoError(t, writer.Close())
		archive := filepath.Join(t.TempDir(), "bad.zip")
		require.NoError(t, os.WriteFile(archive, raw.Bytes(), 0600))
		_, err := Stage(archive, t.TempDir())
		require.Error(t, err)
	}
	require.Error(t, ValidateSettings([]byte(`{"ebk_user_token":"\"secret\""}`)))
	require.Error(t, ValidateSettings([]byte(`{"ebk_app_settings":"invalid"}`)))
}
