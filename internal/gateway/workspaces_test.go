package gateway

import (
	"os"
	"path/filepath"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestKongSinglePersistentWorkspace(t *testing.T) {
	catalog := os.Getenv("TEKTON_CATALOG")
	if catalog == "" {
		t.Skip("set TEKTON_CATALOG to validate catalog workspace bindings")
	}
	content, err := os.ReadFile(filepath.Join(catalog, "pipelines/kong-publish.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var pipeline struct {
		Spec struct {
			Workspaces []struct {
				Name string `yaml:"name"`
			} `yaml:"workspaces"`
			Tasks []struct {
				Name       string `yaml:"name"`
				Workspaces []struct {
					Name      string `yaml:"name"`
					Workspace string `yaml:"workspace"`
					SubPath   string `yaml:"subPath"`
				} `yaml:"workspaces"`
			} `yaml:"tasks"`
		} `yaml:"spec"`
	}
	if err := yaml.Unmarshal(content, &pipeline); err != nil {
		t.Fatal(err)
	}
	if len(pipeline.Spec.Workspaces) != 2 || pipeline.Spec.Workspaces[0].Name != "data" || pipeline.Spec.Workspaces[1].Name != "dockerconfig" {
		t.Fatal("expected shared data workspace and registry secret")
	}
	for _, task := range pipeline.Spec.Tasks {
		for _, binding := range task.Workspaces {
			if binding.Name == "dockerconfig" {
				continue
			}
			expected := binding.Name
			if binding.Name == "output" {
				expected = "source"
				if task.Name == "clone-gitops" {
					expected = "gitops"
				}
			}
			if binding.Workspace != "data" || binding.SubPath != expected {
				t.Fatalf("%s/%s: expected data workspace with subPath %s", task.Name, binding.Name, expected)
			}
		}
	}
	resources, err := loadResources(filepath.Join(catalog, "triggers/kong-publish.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(resources) != 2 {
		t.Fatal("expected template and listener")
	}
	var template struct {
		Spec struct {
			ResourceTemplates []struct {
				Spec struct {
					Workspaces []struct {
						Name                string `yaml:"name"`
						VolumeClaimTemplate any    `yaml:"volumeClaimTemplate"`
					} `yaml:"workspaces"`
				} `yaml:"spec"`
			} `yaml:"resourcetemplates"`
		} `yaml:"spec"`
	}
	content, err = os.ReadFile(filepath.Join(catalog, "triggers/kong-publish.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal(content, &template); err != nil {
		t.Fatal(err)
	}
	if len(template.Spec.ResourceTemplates) != 1 {
		t.Fatal("expected one PipelineRun template")
	}
	claims := 0
	for _, workspace := range template.Spec.ResourceTemplates[0].Spec.Workspaces {
		if workspace.VolumeClaimTemplate != nil {
			claims++
			if workspace.Name != "data" {
				t.Fatal("PVC must bind shared data workspace")
			}
		}
	}
	if claims != 1 {
		t.Fatalf("expected one PVC, got %d", claims)
	}
}
