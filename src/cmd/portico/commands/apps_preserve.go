package commands

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/maxvegac/portico/src/internal/config"
	"github.com/maxvegac/portico/src/internal/docker"
)

// NewAppsPreserveCmd creates the apps preserve command
func NewAppsPreserveCmd() *cobra.Command {
	var file string

	cmd := &cobra.Command{
		Use:   "preserve [app-name]",
		Short: "Preserve manual changes to files",
		Long:  "Mark files as preserved to prevent Portico from overwriting manual changes. Updates the hash so Portico recognizes your changes as intentional.",
		Args:  cobra.ExactArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			appName := args[0]

			cfg, err := config.LoadConfig()
			if err != nil {
				fmt.Printf("Error loading config: %v\n", err)
				return
			}

			appDir := filepath.Join(cfg.AppsDir, appName)
			dm := docker.NewManager(cfg.Registry.URL)

			// If specific file requested
			if file != "" {
				filePath := filepath.Join(appDir, file)
				fmt.Printf("Preserving file: %s\n", filePath)
				// For now, we'll focus on docker-compose.yml
				// Future: could extend to preserve other files like Caddyfile
				if file != "docker-compose.yml" {
					fmt.Printf("Currently only docker-compose.yml can be preserved\n")
					return
				}
			}

			// Load current compose file
			compose, err := dm.LoadComposeFile(appDir)
			if err != nil {
				fmt.Printf("Error loading docker-compose.yml: %v\n", err)
				return
			}

			// Calculate current hash (without the hash field itself)
			composeFile := filepath.Join(appDir, "docker-compose.yml")
			currentData, err := os.ReadFile(composeFile)
			if err != nil {
				fmt.Printf("Error reading docker-compose.yml: %v\n", err)
				return
			}

			// Parse to get current content
			var currentCompose struct {
				Services map[string]interface{}  `yaml:"services"`
				Networks map[string]interface{}  `yaml:"networks,omitempty"`
				Secrets  map[string]interface{}  `yaml:"secrets,omitempty"`
				XPortico *docker.PorticoMetadata `yaml:"x-portico,omitempty"`
			}
			if err := yaml.Unmarshal(currentData, &currentCompose); err != nil {
				fmt.Printf("Error parsing docker-compose.yml: %v\n", err)
				return
			}

			// Remove hash for calculation
			if currentCompose.XPortico != nil {
				currentCompose.XPortico.Generated = ""
			}

			// Calculate new hash
			dataWithoutHash, err := yaml.Marshal(&currentCompose)
			if err != nil {
				fmt.Printf("Error marshaling: %v\n", err)
				return
			}

			hash := sha256.Sum256(dataWithoutHash)
			hashStr := fmt.Sprintf("%x", hash)

			// Update metadata with new hash
			if compose.XPortico == nil {
				compose.XPortico = &docker.PorticoMetadata{}
			}
			compose.XPortico.Generated = hashStr

			// Save updated compose file
			// We need to preserve all existing fields
			composeToSave := &docker.ComposeFile{
				Services: compose.Services,
				Networks: compose.Networks,
				Secrets:  compose.Secrets,
				XPortico: compose.XPortico,
			}

			data, err := yaml.Marshal(composeToSave)
			if err != nil {
				fmt.Printf("Error marshaling updated compose: %v\n", err)
				return
			}

			if err := os.WriteFile(composeFile, data, 0o644); err != nil {
				fmt.Printf("Error saving docker-compose.yml: %v\n", err)
				return
			}

			fmt.Printf("Manual changes to docker-compose.yml have been preserved.\n")
			fmt.Printf("Portico will maintain your customizations in future regenerations.\n")
		},
	}

	cmd.Flags().StringVar(&file, "file", "docker-compose.yml", "File to preserve (default: docker-compose.yml)")
	return cmd
}
