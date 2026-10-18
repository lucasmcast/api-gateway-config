package gateway

import (
	"os"
	"path/filepath"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestCompose(t *testing.T) {
	valid := "services:\n  - name: payment\n    host: payment\n    plugins:\n      - name: rate-limiting\n        config:\n          minute: 10\n    routes:\n      - name: create\n        paths: [/create]\n"
	config, err := Compose([]Document{{"payment.yaml", []byte(valid)}})
	if err != nil {
		t.Fatal(err)
	}
	services := config["services"].([]any)
	if len(services) != 1 || services[0].(map[string]any)["plugins"] == nil {
		t.Fatal("lost service fields")
	}
	for _, content := range []string{"services: {}", "services: []\nother: true", "services: []\nservices: []", "services: []\n---\nservices: []", "services: [null]", "services: [{name: payment}]", "services: [{name: p, host: p, routes: {}}]", "services: [{name: p, host: p, routes: [null]}]"} {
		if _, err := Compose([]Document{{"invalid", []byte(content)}}); err == nil {
			t.Errorf("accepted %s", content)
		}
	}
	if _, err := Compose([]Document{{"a", []byte(valid)}, {"b", []byte(valid)}}); err == nil {
		t.Fatal("accepted duplicate service")
	}
	duplicateRoutes := "services: [{name: one, host: one, routes: [{name: create}]}, {name: two, host: two, routes: [{name: create}]}]"
	if _, err := Compose([]Document{{"routes", []byte(duplicateRoutes)}}); err == nil {
		t.Fatal("accepted duplicate route")
	}
}

func TestImageContextAndPrepare(t *testing.T) {
	root := t.TempDir()
	for _, environment := range []string{"hom", "prd"} {
		content := "services: []\n"
		if environment == "hom" {
			content = "services: [{name: payment, host: payment}]\n"
		}
		if err := write(filepath.Join(root, "gateways/public-api", environment, "services/payment.yaml"), []byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := write(filepath.Join(root, "Dockerfile"), []byte("FROM kong:3.9.4\n")); err != nil {
		t.Fatal(err)
	}
	context, err := ImageContext(root, "prd")
	if err != nil {
		t.Fatal(err)
	}
	content, _ := os.ReadFile(filepath.Join(context, "kong.yml"))
	var config map[string]any
	if err := yaml.Unmarshal(content, &config); err != nil {
		t.Fatal(err)
	}
	if config["declarativeConfig"] != nil || len(config["services"].([]any)) != 0 {
		t.Fatal("incorrect production configuration")
	}
	if _, err := ImageContext(root, "invalid"); err == nil {
		t.Fatal("accepted invalid environment")
	}
	gitops := t.TempDir()
	valuesPath := filepath.Join(gitops, "platforms/kong/environments/hom.yaml")
	if err := write(valuesPath, []byte("{}\n")); err != nil {
		t.Fatal(err)
	}
	changed, fingerprint := filepath.Join(root, "changed"), filepath.Join(root, "fingerprint")
	if err := Prepare(root, "hom", gitops, changed, fingerprint); err != nil {
		t.Fatal(err)
	}
	result, _ := os.ReadFile(changed)
	if string(result) != "true" {
		t.Fatal("expected first publication")
	}
	hash, _ := os.ReadFile(fingerprint)
	if len(hash) != 64 {
		t.Fatal("invalid fingerprint")
	}
	if err := write(valuesPath, []byte("publication:\n  fingerprint: "+string(hash)+"\n")); err != nil {
		t.Fatal(err)
	}
	if err := Prepare(root, "hom", gitops, changed, fingerprint); err != nil {
		t.Fatal(err)
	}
	result, _ = os.ReadFile(changed)
	if string(result) != "false" {
		t.Fatal("rebuilt unchanged configuration")
	}
}
