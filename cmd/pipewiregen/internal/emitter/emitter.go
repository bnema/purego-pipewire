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
	capiTmpl      *template.Template
	portTmpl      *template.Template
	adapterTmpl   *template.Template
	compositeTmpl *template.Template
)

func init() {
	capiTmpl = template.Must(template.New("capi").Funcs(template.FuncMap{
		"title": titleCase,
	}).Parse(capiTemplate))

	portTmpl = template.Must(template.New("port").Funcs(template.FuncMap{
		"title": titleCase,
	}).Parse(portTemplate))

	adapterTmpl = template.Must(template.New("adapter").Funcs(template.FuncMap{
		"title": titleCase,
	}).Parse(adapterTemplate))

	compositeTmpl = template.Must(template.New("composite").Funcs(template.FuncMap{
		"title": titleCase,
	}).Parse(compositeTemplate))
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

		capiContent, err := renderCAPI(group, m.Symbols, m.Libraries, m.Callbacks, m.EventStructs)
		if err != nil {
			return nil, fmt.Errorf("render capi for group %q: %w", group.Name, err)
		}
		out[capiPath] = capiContent

		portContent, err := renderPort(group, m.Symbols, m.EventStructs)
		if err != nil {
			return nil, fmt.Errorf("render port for group %q: %w", group.Name, err)
		}
		out[portPath] = portContent
	}

	// Generate adapters file
	adapterContent, err := renderAdapters(m)
	if err != nil {
		return nil, fmt.Errorf("render adapters: %w", err)
	}
	out[filepath.Join("internal", "capi", "adapters_gen.go")] = adapterContent

	// Generate composite port file
	compositeContent, err := renderComposite(m)
	if err != nil {
		return nil, fmt.Errorf("render composite: %w", err)
	}
	out[filepath.Join("internal", "ports", "out", "capi_gen.go")] = compositeContent

	return out, writeAll(root, out)
}

// renderCAPI generates the raw C API function types and registration helpers.
func renderCAPI(group model.Group, symbols []model.Symbol, libraries []model.Library, callbacks []model.Callback, eventStructs []model.EventStruct) ([]byte, error) {
	// Find symbols belonging to this group
	groupSymbols := filterSymbolsByGroup(symbols, group.Name)

	// Build library map for lookup
	libMap := make(map[string]string)
	for _, lib := range libraries {
		libMap[lib.Name] = lib.SOName
	}

	// Check if any symbol needs unsafe import
	needsUnsafe := false
	for _, sym := range groupSymbols {
		if strings.Contains(sym.Signature, "unsafe.Pointer") {
			needsUnsafe = true
			break
		}
	}

	// Filter callbacks and event structs for this group
	var groupCallbacks []model.Callback
	var groupEventStructs []model.EventStruct
	for _, cb := range callbacks {
		if cb.Group == group.Name {
			groupCallbacks = append(groupCallbacks, cb)
		}
	}
	for _, es := range eventStructs {
		if es.Group == group.Name {
			groupEventStructs = append(groupEventStructs, es)
		}
	}
	// Check if callbacks need unsafe
	for _, cb := range groupCallbacks {
		if strings.Contains(cb.Signature, "unsafe.Pointer") {
			needsUnsafe = true
			break
		}
	}

	data := struct {
		Group        model.Group
		Symbols      []model.Symbol
		LibraryMap   map[string]string
		NeedsUnsafe  bool
		Callbacks    []model.Callback
		EventStructs []model.EventStruct
		HasCallbacks bool
	}{
		Group:        group,
		Symbols:      groupSymbols,
		LibraryMap:   libMap,
		NeedsUnsafe:  needsUnsafe,
		Callbacks:    groupCallbacks,
		EventStructs: groupEventStructs,
		HasCallbacks: len(groupCallbacks) > 0,
	}

	var buf bytes.Buffer
	if err := capiTmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("execute capi template for group %q: %w", group.Name, err)
	}
	return buf.Bytes(), nil
}

// renderPort generates the outbound interface definitions.
func renderPort(group model.Group, symbols []model.Symbol, eventStructs []model.EventStruct) ([]byte, error) {
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
	needsUnsafe := false
	for _, sym := range groupSymbols {
		params, results := parseSignature(sym.Signature)
		methods = append(methods, Method{
			Name:    sym.Name,
			GoName:  toGoName(sym.Name),
			Params:  params,
			Results: results,
			CName:   sym.Name,
		})
		// Check if signature uses unsafe.Pointer
		if strings.Contains(sym.Signature, "unsafe.Pointer") {
			needsUnsafe = true
		}
	}

	// Filter event structs for this group
	var groupEventStructs []model.EventStruct
	for _, es := range eventStructs {
		if es.Group == group.Name {
			groupEventStructs = append(groupEventStructs, es)
		}
	}

	data := struct {
		Group        model.Group
		Methods      []Method
		NeedsUnsafe  bool
		EventStructs []model.EventStruct
	}{
		Group:        group,
		Methods:      methods,
		NeedsUnsafe:  needsUnsafe,
		EventStructs: groupEventStructs,
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

// extractParamNames parses a typed parameter list like "(argc *int32, argv ***byte)"
// and returns a call-site argument list like "(argc, argv)".
// It strips all type information, keeping only the parameter names.
// It uses depth-aware comma splitting to correctly handle nested function-type
// parameters like "callback func(int, int)".
func extractParamNames(params string) string {
	inner := strings.Trim(params, "()")
	if inner == "" {
		return "()"
	}

	// Split on commas at depth 0 only (respecting nested parens)
	var segments []string
	depth := 0
	start := 0
	for i, ch := range inner {
		if ch == '(' {
			depth++
		} else if ch == ')' {
			depth--
		} else if ch == ',' && depth == 0 {
			segments = append(segments, strings.TrimSpace(inner[start:i]))
			start = i + 1
		}
	}
	segments = append(segments, strings.TrimSpace(inner[start:]))

	names := make([]string, 0, len(segments))
	for _, seg := range segments {
		// Each segment is "name type" — the first space at depth 0 separates name from type.
		idx := strings.IndexByte(seg, ' ')
		if idx < 0 {
			names = append(names, seg)
		} else {
			names = append(names, seg[:idx])
		}
	}
	return "(" + strings.Join(names, ", ") + ")"
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

// toCamelCase converts a snake_case name like stream_playback to StreamPlayback.
func toCamelCase(s string) string {
	parts := strings.Split(s, "_")
	for i, p := range parts {
		parts[i] = titleCase(p)
	}
	return strings.Join(parts, "")
}

// renderAdapters generates one thin adapter struct per group with forwarding methods.
func renderAdapters(m *model.Model) ([]byte, error) {
	type AdapterMethod struct {
		CName    string
		GoName   string
		Params   string
		CallArgs string
		Results  string
	}

	type AdapterGroup struct {
		AdapterName string
		Methods     []AdapterMethod
	}

	var groups []AdapterGroup
	needsUnsafe := false
	for _, group := range m.Groups {
		groupSyms := filterSymbolsByGroup(m.Symbols, group.Name)
		methods := make([]AdapterMethod, 0, len(groupSyms))

		for _, sym := range groupSyms {
			params, results := parseSignature(sym.Signature)
			methods = append(methods, AdapterMethod{
				CName:    sym.Name,
				GoName:   toGoName(sym.Name),
				Params:   params,
				CallArgs: extractParamNames(params),
				Results:  results,
			})
			if strings.Contains(sym.Signature, "unsafe.Pointer") {
				needsUnsafe = true
			}
		}

		groups = append(groups, AdapterGroup{
			AdapterName: toCamelCase(group.Name) + "CAPIAdapter",
			Methods:     methods,
		})
	}

	data := struct {
		Groups      []AdapterGroup
		NeedsUnsafe bool
	}{
		Groups:      groups,
		NeedsUnsafe: needsUnsafe,
	}

	var buf bytes.Buffer
	if err := adapterTmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("execute adapter template: %w", err)
	}
	return buf.Bytes(), nil
}

// renderComposite generates the composite CAPI interface that embeds all group interfaces.
func renderComposite(m *model.Model) ([]byte, error) {
	data := struct {
		Groups []model.Group
	}{
		Groups: m.Groups,
	}

	var buf bytes.Buffer
	if err := compositeTmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("execute composite template: %w", err)
	}
	return buf.Bytes(), nil
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
{{if .NeedsUnsafe}}
import "unsafe"
{{end}}
{{range .Symbols}}
// {{.Name}}Func is the function type for {{.Name}}.
type {{.Name}}Func {{.Signature}}
{{end}}
{{if .HasCallbacks}}
{{range .Callbacks}}
// {{.Name}} represents the callback struct for {{.Name}}.
type {{.Name}} {{.Signature}}
{{end}}
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
{{if .NeedsUnsafe}}
import "unsafe"
{{end}}
// {{.Group.Interface}} defines the outbound interface for {{.Group.Name}} operations.
type {{.Group.Interface}} interface {
{{- range .Methods}}
	{{.GoName}}{{.Params}}{{if .Results}} {{.Results}}{{end}}
{{- end}}
}
`

const adapterTemplate = `// Code generated by pipewiregen. DO NOT EDIT.

package capi
{{if .NeedsUnsafe}}
import "unsafe"
{{end}}
{{range $group := .Groups}}
// {{$group.AdapterName}} forwards calls to the generated CAPI variables.
type {{$group.AdapterName}} struct{}
{{range $group.Methods}}
func (a *{{$group.AdapterName}}) {{.GoName}}{{.Params}}{{if .Results}} {{.Results}}{{end}} {
{{- if .Results}}
	return {{.CName}}{{.CallArgs}}
{{- else}}
	{{.CName}}{{.CallArgs}}
{{- end}}
}
{{end}}
{{end}}
`

const compositeTemplate = `// Code generated by pipewiregen. DO NOT EDIT.

package out

// CAPI is the composite interface that embeds all generated group interfaces.
type CAPI interface {
{{- range .Groups}}
	{{.Interface}}
{{- end}}
}
`
