package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/maxvegac/portico/src/internal/addon"
	"github.com/maxvegac/portico/src/internal/config"
	"github.com/maxvegac/portico/src/internal/util"
)

// databaseTypes are the addon types that support database management and linking
var databaseTypes = map[string]bool{
	"postgresql": true,
	"mysql":      true,
	"mariadb":    true,
	"mongodb":    true,
}

// generateSecret returns the initial value for an addon secret.
// Names are predictable; passwords are random (never a fixed default).
func generateSecret(secretName string) (string, error) {
	nameLower := strings.ToLower(secretName)
	switch {
	case strings.Contains(nameLower, "password"):
		return util.RandomSecret(16)
	case strings.Contains(nameLower, "user"):
		return "admin", nil
	case strings.Contains(nameLower, "name") || strings.Contains(nameLower, "db"):
		return "database", nil
	case strings.Contains(nameLower, "root"):
		return "root", nil
	default:
		return util.RandomSecret(16)
	}
}

// loadDatabaseInstance loads an addon instance and checks it is a database
func loadDatabaseInstance(cfg *config.Config, instanceName string) (*addon.Instance, string, error) {
	am := addon.NewManager(cfg.AddonsDir, filepath.Join(cfg.AddonsDir, "instances"))
	addonConfig, err := am.LoadConfig()
	if err != nil {
		return nil, "", fmt.Errorf("error loading addons config: %w", err)
	}

	instance, exists := addonConfig.Instances[instanceName]
	if !exists {
		return nil, "", fmt.Errorf("addon instance %s not found", instanceName)
	}
	if !databaseTypes[instance.Type] {
		return nil, "", fmt.Errorf("addon instance %s is not a database type", instanceName)
	}

	instanceDir := filepath.Join(cfg.AddonsDir, "instances", instanceName)
	if _, err := os.Stat(filepath.Join(instanceDir, "docker-compose.yml")); err != nil {
		return nil, "", fmt.Errorf("docker-compose.yml not found for instance %s", instanceName)
	}
	return &instance, instanceDir, nil
}

// databaseQuery runs a query with the database client inside an addon container,
// authenticating with the credentials stored in the container's /run/secrets.
// The query is passed as a positional argument to sh, so it is never re-parsed by the shell.
func databaseQuery(instanceDir, instanceType, query string) ([]byte, error) {
	var script string
	switch instanceType {
	case "postgresql":
		script = `psql -v ON_ERROR_STOP=1 -At -U "$(cat /run/secrets/db_user)" -d "$(cat /run/secrets/db_name)" -c "$1"`
	case "mysql", "mariadb":
		// MariaDB 11+ ships only the "mariadb" client
		script = `client=$(command -v mariadb || command -v mysql); "$client" -N -uroot -p"$(cat /run/secrets/root_password)" -e "$1"`
	case "mongodb":
		script = `mongosh --quiet -u "$(cat /run/secrets/db_user)" -p "$(cat /run/secrets/db_password)" --authenticationDatabase admin --eval "$1"`
	default:
		return nil, fmt.Errorf("unsupported database type %s", instanceType)
	}

	composeFile := filepath.Join(instanceDir, "docker-compose.yml")
	cmd := exec.Command("docker", "compose", "-f", composeFile, "exec", "-T", instanceType, "sh", "-c", script, "sh", query)
	cmd.Dir = instanceDir
	return cmd.CombinedOutput()
}

// createDatabaseQuery returns the query that creates a database (name must be validated).
// appUser is the user apps connect as: on MySQL/MariaDB it only has privileges on the
// initial database, so it is granted access to the new one. (On PostgreSQL and
// MongoDB the app user is the superuser.)
func createDatabaseQuery(instanceType, dbName, appUser string) string {
	switch instanceType {
	case "postgresql":
		return fmt.Sprintf(`CREATE DATABASE "%s"`, dbName)
	case "mongodb":
		return fmt.Sprintf(`db.getSiblingDB('%s').createCollection('init')`, dbName)
	default:
		query := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s`", dbName)
		if appUser != "" && util.ValidateDBIdentifier(appUser) == nil {
			query += fmt.Sprintf("; GRANT ALL PRIVILEGES ON `%s`.* TO '%s'@'%%'", dbName, appUser)
		}
		return query
	}
}

// databaseAppUser returns the user apps use to connect to a database instance
func databaseAppUser(instanceDir string) string {
	return readSecret(filepath.Join(instanceDir, "secrets", "db_user"))
}

// dropDatabaseQuery returns the query that deletes a database (name must be validated)
func dropDatabaseQuery(instanceType, dbName string) string {
	switch instanceType {
	case "postgresql":
		return fmt.Sprintf(`DROP DATABASE "%s"`, dbName)
	case "mongodb":
		return fmt.Sprintf(`db.getSiblingDB('%s').dropDatabase()`, dbName)
	default:
		return fmt.Sprintf("DROP DATABASE `%s`", dbName)
	}
}

// listDatabasesQuery returns the query that lists databases
func listDatabasesQuery(instanceType string) string {
	switch instanceType {
	case "postgresql":
		return `SELECT datname FROM pg_database WHERE NOT datistemplate ORDER BY 1`
	case "mongodb":
		return `db.adminCommand({listDatabases: 1}).databases.forEach(function (d) { print(d.name) })`
	default:
		return `SHOW DATABASES`
	}
}
