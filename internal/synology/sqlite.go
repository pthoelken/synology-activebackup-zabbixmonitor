package synology

import (
	"database/sql"
	"fmt"
	"net/url"
	"sort"
	"strings"

	_ "modernc.org/sqlite"
)

type TableInfo struct {
	Name    string
	Columns []ColumnInfo
}

type ColumnInfo struct {
	Name string
	Type string
}

func OpenSQLiteReadOnly(path string) (*sql.DB, error) {
	db, err := openSQLite(path, false)
	if err == nil {
		return db, nil
	}
	immutableDB, immutableErr := openSQLite(path, true)
	if immutableErr == nil {
		return immutableDB, nil
	}
	return nil, fmt.Errorf("open sqlite read-only failed: %w; immutable fallback failed: %v", err, immutableErr)
}

func openSQLite(path string, immutable bool) (*sql.DB, error) {
	u := url.URL{Scheme: "file", Path: path}
	q := u.Query()
	q.Set("mode", "ro")
	q.Set("_pragma", "busy_timeout(1000)")
	if immutable {
		q.Set("immutable", "1")
	}
	u.RawQuery = q.Encode()
	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func ListTables(db *sql.DB) ([]TableInfo, error) {
	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table' ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []TableInfo
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		columns, err := ListColumns(db, name)
		if err != nil {
			continue
		}
		tables = append(tables, TableInfo{Name: name, Columns: columns})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return tables, nil
}

func ListColumns(db *sql.DB, table string) ([]ColumnInfo, error) {
	rows, err := db.Query("PRAGMA table_info(" + QuoteIdent(table) + ")")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []ColumnInfo
	for rows.Next() {
		var cid int
		var name string
		var typ string
		var notNull int
		var defaultValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
			return nil, err
		}
		columns = append(columns, ColumnInfo{Name: name, Type: typ})
	}
	sort.Slice(columns, func(i, j int) bool {
		return columns[i].Name < columns[j].Name
	})
	return columns, rows.Err()
}

func HasTable(tables []TableInfo, table string) bool {
	for _, t := range tables {
		if strings.EqualFold(t.Name, table) {
			return true
		}
	}
	return false
}

func ColumnNames(columns []ColumnInfo) map[string]string {
	out := map[string]string{}
	for _, col := range columns {
		out[NormalizeName(col.Name)] = col.Name
	}
	return out
}

func NormalizeName(name string) string {
	name = strings.ToLower(name)
	replacer := strings.NewReplacer("-", "_", " ", "_", ".", "_")
	return replacer.Replace(name)
}

func QuoteIdent(ident string) string {
	return `"` + strings.ReplaceAll(ident, `"`, `""`) + `"`
}
