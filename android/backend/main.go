// CYLedger's Android host uses the same Go services and SQLite driver as desktop.
package main

/*
#include <stdlib.h>
*/
import "C"

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	_ "time/tzdata"
	"unsafe"

	"github.com/mayswind/ezbookkeeping/cmd"
	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/urfave/cli/v3"
	"gopkg.in/ini.v1"
)

var startOnce sync.Once

//export CYLedgerStart
func CYLedgerStart(directory *C.char, port C.int) *C.char {
	var result error
	startOnce.Do(func() {
		root := C.GoString(directory)
		configPath, err := configure(root, int(port))
		if err != nil {
			result = err
			return
		}
		os.Setenv("EBK_WORK_DIR", root)
		// Android's current trust store is supplied by the platform TrustManager.
		os.Setenv("SSL_CERT_FILE", filepath.Join(root, "ca-certificates.pem"))
		core.Version = "CYLedger-Android-0.1.1"
		app := &cli.Command{
			Name:     "CYLedger",
			Commands: []*cli.Command{cmd.WebServer},
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "conf-path"},
				&cli.BoolFlag{Name: "no-boot-log"},
			},
		}
		result = app.Run(context.Background(), []string{"CYLedger", "--conf-path", configPath, "--no-boot-log", "server", "run"})
	})
	if result != nil {
		// No financial data, configuration values or credentials leave the backend.
		return C.CString("手机账本服务启动失败，请保留应用数据并重新打开。")
	}
	return nil
}

//export CYLedgerFree
func CYLedgerFree(value *C.char) { C.free(unsafe.Pointer(value)) }

//export CYLedgerSession
func CYLedgerSession() *C.char {
	value, err := personalSession()
	if err != nil {
		return nil
	}
	return C.CString(value)
}

func configure(root string, port int) (string, error) {
	if port < 1024 || port > 65535 {
		return "", fmt.Errorf("invalid local port")
	}
	for _, name := range []string{"data", "storage", "log"} {
		if err := os.MkdirAll(filepath.Join(root, name), 0700); err != nil {
			return "", err
		}
	}
	configPath := filepath.Join(root, "cyledger.ini")
	sources := []interface{}{filepath.Join(root, "defaults.ini")}
	if _, err := os.Stat(configPath); err == nil {
		sources = append(sources, configPath)
	}
	config, err := ini.Load(sources[0], sources[1:]...)
	if err != nil {
		return "", err
	}
	values := map[string]map[string]string{
		"global": {"mode": "production"},
		"server": {"protocol": "http", "http_addr": "127.0.0.1", "http_port": fmt.Sprint(port),
			"domain": "127.0.0.1", "root_url": fmt.Sprintf("http://127.0.0.1:%d/", port),
			"static_root_path": filepath.Join(root, "public"), "log_request": "false"},
		"database": {"type": "sqlite3", "db_path": filepath.Join(root, "data", "cyledger.db"),
			"max_open_conn": "1", "log_query": "false", "auto_update_database": "true"},
		"storage": {"type": "local_filesystem", "local_filesystem_path": filepath.Join(root, "storage")},
		"log":     {"mode": "file", "level": "warn", "log_path": filepath.Join(root, "log", "cyledger.log"), "log_file_rotate": "true"},
		"user":    {"enable_register": "false"},
		"auth":    {"enable_forget_password": "false", "oauth2_auto_register": "false"},
		"mail":    {"enable_smtp": "false"},
		"mcp":     {"enable_mcp": "false"},
		"llm":     {"transaction_from_ai_text_recognition": "false", "transaction_from_ai_image_recognition": "false"},
	}
	for section, options := range values {
		for key, value := range options {
			config.Section(section).Key(key).SetValue(value)
		}
	}
	if config.Section("security").Key("secret_key").String() == "" {
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return "", err
		}
		config.Section("security").Key("secret_key").SetValue(hex.EncodeToString(key))
	}
	if err := config.SaveTo(configPath); err != nil {
		return "", err
	}
	if err := os.Chmod(configPath, 0600); err != nil {
		return "", err
	}
	return configPath, nil
}

func main() {}
