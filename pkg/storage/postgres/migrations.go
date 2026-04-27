package postgres

import (
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type migration struct {
	version int
	name    string
	sql     string
}

func loadMigrations(schema string) ([]migration, error) {
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return nil, err
	}

	out := make([]migration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		versionPart, _, ok := strings.Cut(entry.Name(), "_")
		if !ok {
			return nil, fmt.Errorf("postgres: nome de migration invalido: %s", entry.Name())
		}
		version, err := strconv.Atoi(versionPart)
		if err != nil {
			return nil, fmt.Errorf("postgres: versao de migration invalida %q: %w", entry.Name(), err)
		}

		body, err := migrationsFS.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return nil, err
		}

		out = append(out, migration{
			version: version,
			name:    entry.Name(),
			sql:     strings.ReplaceAll(string(body), "{{schema}}", quoteIdentifier(schema)),
		})
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].version < out[j].version
	})
	return out, nil
}
