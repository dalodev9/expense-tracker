package migrations

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

//go:embed *.sql
var FS embed.FS

// RunMigrations applies all SQL migrations from the embedded filesystem in alphabetical order.
func RunMigrations(db *sql.DB) error {
	entries, err := fs.ReadDir(FS, ".")
	if err != nil {
		return fmt.Errorf("reading migrations directory: %w", err)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		data, err := FS.ReadFile(entry.Name())
		if err != nil {
			return fmt.Errorf("reading migration file %s: %w", entry.Name(), err)
		}
		if _, err := db.Exec(string(data)); err != nil {
			return fmt.Errorf("executing migration %s: %w", entry.Name(), err)
		}
	}
	return nil
}
