package psqldb

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoad(t *testing.T) {
	config := ConnectionPoolConfig{}
	config.SetDefaults()
	assert.True(t, config.IsValid())

	mainConfig := Config{
		ConnectionPoolConfig: config,
	}

	assert.NotNil(t, mainConfig.ConnectionPoolConfig)
}
