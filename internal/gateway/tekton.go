package gateway

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"
)

type parameter struct {
	Name    string `yaml:"name"`
	Default any    `yaml:"default"`
	Value   any    `yaml:"value"`
}

type resource struct {
	Kind     string `yaml:"kind"`
	Metadata struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Resources []string `yaml:"resources"`
	Spec      struct {
		Params     []parameter `yaml:"params"`
		Workspaces []parameter `yaml:"workspaces"`
		Results    []parameter `yaml:"results"`
		Steps      []struct {
			Name   string `yaml:"name"`
			Script string `yaml:"script"`
		} `yaml:"steps"`
		Tasks []struct {
			Name    string `yaml:"name"`
			TaskRef struct {
				Name string `yaml:"name"`
			} `yaml:"taskRef"`
			Params     []parameter `yaml:"params"`
			Workspaces []parameter `yaml:"workspaces"`
		} `yaml:"tasks"`
		Triggers []struct {
			Template struct {
				Ref string `yaml:"ref"`
			} `yaml:"template"`
			Bindings     []parameter `yaml:"bindings"`
			Interceptors []struct {
				Params []parameter `yaml:"params"`
			} `yaml:"interceptors"`
		} `yaml:"triggers"`
	} `yaml:"spec"`
}

func loadResources(path string) ([]resource, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	decoder := yaml.NewDecoder(bytes.NewReader(content))
	var resources []resource
	for {
		var item resource
		err := decoder.Decode(&item)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		resources = append(resources, item)
	}
	return resources, nil
}

func containsParameter(parameters []parameter, name string) bool {
	for _, item := range parameters {
		if item.Name == name {
			return true
		}
	}
	return false
}

func ValidateTekton(root string) error {
	kustomization, err := loadResources(filepath.Join(root, "kustomization.yaml"))
	if err != nil {
		return err
	}
	if len(kustomization) != 1 {
		return fmt.Errorf("expected one kustomization")
	}
	tasks := map[string]resource{}
	var pipeline, listener, template resource
	for _, name := range kustomization[0].Resources {
		items, err := loadResources(filepath.Join(root, name))
		if err != nil {
			return err
		}
		for _, item := range items {
			switch item.Kind {
			case "Task":
				tasks[item.Metadata.Name] = item
			case "Pipeline":
				if item.Metadata.Name == "kong-publish" {
					pipeline = item
				}
			case "EventListener":
				if item.Metadata.Name == "kong" {
					listener = item
				}
			case "TriggerTemplate":
				if item.Metadata.Name == "kong-publish" {
					template = item
				}
			}
		}
	}
	if pipeline.Kind == "" || listener.Kind == "" || template.Kind == "" {
		return fmt.Errorf("missing Kong pipeline, listener or template")
	}
	for _, task := range pipeline.Spec.Tasks {
		definition, ok := tasks[task.TaskRef.Name]
		if !ok {
			return fmt.Errorf("missing task %s", task.TaskRef.Name)
		}
		for _, param := range definition.Spec.Params {
			if param.Default == nil && !containsParameter(task.Params, param.Name) {
				return fmt.Errorf("%s: missing parameter %s", task.Name, param.Name)
			}
		}
		for _, workspace := range definition.Spec.Workspaces {
			if !containsParameter(task.Workspaces, workspace.Name) {
				return fmt.Errorf("%s: missing workspace %s", task.Name, workspace.Name)
			}
		}
	}
	for name, task := range tasks {
		if !strings.HasPrefix(name, "kong-") {
			continue
		}
		for _, step := range task.Spec.Steps {
			if step.Script == "" {
				continue
			}
			command := exec.Command("sh", "-n")
			command.Stdin = strings.NewReader(step.Script)
			if output, err := command.CombinedOutput(); err != nil {
				return fmt.Errorf("%s/%s: %s: %w", name, step.Name, output, err)
			}
		}
	}
	if len(listener.Spec.Triggers) != 2 {
		return fmt.Errorf("expected two Kong triggers")
	}
	for _, trigger := range listener.Spec.Triggers {
		if trigger.Template.Ref != template.Metadata.Name {
			return fmt.Errorf("invalid trigger template reference")
		}
		for _, param := range template.Spec.Params {
			if !containsParameter(trigger.Bindings, param.Name) {
				return fmt.Errorf("missing trigger binding %s", param.Name)
			}
		}
		filter := ""
		for _, interceptor := range trigger.Interceptors {
			for _, param := range interceptor.Params {
				if param.Name == "filter" {
					filter, _ = param.Value.(string)
				}
			}
		}
		if !strings.Contains(filter, "refs/heads/main") || !strings.Contains(filter, "MacroData/api-gateway-config") {
			return fmt.Errorf("missing Kong trigger restrictions")
		}
	}
	publish := tasks["kong-update-gitops"]
	if len(publish.Spec.Steps) == 0 {
		return fmt.Errorf("missing GitOps steps")
	}
	script := publish.Spec.Steps[len(publish.Spec.Steps)-1].Script
	if !strings.Contains(script, "git rebase origin/main") || !strings.Contains(script, "git rev-parse origin/main") || strings.Contains(script, "--force") {
		return fmt.Errorf("missing safe GitOps publication checks")
	}
	if !containsParameter(tasks["kong-build"].Spec.Results, "digest") {
		return fmt.Errorf("missing image digest result")
	}
	return nil
}
