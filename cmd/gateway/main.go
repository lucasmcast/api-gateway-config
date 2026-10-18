package main

import (
	"fmt"
	"os"

	"macrodata.dev/api-gateway-config/internal/gateway"
)

func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: gateway image-context <hom|prd>|prepare <hom|prd>|validate-tekton <catalog-path>")
	}
	switch args[0] {
	case "validate-tekton":
		if len(args) != 2 {
			return fmt.Errorf("validate-tekton requires catalog path")
		}
		return gateway.ValidateTekton(args[1])
	case "image-context":
		if len(args) != 2 {
			return fmt.Errorf("image-context requires environment")
		}
		path, err := gateway.ImageContext(".", args[1])
		if err == nil {
			fmt.Println(path)
		}
		return err
	case "prepare":
		if len(args) != 2 {
			return fmt.Errorf("prepare requires environment")
		}
		for _, name := range []string{"GITOPS", "RESULT_CHANGED", "RESULT_FINGERPRINT"} {
			if os.Getenv(name) == "" {
				return fmt.Errorf("%s is required", name)
			}
		}
		return gateway.Prepare(".", args[1], os.Getenv("GITOPS"), os.Getenv("RESULT_CHANGED"), os.Getenv("RESULT_FINGERPRINT"))
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
