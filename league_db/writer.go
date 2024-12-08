package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/marcboeker/go-duckdb"
)

func (l *League) OpenWriter() {
	var err error

	l.db, err = sql.Open("duckdb", "")
	if err != nil {
		log.Panic(err)
	}
}

func (l *League) CloseWriter() {
	l.db.Close()
}

func (l *League) jsonToParquet(fnJson string, fnParquet string) {
	_, err := l.db.ExecContext(context.Background(),
		fmt.Sprintf("COPY (SELECT * FROM READ_JSON_AUTO('%s')) TO 'data/%s' (FORMAT PARQUET)",
			fnJson, fnParquet))
	if err != nil {
		log.Panic(err)
	}
}

func (l *League) WriteParquet(data any, name string) {
	bytes, err := json.Marshal(data)
	if err != nil {
		log.Panic(err)
	}

	f := getTmpFile(name)
	_, err = f.Write(bytes)
	if err != nil {
		log.Panic(err)
	}

	f.Close()

	defer os.Remove(f.Name())

	l.jsonToParquet(f.Name(), fmt.Sprintf("%s.parquet", name))
}

func getTmpFile(name string) *os.File {
	f, err := os.CreateTemp(os.TempDir(), fmt.Sprintf("%s*.json", name))
	if err != nil {
		log.Panic(err)
	}

	return f
}

func (l *League) MergeParquet(pattern string, target string) {
	tmp := fmt.Sprintf("data/TMP_%s.parquet", target)
	merged := fmt.Sprintf("data/%s.parquet", target)

	files, err := filepath.Glob(fmt.Sprintf("data/%s.parquet", pattern))
	if err != nil {
		log.Panic(err)
	}

	if len(files) == 0 {
		return
	}

	_, err = os.Stat(merged)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Panic(err)
		}
	} else {
		files = append(files, merged)
	}

	sql := fmt.Sprintf("COPY (SELECT * FROM read_parquet(['%s'], union_by_name=True)) TO '%s'",
		strings.Join(files, "','"), tmp)
	_, err = l.db.ExecContext(context.Background(), sql)
	if err != nil {
		log.Panic(err)
	}

	for _, f := range files {
		err = os.Remove(f)
		if err != nil && os.IsNotExist(err) {
			log.Panic(err)
		}
	}

	err = os.Rename(tmp, merged)
	if err != nil {
		log.Panic(err)
	}
}

func (l *League) SessionExists(sessionPrefix string, subsessionId int) bool {
	_, err := os.Stat(fmt.Sprintf("data/%sresults.parquet", sessionPrefix))
	if os.IsNotExist(err) {
		return false
	}

	if err != nil {
		log.Panic(err)
	}

	sql := fmt.Sprintf("SELECT EXISTS (FROM 'data/%sresults.parquet' WHERE subsession_id=%d)",
		sessionPrefix, subsessionId)

	var exists bool
	err = l.db.QueryRowContext(context.Background(), sql).Scan(&exists)
	if err != nil {
		log.Panic(err)
	}

	return exists
}
