package main

/*
#include <stdlib.h>
*/
import "C"
import (
	"database/sql"
	"encoding/json"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/localbackup"
	"os"
	"path/filepath"
)

func backupReply(value string, err error) *C.char {
	result := map[string]string{"value": value}
	if err != nil {
		result["error"] = localbackup.Failure(err)
	}
	raw, _ := json.Marshal(result)
	return C.CString(string(raw))
}

//export CYLedgerBackup
func CYLedgerBackup(root, destination, settings *C.char) *C.char {
	core.LocalRequestGate.Lock()
	defer core.LocalRequestGate.Unlock()
	core.LocalCronGate.Lock()
	defer core.LocalCronGate.Unlock()
	directory := C.GoString(root)
	db, err := sql.Open("sqlite3", filepath.Join(directory, "data", "cyledger.db")+"?_busy_timeout=10000")
	if err != nil {
		return backupReply("", err)
	}
	defer db.Close()
	err = localbackup.Create(directory, C.GoString(destination), []byte(C.GoString(settings)), func(target string) error { _, e := db.Exec("VACUUM INTO ?", target); return e })
	return backupReply(C.GoString(destination), err)
}

//export CYLedgerStageRestore
func CYLedgerStageRestore(archive, parent *C.char) *C.char {
	stage, err := localbackup.Stage(C.GoString(archive), C.GoString(parent))
	if err != nil {
		return backupReply("", err)
	}
	db, err := sql.Open("sqlite3", filepath.Join(stage, "data", "cyledger.db")+"?mode=ro&immutable=1")
	if err == nil {
		var integrity string
		err = db.QueryRow("PRAGMA integrity_check").Scan(&integrity)
		if err == nil && integrity != "ok" {
			err = fmt.Errorf("invalid database")
		}
		var count int
		if err == nil {
			err = db.QueryRow("SELECT COUNT(*) FROM user WHERE deleted=0 AND disabled=0").Scan(&count)
			if err == nil && count != 1 {
				err = fmt.Errorf("invalid personal owner")
			}
		}
		if err == nil {
			var total int
			err = db.QueryRow("SELECT COUNT(*) FROM user").Scan(&total)
			if err == nil && total != 1 {
				err = fmt.Errorf("ambiguous owner")
			}
		}
		db.Close()
	}
	if err != nil {
		os.RemoveAll(stage)
		return backupReply("", err)
	}
	return backupReply(stage, nil)
}
