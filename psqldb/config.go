package psqldb

import (
	"fmt"

	"github.com/devspotai/arcusdata/util"
)

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

func Load() Config {

	return Config{
		ApplicationRWDatabaseURL:              getApplicationRwDatabaseUrl(),
		ApplicationSchemaMigrationDatabaseUrl: getApplicationSchemaMigrationDatabaseUrl(),
		ApplicationServerPort:                 util.GetEnv("APPLICATION_LISTENER_PORT", "8080"),
		ConnectionPoolConfig: ConnectionPoolConfig{
			MaxOpenConns:           util.GetEnvAsInt("APPLICATION_DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns:           util.GetEnvAsInt("APPLICATION_DB_MAX_IDLE_CONNS", 10),
			ConnMaxLifetimeInHours: util.GetEnvAsInt("APPLICATION_DB_CONN_MAX_LIFETIME_HOURS", 5),
			ConnMaxIdleTimeInMin:   util.GetEnvAsInt("APPLICATION_DB_CONN_MAX_IDLE_TIME_MIN", 5),
			HealthCheckPeriodInSec: util.GetEnvAsInt("APPLICATION_DB_HEALTH_CHECK_PERIOD_SEC", 5),
		},
	}
}

func getApplicationRwDatabaseUrl() string {
	var dbUser = util.GetEnv("APPLICATION_DB_APP_USER", "postgres")
	var dbUserPassword = util.GetEnv("APPLICATION_DB_APP_PASSWORD", "password")
	return getApplicationDatabaseUrl(dbUser, dbUserPassword)
}

func getApplicationSchemaMigrationDatabaseUrl() string {
	var dbUser = util.GetEnv("APPLICATION_DB_MIGRATION_USER", "postgres")
	var dbUserPassword = util.GetEnv("APPLICATION_DB_MIGRATION_PASSWORD", "password")
	return getApplicationDatabaseUrl(dbUser, dbUserPassword)
}

func getApplicationDatabaseUrl(dbUser string, dbUserPassword string) string {
	var dbHost = util.GetEnv("APPLICATION_DB_HOST", "localhost")
	var dbName = util.GetEnv("APPLICATION_DB_NAME", "serveyourstaydb")
	var dbListenerPort = util.GetEnv("APPLICATION_DB_LISTENER_PORT", "5432")
	var dbSchema = util.GetEnv("APPLICATION_DB_SCHEMA_NAME", "public")
	var sslRootCert = util.GetEnv("APPLICATION_DB_SSL_ROOT_CERT", "")
	var sslCert = util.GetEnv("APPLICATION_DB_SSL_CLIENT_CERT", "")
	var sslKey = util.GetEnv("APPLICATION_DB_SSL_CLIENT_KEY", "")
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable&search_path=%s", dbUser, dbUserPassword, dbHost, dbListenerPort, dbName, dbSchema)
	if util.GetEnvName() == "prod" {
		dsn = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?search_path=%s&sslmode=verify-full&sslrootcert=%s&sslcert=%s&sslkey=%s", dbUser, dbUserPassword, dbHost, dbListenerPort, dbName, dbSchema, sslRootCert, sslCert, sslKey)
	}
	return dsn
}
