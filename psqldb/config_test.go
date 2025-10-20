package psqldb

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoad(t *testing.T) {
	config := Load()
	assert.NotEmpty(t, config.ApplicationRWDatabaseURL)
	assert.NotEmpty(t, config.ApplicationServerPort)
}
