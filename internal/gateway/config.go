package gateway

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"
)

type Document struct {
	Name    string
	Content []byte
}

func Compose(documents []Document) (map[string]any, error) {
	services := []any{}
	serviceNames, routeNames := map[string]bool{}, map[string]bool{}
	for _, document := range documents {
		var data map[string]any
		decoder := yaml.NewDecoder(bytes.NewReader(document.Content))
		if err := decoder.Decode(&data); err != nil {
			return nil, fmt.Errorf("%s: %w", document.Name, err)
		}
		var extra any
		if err := decoder.Decode(&extra); err != io.EOF {
			return nil, fmt.Errorf("%s: expected one YAML document", document.Name)
		}
		items, ok := data["services"].([]any)
		if !ok || len(data) != 1 {
			return nil, fmt.Errorf("%s: expected only a services array", document.Name)
		}
		for _, item := range items {
			service, ok := item.(map[string]any)
			name, _ := service["name"].(string)
			host, _ := service["host"].(string)
			if !ok || name == "" || host == "" {
				return nil, fmt.Errorf("%s: each service requires name and host", document.Name)
			}
			if serviceNames[name] {
				return nil, fmt.Errorf("duplicate service %s", name)
			}
			serviceNames[name] = true
			if raw, exists := service["routes"]; exists {
				routes, ok := raw.([]any)
				if !ok {
					return nil, fmt.Errorf("%s: routes must be an array", name)
				}
				for _, item := range routes {
					route, _ := item.(map[string]any)
					routeName, _ := route["name"].(string)
					if routeName == "" {
						return nil, fmt.Errorf("%s: each route requires a name", name)
					}
					if routeNames[routeName] {
						return nil, fmt.Errorf("duplicate route %s", routeName)
					}
					routeNames[routeName] = true
				}
			}
			services = append(services, service)
		}
	}
	return map[string]any{"_format_version": "3.0", "services": services}, nil
}

func config(root, environment string) (map[string]any, error) {
	if environment != "hom" && environment != "prd" {
		return nil, fmt.Errorf("expected hom or prd")
	}
	directory := filepath.Join(root, "gateways/public-api", environment, "services")
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}
	documents := []Document{}
	for _, entry := range entries {
		if entry.IsDir() || (!strings.HasSuffix(entry.Name(), ".yaml") && !strings.HasSuffix(entry.Name(), ".yml")) {
			continue
		}
		content, err := os.ReadFile(filepath.Join(directory, entry.Name()))
		if err != nil {
			return nil, err
		}
		documents = append(documents, Document{entry.Name(), content})
	}
	return Compose(documents)
}

func encode(data any) ([]byte, error) {
	var output bytes.Buffer
	encoder := yaml.NewEncoder(&output)
	encoder.SetIndent(2)
	err := encoder.Encode(data)
	return output.Bytes(), err
}

func write(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, content, 0644)
}

func ImageContext(root, environment string) (string, error) {
	data, err := config(root, environment)
	if err != nil {
		return "", err
	}
	content, err := encode(data)
	if err != nil {
		return "", err
	}
	dockerfile, err := os.ReadFile(filepath.Join(root, "Dockerfile"))
	if err != nil {
		return "", err
	}
	context := filepath.Join(root, "dist", environment)
	if err := write(filepath.Join(context, "kong.yml"), content); err != nil {
		return "", err
	}
	if err := write(filepath.Join(context, "Dockerfile"), dockerfile); err != nil {
		return "", err
	}
	return context, nil
}

func Prepare(root, environment, gitops, changedPath, fingerprintPath string) error {
	context, err := ImageContext(root, environment)
	if err != nil {
		return err
	}
	hash := sha256.New()
	for _, name := range []string{"Dockerfile", "kong.yml"} {
		content, err := os.ReadFile(filepath.Join(context, name))
		if err != nil {
			return err
		}
		hash.Write(content)
	}
	fingerprint := fmt.Sprintf("%x", hash.Sum(nil))
	content, err := os.ReadFile(filepath.Join(gitops, "platforms/kong/environments", environment+".yaml"))
	if err != nil {
		return err
	}
	var previous struct {
		Publication struct {
			Fingerprint string `yaml:"fingerprint"`
		} `yaml:"publication"`
	}
	if err := yaml.Unmarshal(content, &previous); err != nil {
		return err
	}
	changed := "true"
	if previous.Publication.Fingerprint == fingerprint {
		changed = "false"
	}
	if err := write(fingerprintPath, []byte(fingerprint)); err != nil {
		return err
	}
	return write(changedPath, []byte(changed))
}
