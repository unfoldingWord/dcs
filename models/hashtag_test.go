package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetHashtagSummary(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	results, err := GetHashtagSummary("repo", 10)
	assert.NoError(t, err)

	assert.Len(t, results, 2)
	assert.Equal(t, []map[string]string{
		{"tag_name": "tag1", "count_of_occurrences": "2"},
		{"tag_name": "tag2", "count_of_occurrences": "1"},
	}, results)
}
