package commands

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/maxvegac/portico/src/internal/app"
	"github.com/maxvegac/portico/src/internal/config"
	"github.com/maxvegac/portico/src/internal/docker"
	"github.com/maxvegac/portico/src/internal/proxy"
	"github.com/maxvegac/portico/src/internal/util"
)

// createAppSkeleton creates the app directories and an initial docker-compose.yml
// (no services yet). httpPort is the internal port Caddy will proxy to (0 = worker).
func createAppSkeleton(cfg *config.Config, am *app.Manager, appName string, httpPort int) (*app.App, error) {
	if err := util.ValidateNewAppName(appName); err != nil {
		return nil, err
	}
	// Addon instances created by older versions use their name as compose project
	if _, err := os.Stat(filepath.Join(cfg.AddonsDir, "instances", appName)); err == nil {
		return nil, fmt.Errorf("name %q is already used by an addon instance", appName)
	}
	if err := am.CreateAppDirectories(appName); err != nil {
		return nil, fmt.Errorf("error creating app directories: %w", err)
	}

	domain, err := app.DefaultDomain(appName)
	if err != nil {
		return nil, fmt.Errorf("%w (set one with 'portico set external-ip <ip>')", err)
	}

	a := &app.App{
		Name:     appName,
		Domain:   domain,
		Port:     httpPort,
		Services: []app.Service{},
	}

	appDir := filepath.Join(cfg.AppsDir, appName)
	dm := docker.NewManager(cfg.Registry.URL)
	if err := dm.GenerateDockerCompose(appDir, nil, a.Metadata()); err != nil {
		return nil, fmt.Errorf("error generating docker compose: %w", err)
	}
	return a, nil
}

// loadOrCreateApp loads an app, creating an empty one (HTTP enabled) if it doesn't exist yet
func loadOrCreateApp(cfg *config.Config, am *app.Manager, appName string) (*app.App, error) {
	if err := util.ValidateAppName(appName); err != nil {
		return nil, err
	}
	if am.Exists(appName) {
		return am.LoadApp(appName)
	}
	fmt.Printf("App %s not found. Creating app...\n", appName)
	return createAppSkeleton(cfg, am, appName, app.DefaultHTTPPort)
}

// applyApp saves the app (regenerating docker-compose.yml), starts its services and
// brings the reverse proxy in sync.
func applyApp(cfg *config.Config, am *app.Manager, a *app.App) error {
	lock, err := util.AcquireLock(filepath.Join(cfg.AppsDir, a.Name, ".portico.lock"))
	if err != nil {
		return err
	}
	defer lock.Release()

	if err := am.SaveApp(a); err != nil {
		return fmt.Errorf("error saving app: %w", err)
	}

	if len(a.Services) > 0 {
		appDir := filepath.Join(cfg.AppsDir, a.Name)
		dm := docker.NewManager(cfg.Registry.URL)
		if err := dm.DeployApp(appDir, a.DockerServices()); err != nil {
			return fmt.Errorf("error deploying app: %w", err)
		}
	}

	return syncProxy(cfg, am, a)
}

// syncProxy creates or removes the app Caddyfile depending on whether HTTP is
// enabled, then refreshes and reloads the main Caddy configuration.
func syncProxy(cfg *config.Config, am *app.Manager, a *app.App) error {
	caddyfilePath := filepath.Join(cfg.AppsDir, a.Name, "Caddyfile")
	if a.Port > 0 && len(a.Services) > 0 {
		if err := am.CreateDefaultCaddyfile(a.Name); err != nil {
			return fmt.Errorf("error creating Caddyfile: %w", err)
		}
	} else if err := os.Remove(caddyfilePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("error removing Caddyfile: %w", err)
	}

	pm := proxy.NewCaddyManager(cfg.ProxyDir, cfg.TemplatesDir)
	if err := pm.UpdateCaddyfile(cfg.AppsDir); err != nil {
		return fmt.Errorf("error updating proxy Caddyfile: %w", err)
	}
	return nil
}

// setServiceImage points the given service (or the HTTP service when serviceName
// is empty) at image, creating a "web" service if the app has none.
// Returns the name of the updated service and whether it was created.
func setServiceImage(a *app.App, serviceName, image string) (string, bool) {
	if serviceName == "" {
		serviceName = a.HTTPServiceName()
	}
	if serviceName == "" {
		serviceName = "web"
	}

	if svc := a.FindService(serviceName); svc != nil {
		svc.Image = image
		return serviceName, false
	}

	a.Services = append(a.Services, app.Service{
		Name:        serviceName,
		Image:       image,
		Environment: map[string]string{},
	})
	return serviceName, true
}

// buildImage runs docker build for a source directory
func buildImage(imageName, dockerfile, contextDir string, buildArgs []string) error {
	args := []string{"build", "-t", imageName, "-f", dockerfile}
	for _, arg := range buildArgs {
		args = append(args, "--build-arg", arg)
	}
	args = append(args, contextDir)

	cmd := exec.Command("docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// pullImage pulls an image, ignoring failures (the image may only exist locally)
func pullImage(image string) {
	fmt.Printf("Pulling image: %s\n", image)
	cmd := exec.Command("docker", "pull", image)
	if output, err := cmd.CombinedOutput(); err != nil {
		fmt.Printf("Warning: could not pull image (may be local): %v\n", err)
		if msg := strings.TrimSpace(string(output)); msg != "" {
			fmt.Println(msg)
		}
	}
}

// confirm asks a yes/no question on stdin (default: no)
func confirm(question string) bool {
	fmt.Print(question + " (y/N): ")
	reader := bufio.NewReader(os.Stdin)
	response, _ := reader.ReadString('\n')
	response = strings.TrimSpace(response)
	return strings.EqualFold(response, "y") || strings.EqualFold(response, "yes")
}

// appURL returns the public URL of an app
func appURL(a *app.App) string {
	if a.Port == 0 || a.Domain == "" {
		return ""
	}
	return "https://" + a.Domain
}
