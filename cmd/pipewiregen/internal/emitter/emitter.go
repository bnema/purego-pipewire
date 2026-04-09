// Package emitter renders generated Go source files from the binding model.
package emitter

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"unicode"

	"github.com/bnema/purego-pipewire/cmd/pipewiregen/internal/model"
)

var (
	capiTmpl *template.Template
	portTmpl *template.Template
)

func init() {
	capiTmpl = template.Must(template.New("capi").Funcs(template.FuncMap{
		"title": titleCase,
	}).Parse(capiTemplate))

	portTmpl = template.Must(template.New("port").Funcs(template.FuncMap{
		"title": titleCase,
	}).Parse(portTemplate))
}

// titleCase converts a string to title case (first letter uppercase, rest lowercase).
// Replaces deprecated strings.Title.
func titleCase(s string) string {
	if s == "" {
		return ""
	}
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	for i := 1; i < len(runes); i++ {
		runes[i] = unicode.ToLower(runes[i])
	}
	return string(runes)
}

// Emit generates Go source files from the model and writes them to the root directory.
// It returns a map of generated file paths to their content.
func Emit(m *model.Model, root string) (map[string][]byte, error) {
	out := make(map[string][]byte)
	for _, group := range m.Groups {
		capiPath := filepath.Join("internal", "capi", group.Name+"_gen.go")
		portPath := filepath.Join("internal", "ports", "out", group.Name+"_gen.go")

		capiContent, err := renderCAPI(group, m.Symbols, m.Libraries)
		if err != nil {
			return nil, fmt.Errorf("render capi for group %q: %w", group.Name, err)
		}
		out[capiPath] = capiContent

		portContent, err := renderPort(group, m.Symbols)
		if err != nil {
			return nil, fmt.Errorf("render port for group %q: %w", group.Name, err)
		}
		out[portPath] = portContent
	}
	return out, writeAll(root, out)
}

// renderCAPI generates the raw C API function types and registration helpers.
func renderCAPI(group model.Group, symbols []model.Symbol, libraries []model.Library) ([]byte, error) {
	// Find symbols belonging to this group
	groupSymbols := filterSymbolsByGroup(symbols, group.Name)

	// Build library map for lookup
	libMap := make(map[string]string)
	for _, lib := range libraries {
		libMap[lib.Name] = lib.SOName
	}

	data := struct {
		Group      model.Group
		Symbols    []model.Symbol
		LibraryMap map[string]string
	}{
		Group:      group,
		Symbols:    groupSymbols,
		LibraryMap: libMap,
	}

	var buf bytes.Buffer
	if err := capiTmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("execute capi template for group %q: %w", group.Name, err)
	}
	return buf.Bytes(), nil
}

// renderPort generates the outbound interface definitions.
func renderPort(group model.Group, symbols []model.Symbol) ([]byte, error) {
	groupSymbols := filterSymbolsByGroup(symbols, group.Name)

	// Convert symbols to methods with proper Go names
	type Method struct {
		Name    string
		GoName  string
		Params  string
		Results string
		CName   string
	}

	methods := make([]Method, 0, len(groupSymbols))
	for _, sym := range groupSymbols {
		params, results := parseSignature(sym.Signature)
		methods = append(methods, Method{
			Name:    sym.Name,
			GoName:  toGoName(sym.Name),
			Params:  params,
			Results: results,
			CName:   sym.Name,
		})
	}

	data := struct {
		Group   model.Group
		Methods []Method
	}{
		Group:   group,
		Methods: methods,
	}

	var buf bytes.Buffer
	if err := portTmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("execute port template for group %q: %w", group.Name, err)
	}
	return buf.Bytes(), nil
}

// parseSignature splits a func signature like "func(argc *int32, argv ***byte)" into params and results.
// Returns ("(argc *int32, argv ***byte)", "") for func with no returns,
// or ("(x int)", "(int, error)") for func with returns.
func parseSignature(sig string) (params, results string) {
	// Remove "func" prefix
	if !strings.HasPrefix(sig, "func") {
		return "()", ""
	}
	body := strings.TrimSpace(sig[4:])

	// Find the closing paren of params (handles nested parens)
	depth := 0
	paramsEnd := -1
	for i, ch := range body {
		if ch == '(' {
			depth++
		} else if ch == ')' {
			depth--
			if depth == 0 {
				paramsEnd = i
				break
			}
		}
	}
	if paramsEnd < 0 {
		return "()", ""
	}

	params = body[:paramsEnd+1]
	rest := strings.TrimSpace(body[paramsEnd+1:])

	// Check for return values
	if rest != "" {
		// Returns could be "error" or "(int, error)" or "int"
		results = rest
	}

	return params, results
}

// filterSymbolsByGroup returns symbols that belong to the given group.
func filterSymbolsByGroup(symbols []model.Symbol, groupName string) []model.Symbol {
	var result []model.Symbol
	for _, sym := range symbols {
		if sym.Group == groupName {
			result = append(result, sym)
		}
	}
	return result
}

// toGoName converts a C function name like pw_init to a Go name like PWInit.
func toGoName(cName string) string {
	// Handle pw_ prefix specially
	if strings.HasPrefix(cName, "pw_") {
		name := cName[3:]
		parts := strings.Split(name, "_")
		for i, p := range parts {
			parts[i] = titleCase(p)
		}
		return "PW" + strings.Join(parts, "")
	}
	// Generic conversion
	parts := strings.Split(cName, "_")
	for i, p := range parts {
		parts[i] = titleCase(p)
	}
	return strings.Join(parts, "")
}

// writeAll writes all generated files to disk.
func writeAll(root string, files map[string][]byte) error {
	for path, content := range files {
		fullPath := filepath.Join(root, path)
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("mkdir %s: %w", dir, err)
		}
		if err := os.WriteFile(fullPath, content, 0644); err != nil {
			return fmt.Errorf("write %s: %w", fullPath, err)
		}
	}
	return nil
}

const capiTemplate = `// Code generated by pipewiregen. DO NOT EDIT.

package capi

import "github.com/ebitengine/purego"
{{range .Symbols}}
// {{.Name}}Func is the function type for {{.Name}}.
type {{.Name}}Func {{.Signature}}
{{end}}

var (
{{range .Symbols}}
	{{.Name}} {{.Name}}Func
{{end}}
)

func register{{.Group.Name | title}}(handle uintptr) {
{{range .Symbols}}
	purego.RegisterLibFunc(&{{.Name}}, handle, "{{.Name}}")
{{end}}
}
`

const portTemplate = `// Code generated by pipewiregen. DO NOT EDIT.

package out

// {{.Group.Interface}} defines the outbound interface for {{.Group.Name}} operations.
type {{.Group.Interface}} interface {
{{- range .Methods}}
	{{.GoName}}{{.Params}}{{if .Results}} {{.Results}}{{end}}
{{- end}}
}
`
