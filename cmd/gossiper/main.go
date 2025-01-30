package main

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
	"os/exec"
	"strings"
	"text/template"
)

// USAGE:
// external run ```gossiper gen -m some```
// test: ```go run cmd/gossiper/main.go gen -m some```

const svcTemplate = `package svc

type {{.ModuleName}}Svc struct {}

func New{{.ModuleName}}Service() *{{.ModuleName}}Svc {
	return &{{.ModuleName}}Svc{}
}`

const ctrlTemplate = `package ctrl

import (
	"{{.RootModuleName}}/internal/pkg/{{.moduleName}}/svc"
)

type {{.ModuleName}}Ctrl struct {
	{{.moduleName}}Svc *svc.{{.ModuleName}}Svc
}

func New{{.ModuleName}}Controller(svc *svc.{{.ModuleName}}Svc) *{{.ModuleName}}Ctrl {
	return &{{.ModuleName}}Ctrl{
		{{.moduleName}}Svc: svc,
	}
}`

const moduleTemplate = `package {{.moduleName}}

import (
	"{{.RootModuleName}}/internal/pkg/{{.moduleName}}/ctrl"
	"{{.RootModuleName}}/internal/pkg/{{.moduleName}}/svc"
)

type Module struct {
	name    string
	version string

	{{.ModuleName}}API *ctrl.{{.ModuleName}}Ctrl
}

func New() *Module {
	mod := &Module{
		name:    "{{.moduleName}}-module",
		version: "v1",
		{{.ModuleName}}API: ctrl.New{{.ModuleName}}Controller(
			svc.New{{.ModuleName}}Service(),
		),
	}
	return mod
}

// Initialize initializes the module.
func (m Module) Initialize() error {
	panic("Not implemented")
}

// Version returns the version of the module.
func (m Module) Version() string {
	return m.version
}

// Name returns the name of the module.
func (m Module) Name() string {
	return m.name
}
`

func NewGenerateCommand() *cobra.Command {
	var moduleName string

	cmd := &cobra.Command{
		Use:   "gen",
		Short: "Генерация модулей",
		RunE: func(cmd *cobra.Command, args []string) error {
			if moduleName == "" {
				return fmt.Errorf("module name is empty")
			}
			return generateModule(moduleName)
		},
	}
	cmd.Flags().StringVarP(&moduleName, "module", "m", "", "module name (required)")
	return cmd
}

func getModuleName() (string, error) {
	file, err := os.Open("go.mod")
	if err != nil {
		return "", fmt.Errorf("failed to open go.mod file: %w", err)
	}
	defer file.Close()

	var moduleName string
	_, err = fmt.Fscanf(file, "module %s", &moduleName)
	if err != nil {
		return "", fmt.Errorf("failed to read module name from go.mod: %w", err)
	}
	return moduleName, nil
}

func generateModule(moduleName string) error {
	rootModuleName, err := getModuleName()
	if err != nil {
		return fmt.Errorf("could not get module name: %w", err)
	}

	modulePath := fmt.Sprintf("internal/pkg/%s", strings.ToLower(moduleName))

	files := []struct {
		path     string
		template string
	}{
		{fmt.Sprintf("%s/svc/%s.svc.go", modulePath, strings.ToLower(moduleName)), svcTemplate},
		{fmt.Sprintf("%s/ctrl/%s.ctrl.go", modulePath, strings.ToLower(moduleName)), ctrlTemplate},
		{fmt.Sprintf("%s/%s.module.go", modulePath, strings.ToLower(moduleName)), moduleTemplate},
	}

	for _, file := range files {
		if err := writeTemplate(file.path, file.template, moduleName, rootModuleName); err != nil {
			return err
		}
		if err := gitAdd(file.path); err != nil {
			return err
		}
	}

	fmt.Println("Module generated and added to git!")
	return nil
}

func writeTemplate(path, tmpl, moduleName, rootModuleName string) error {
	if err := os.MkdirAll(getDir(path), os.ModePerm); err != nil {
		return err
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	t := template.Must(template.New("file").Parse(tmpl))
	return t.Execute(file, map[string]string{
		"ModuleName":     strings.Title(moduleName),
		"moduleName":     strings.ToLower(moduleName),
		"RootModuleName": rootModuleName,
	})
}

func gitAdd(filePath string) error {
	cmd := exec.Command("git", "add", filePath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to add file to git: %w", err)
	}
	return nil
}

func getDir(path string) string {
	parts := strings.Split(path, "/")
	return strings.Join(parts[:len(parts)-1], "/")
}

func main() {
	rootCmd := &cobra.Command{
		Use:   "gossiper",
		Short: "Gossiper CLI tool",
	}

	rootCmd.AddCommand(NewGenerateCommand()) // Импорт генератора
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
