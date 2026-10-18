package gateway

import (
	"path/filepath"
	"testing"
)

func TestValidateTektonRejectsMissingResources(t *testing.T) {
	root := t.TempDir()
	if err := write(filepath.Join(root, "kustomization.yaml"), []byte("resources: []\n")); err != nil {
		t.Fatal(err)
	}
	if err := ValidateTekton(root); err == nil {
		t.Fatal("accepted missing Kong resources")
	}
}

func TestLoadResourcesHandlesStructuredDefaultsAndOverlays(t *testing.T) {
	path := filepath.Join(t.TempDir(), "resources.yaml")
	content := "kind: Task\nspec:\n  params:\n    - name: revision\n      default: main\n    - name: overlays\n      value: [{key: source, expression: body.ref}]\n---\nkind: Pipeline\n"
	if err := write(path, []byte(content)); err != nil {
		t.Fatal(err)
	}
	resources, err := loadResources(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(resources) != 2 || resources[0].Spec.Params[0].Default != "main" {
		t.Fatal("lost YAML values")
	}
}
