package e2eTest

import (
	"github.com/archstats/archstats/cmd"
	"github.com/archstats/archstats/e2eTest/repo"
	"github.com/archstats/archstats/extensions/components"
	"github.com/jmoiron/sqlx"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"io"
	"strings"
	"testing"
)

func TestElepy(t *testing.T) {
	theRepo, err := repo.EnsureCloned("https://github.com/RyanSusana/elepy", "83d3069")
	assert.NoError(t, err)

	dbFile := t.TempDir() + "/elepy.db"
	db, err := theRepo.GetExportedDB(&repo.ExportDBCommand{
		FileName:   dbFile,
		Extensions: []string{"java", "indentations"},
	})

	assert.NoError(t, err)
	defer db.Close()

	maps, err := queryToRowsOfMaps(db, "SELECT * FROM components")
	assert.NoError(t, err)
	assertTableHasRowsAndColumns(t, "name", maps, []map[string]interface{}{
		{"name": "com.elepy", components.AfferentCouplings: 59, components.EfferentCouplings: 17},
	})

	// Verify that the file_contents table was created and has records
	contents, err := queryToRowsOfMaps(db, "SELECT * FROM file_contents LIMIT 5")
	assert.NoError(t, err)
	assert.NotEmpty(t, contents)

	for _, row := range contents {
		assert.Contains(t, row, "file")
		assert.Contains(t, row, "content")
		assert.Contains(t, row, "report_id")
		assert.Contains(t, row, "timestamp")

		assert.NotEmpty(t, row["file"])
		assert.NotEmpty(t, row["content"])
		
		if strings.HasSuffix(row["file"].(string), ".java") {
			assert.Contains(t, row["content"].(string), "package ")
		}
	}
}

func assertTableHasRowsAndColumns(t *testing.T, key string, actual []map[string]interface{}, expected []map[string]interface{}) {
	expectedIndex := lo.Associate(expected, func(item map[string]interface{}) (interface{}, map[string]interface{}) {
		return item[key], item
	})

	actualIndex := lo.Associate(actual, func(item map[string]interface{}) (interface{}, map[string]interface{}) {
		return item[key], item
	})
	for k, expectedMapAndValues := range expectedIndex {
		matchingActual, _ := actualIndex[k]
		assert.NotNil(t, matchingActual)

		for expectedKey, expectedValue := range expectedMapAndValues {
			matchingActualValue, _ := matchingActual[expectedKey]
			switch matchingActualValue.(type) {
			case int64:
				assert.Equal(t, int64(expectedValue.(int)), matchingActualValue)
			default:
				assert.Equal(t, expectedValue, matchingActualValue)
			}
		}
	}
}
func queryToRowsOfMaps(db *sqlx.DB, query string) ([]map[string]interface{}, error) {
	var results []map[string]interface{}
	rows, err := db.Queryx(query)

	if err != nil {
		return nil, err
	}
	for rows.Next() {
		mapResult := make(map[string]interface{})
		err := rows.MapScan(mapResult)
		if err != nil {
			return nil, err
		}

		results = append(results, mapResult)
	}
	return results, nil
}

func TestSQLiteExporterStoreContentFlag(t *testing.T) {
	theRepo, err := repo.EnsureCloned("https://github.com/RyanSusana/elepy", "83d3069")
	assert.NoError(t, err)

	// Run with --store-content=false
	dbFile := t.TempDir() + "/elepy_no_content.db"
	args := []string{
		"-f", theRepo.Location,
		"-e", "java",
		"export", "sqlite", dbFile,
		"--store-content=false",
	}

	err = cmd.Execute(io.Discard, io.Discard, nil, args)
	assert.NoError(t, err)

	db, err := sqlx.Connect("sqlite3", dbFile)
	assert.NoError(t, err)
	defer db.Close()

	// Verify that table file_contents does NOT exist
	var count int
	err = db.Get(&count, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name='file_contents'")
	assert.NoError(t, err)
	assert.Equal(t, 0, count)
}
