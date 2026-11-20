package commands

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/crypto/bcrypt"
	"gopkg.in/yaml.v3"

	"github.com/maxvegac/portico/src/internal/addon"
	"github.com/maxvegac/portico/src/internal/config"
	"github.com/maxvegac/portico/src/internal/proxy"
	"github.com/maxvegac/portico/src/internal/util"
)

// Admin tools are addon instances of type "tool": a web UI that reaches the
// databases over portico-network. They always listen on 127.0.0.1 (reachable
// through an SSH tunnel) and, with --domain, are also published by Caddy behind
// HTTP basic auth.

// addonTypeNames lists the bundled addon definitions
var addonTypeNames = []string{"postgresql", "mariadb", "mysql", "mongodb", "redis", "valkey", "adminer", "mongo-express"}

// inlineAddonTypes are added to apps as services named after the type
var inlineAddonTypes = map[string]bool{"redis": true, "valkey": true}

// toolTargets are the database types each admin tool can manage
var toolTargets = map[string]map[string]bool{
	"adminer":       {"postgresql": true, "mysql": true, "mariadb": true},
	"mongo-express": {"mongodb": true},
}

const toolAdminUser = "admin"

// toolCaddyfilePath is where the proxy config of a published tool lives. The
// directory is mounted into the Caddy container at /etc/caddy/addons and
// imported by the main Caddyfile (addon instance directories are never mounted:
// they hold database data and secrets).
func toolCaddyfilePath(cfg *config.Config, instanceName string) string {
	return filepath.Join(cfg.ProxyDir, "addons", instanceName+".Caddyfile")
}

// toolTargetInfo resolves the database instance a tool points at
type toolTargetInfo struct {
	Instance addon.Instance
	Host     string
	Port     int
	Dir      string
}

func resolveToolTarget(cfg *config.Config, am *addon.Manager, instances map[string]addon.Instance, toolType, target string) (*toolTargetInfo, error) {
	if target == "" {
		if toolType == "mongo-express" {
			return nil, fmt.Errorf("--target <mongodb-instance> is required for mongo-express")
		}
		return nil, nil
	}
	inst, ok := instances[target]
	if !ok {
		return nil, fmt.Errorf("addon instance %s not found", target)
	}
	if !toolTargets[toolType][inst.Type] {
		return nil, fmt.Errorf("%s cannot manage %s instances", toolType, inst.Type)
	}
	dir := filepath.Join(cfg.AddonsDir, "instances", target)
	return &toolTargetInfo{
		Instance: inst,
		Host:     addonHost(dir, inst),
		Port:     internalPort(am, inst),
		Dir:      dir,
	}, nil
}

// toolEnvironment returns the environment of a tool container. It is written to
// an env file readable only by portico because it contains credentials.
func toolEnvironment(toolType, adminPassword string, target *toolTargetInfo) (map[string]string, error) {
	env := map[string]string{}
	switch toolType {
	case "adminer":
		if target != nil {
			env["ADMINER_DEFAULT_SERVER"] = fmt.Sprintf("%s:%d", target.Host, target.Port)
		}
	case "mongo-express":
		user := readSecret(filepath.Join(target.Dir, "secrets", "db_user"))
		password := readSecret(filepath.Join(target.Dir, "secrets", "db_password"))
		if user == "" || password == "" {
			return nil, fmt.Errorf("could not read the credentials of %s", target.Instance.Name)
		}
		mongoURL := url.URL{
			Scheme:   "mongodb",
			User:     url.UserPassword(user, password),
			Host:     fmt.Sprintf("%s:%d", target.Host, target.Port),
			Path:     "/",
			RawQuery: "authSource=admin",
		}
		cookieSecret, err := util.RandomSecret(16)
		if err != nil {
			return nil, err
		}
		sessionSecret, err := util.RandomSecret(16)
		if err != nil {
			return nil, err
		}
		env["ME_CONFIG_MONGODB_URL"] = mongoURL.String()
		env["ME_CONFIG_MONGODB_ENABLE_ADMIN"] = "true"
		// mongo-express has its own login; use the same credentials as the proxy
		env["ME_CONFIG_BASICAUTH"] = "true"
		env["ME_CONFIG_BASICAUTH_USERNAME"] = toolAdminUser
		env["ME_CONFIG_BASICAUTH_PASSWORD"] = adminPassword
		env["ME_CONFIG_SITE_COOKIESECRET"] = cookieSecret
		env["ME_CONFIG_SITE_SESSIONSECRET"] = sessionSecret
	}
	return env, nil
}

// writeEnvFile writes a docker compose env file (KEY=value per line)
func writeEnvFile(path string, env map[string]string) error {
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		if strings.ContainsAny(env[k], "\n\r") {
			return fmt.Errorf("invalid value for %s", k)
		}
		fmt.Fprintf(&b, "%s=%s\n", k, env[k])
	}
	return util.WriteFileAtomic(path, []byte(b.String()), 0o600)
}

// generateToolCompose writes docker-compose.yml for an admin tool instance
func generateToolCompose(instanceDir string, inst addon.Instance, versionConfig *addon.VersionConfig, hasEnv bool) error {
	internal := 0
	if len(versionConfig.Ports) > 0 {
		internal = versionConfig.Ports[0].Internal
	}
	if internal == 0 {
		return fmt.Errorf("addon definition %s has no port", inst.Type)
	}

	service := map[string]interface{}{
		"image":   versionConfig.Image,
		"restart": "unless-stopped",
		"networks": map[string]interface{}{
			"portico-network": map[string]interface{}{
				"aliases": []string{util.AddonPrefix + inst.Name},
			},
		},
		// Never published on a public interface: use an SSH tunnel or --domain
		"ports": []string{fmt.Sprintf("127.0.0.1:%d:%d", inst.Port, internal)},
	}
	if hasEnv {
		service["env_file"] = []string{"./secrets/tool.env"}
	}

	compose := map[string]interface{}{
		"name":     util.AddonPrefix + inst.Name,
		"services": map[string]interface{}{inst.Type: service},
		"networks": map[string]interface{}{
			"portico-network": map[string]interface{}{"external": true},
		},
	}
	data, err := yaml.Marshal(compose)
	if err != nil {
		return fmt.Errorf("error marshaling compose: %w", err)
	}
	return util.WriteFileAtomic(filepath.Join(instanceDir, "docker-compose.yml"), data, 0o644)
}

// renderToolCaddyfile returns the Caddy site block that publishes a tool
func renderToolCaddyfile(inst addon.Instance, internalPort int, passwordHash string) string {
	return fmt.Sprintf(`# Portico admin tool %[1]s (%[2]s)
# Generated by Portico - remove with: portico addons %[1]s delete

%[3]s {
	basic_auth {
		%[4]s %[5]s
	}

	reverse_proxy %[6]s:%[7]d

	header {
		X-Content-Type-Options nosniff
		X-Frame-Options DENY
		Referrer-Policy no-referrer
		-Server
	}
}
`, inst.Name, inst.Type, inst.Domain, toolAdminUser, passwordHash, util.AddonPrefix+inst.Name, internalPort)
}

// publishTool writes the proxy config of a tool and reloads Caddy
func publishTool(cfg *config.Config, inst addon.Instance, internalPort int, adminPassword string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(adminPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("error hashing password: %w", err)
	}
	path := toolCaddyfilePath(cfg, inst.Name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("error creating %s: %w", filepath.Dir(path), err)
	}
	_ = util.FixDirOwnership(filepath.Dir(path))
	if err := util.WriteFileAtomic(path, []byte(renderToolCaddyfile(inst, internalPort, string(hash))), 0o644); err != nil {
		return fmt.Errorf("error writing Caddyfile: %w", err)
	}
	_ = util.FixFileOwnership(path)

	if !proxyMountsAddons(cfg) {
		fmt.Println("⚠️  The reverse proxy does not load addon configs yet. Update it with:")
		fmt.Printf("     portico init && cd %s && docker compose up -d\n", cfg.ProxyDir)
	}
	pm := proxy.NewCaddyManager(cfg.ProxyDir, cfg.TemplatesDir)
	return pm.UpdateCaddyfile(cfg.AppsDir)
}

// unpublishTool removes the proxy config of a tool (if any) and reloads Caddy
func unpublishTool(cfg *config.Config, instanceName string) error {
	err := os.Remove(toolCaddyfilePath(cfg, instanceName))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	pm := proxy.NewCaddyManager(cfg.ProxyDir, cfg.TemplatesDir)
	return pm.UpdateCaddyfile(cfg.AppsDir)
}

// proxyMountsAddons reports whether the installed reverse-proxy compose file
// mounts the addon Caddyfiles directory (installs older than admin tools don't)
func proxyMountsAddons(cfg *config.Config) bool {
	data, err := os.ReadFile(filepath.Join(cfg.ProxyDir, "docker-compose.yml"))
	return err == nil && strings.Contains(string(data), "/etc/caddy/addons")
}

// createToolInstance sets up the files of a new admin tool instance and returns
// the admin password (shown once to the user)
func createToolInstance(cfg *config.Config, am *addon.Manager, instances map[string]addon.Instance, inst *addon.Instance, versionConfig *addon.VersionConfig) (string, error) {
	if inst.Domain != "" {
		if err := validateDomain(inst.Domain); err != nil {
			return "", err
		}
		for name, other := range instances {
			if other.Domain == inst.Domain {
				return "", fmt.Errorf("domain %s is already used by addon %s", inst.Domain, name)
			}
		}
	}
	target, err := resolveToolTarget(cfg, am, instances, inst.Type, inst.Target)
	if err != nil {
		return "", err
	}

	instanceDir := filepath.Join(cfg.AddonsDir, "instances", inst.Name)
	secretsDir := filepath.Join(instanceDir, "secrets")
	if err := os.MkdirAll(secretsDir, 0o700); err != nil {
		return "", fmt.Errorf("error creating instance directory: %w", err)
	}

	password, err := util.RandomSecret(16)
	if err != nil {
		return "", err
	}
	if err := util.WriteFileAtomic(filepath.Join(secretsDir, "admin_password"), []byte(password), 0o600); err != nil {
		return "", fmt.Errorf("error writing admin password: %w", err)
	}

	env, err := toolEnvironment(inst.Type, password, target)
	if err != nil {
		return "", err
	}
	if len(env) > 0 {
		if err := writeEnvFile(filepath.Join(secretsDir, "tool.env"), env); err != nil {
			return "", fmt.Errorf("error writing tool environment: %w", err)
		}
	}
	if err := generateToolCompose(instanceDir, *inst, versionConfig, len(env) > 0); err != nil {
		return "", err
	}
	return password, nil
}

// printToolAccess explains how to reach an admin tool
func printToolAccess(inst addon.Instance, password string) {
	fmt.Println()
	fmt.Printf("Start it with: portico addons %s up\n", inst.Name)
	fmt.Println()
	fmt.Println("Access:")
	fmt.Printf("  SSH tunnel: ssh -N -L %d:127.0.0.1:%d <admin-user>@<server>  →  http://localhost:%d\n", inst.Port, inst.Port, inst.Port)
	fmt.Println("              (git-only keys can't open tunnels: use a key added with 'portico ssh add --admin')")
	if inst.Domain != "" {
		fmt.Printf("  Web:        https://%s\n", inst.Domain)
	}
	if inst.Domain != "" || inst.Type == "mongo-express" {
		fmt.Printf("  User:       %s\n", toolAdminUser)
		fmt.Printf("  Password:   %s\n", password)
		fmt.Printf("              (stored in %s)\n", filepath.Join("addons", "instances", inst.Name, "secrets", "admin_password"))
	}
	if inst.Type == "adminer" {
		fmt.Println()
		fmt.Println("Log in to Adminer with the database credentials; the server is the instance alias,")
		fmt.Printf("e.g. %smy-postgres (see 'portico addons instances').\n", util.AddonPrefix)
	}
}
