package e2eTest

import (
	"github.com/archstats/archstats/e2eTest/repo"
	"github.com/archstats/archstats/extensions/components"
	"github.com/jmoiron/sqlx"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestElepy(t *testing.T) {
	theRepo, _ := repo.EnsureCloned("https://github.com/RyanSusana/elepy", "83d3069")

	db, err := theRepo.GetExportedDB(&repo.ExportDBCommand{
		Extensions: []string{"java", "indentations"},
	})

	assert.NoError(t, err)
	maps, err := queryToRowsOfMaps(db, "SELECT * FROM components")
	assert.NoError(t, err)
	assertTableHasRowsAndColumns(t, "name", maps, []map[string]interface{}{
		{"name": "com.elepy", components.AfferentCouplings: 59, components.EfferentCouplings: 17},
	})
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
