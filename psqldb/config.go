package psqldb

type ConnectionPoolConfig struct {
	MaxOpenConns           int
	MaxIdleConns           int
	ConnMaxLifetimeInHours int
	ConnMaxIdleTimeInMin   int
	HealthCheckPeriodInSec int
}

func (c *ConnectionPoolConfig) SetDefaults() {
	if c.MaxOpenConns == 0 {
		c.MaxOpenConns = 25
	}
	if c.MaxIdleConns == 0 {
		c.MaxIdleConns = 25
	}
	if c.ConnMaxLifetimeInHours == 0 {
		c.ConnMaxLifetimeInHours = 1
	}
	if c.ConnMaxIdleTimeInMin == 0 {
		c.ConnMaxIdleTimeInMin = 15
	}
	if c.HealthCheckPeriodInSec == 0 {
		c.HealthCheckPeriodInSec = 30
	}
}

func (c ConnectionPoolConfig) IsValid() bool {
	if c.MaxOpenConns < 0 || c.MaxIdleConns < 0 {
		return false
	}
	if c.ConnMaxLifetimeInHours < 0 || c.ConnMaxIdleTimeInMin < 0 || c.HealthCheckPeriodInSec < 0 {
		return false
	}
	return true
}

type Config struct {
	ApplicationRWDatabaseURL              string
	ApplicationSchemaMigrationDatabaseUrl string
	ApplicationServerPort                 string
	ConnectionPoolConfig                  ConnectionPoolConfig
}
