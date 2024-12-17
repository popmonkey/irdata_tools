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

func getTmpFile(name string) *os.File {
	f, err := os.CreateTemp(os.TempDir(), fmt.Sprintf("%s*.json", name))
	if err != nil {
		log.Panic(err)
	}

	return f
}

func (l *League) WriteJson(data any, name string) {
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

	// this conversion normalizes the raw json fixing stuff like
	//  timestamps to be consistent
	sql := fmt.Sprintf("COPY (SELECT * FROM read_json('%s')) TO 'data/%s.json'", f.Name(), name)
	_, err = l.db.ExecContext(context.Background(), sql)
	if err != nil {
		log.Panic(err)
	}
}

func (l *League) MergeJson(pattern string, target string) {
	tmp := fmt.Sprintf("data/TMP_%s.json", target)
	merged := fmt.Sprintf("data/%s.json", target)

	files, err := filepath.Glob(fmt.Sprintf("data/%s.json", pattern))
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

	sql := fmt.Sprintf("COPY (SELECT * FROM read_json(['%s'], union_by_name=true)) TO '%s'",
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

func (l *League) ConvertJsonToParquet(fn string) {
	sql := fmt.Sprintf("COPY (SELECT * FROM read_json('data/%s.json')) TO 'data/%s.parquet'", fn, fn)

	_, err := l.db.ExecContext(context.Background(), sql)
	if err != nil {
		log.Panic(err)
	}

	os.Remove(fmt.Sprintf("data/%s.json", fn))
}

func (l *League) ConvertParquetToJson(fn string) {
	sql := fmt.Sprintf("COPY (SELECT * FROM read_parquet('data/%s.parquet')) TO 'data/%s.json'", fn, fn)

	// ignore errors
	l.db.ExecContext(context.Background(), sql)
}
