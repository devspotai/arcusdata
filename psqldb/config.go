package psqldb

type ConnectionPoolConfig struct {
	MaxOpenConns           int
	MaxIdleConns           int
	ConnMaxLifetimeInHours int
	ConnMaxIdleTimeInMin   int
	HealthCheckPeriodInSec int
}

type Config struct {
	ApplicationRWDatabaseURL              string
	ApplicationSchemaMigrationDatabaseUrl string
	ApplicationServerPort                 string
	ConnectionPoolConfig                  ConnectionPoolConfig
}
