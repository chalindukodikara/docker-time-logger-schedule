package main

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"gopkg.in/yaml.v3"
)

/* ---------- helpers --------------------------------------------------*/

// deepMerge copies src into dst (recursive)
func deepMerge(dst, src map[string]any) map[string]any {
	if dst == nil {
		dst = map[string]any{}
	}
	for k, v := range src {
		if sm, ok := v.(map[string]any); ok {
			if dm, ok := dst[k].(map[string]any); ok {
				dst[k] = deepMerge(dm, sm)
				continue
			}
		}
		dst[k] = v
	}
	return dst
}

// fetch nested value "a.b.c"
func getNested(m map[string]any, dotted string) (any, bool) {
	parts := strings.Split(dotted, ".")
	cur := m
	for i, p := range parts {
		if i == len(parts)-1 {
			v, ok := cur[p]
			return v, ok
		}
		nxt, ok := cur[p].(map[string]any)
		if !ok {
			return nil, false
		}
		cur = nxt
	}
	return nil, false
}

// set nested value (create maps along the way)
func setNested(m map[string]any, dotted string, v any) {
	parts := strings.Split(dotted, ".")
	cur := m
	for i, p := range parts {
		if i == len(parts)-1 {
			cur[p] = v
			return
		}
		if _, ok := cur[p]; !ok {
			cur[p] = map[string]any{}
		}
		cur = cur[p].(map[string]any)
	}
}

// read YAML into generic map
func readMap(path string) map[string]any {
	raw, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	out := map[string]any{}
	if err := yaml.Unmarshal(raw, &out); err != nil {
		panic(err)
	}
	return out
}

// extractTemplateDefaults extracts default values from template placeholders
func extractTemplateDefaults(templateContent string) map[string]any {
	defaults := map[string]any{}

	// Find patterns like '{{.param | default "value"}}' and '{{index . `param` | default "value"}}'
	dotDefaultRe := regexp.MustCompile(`'?\{\{\.([a-zA-Z_][a-zA-Z0-9_]*(?:\.[a-zA-Z_][a-zA-Z0-9_]*)*)\s*\|\s*default\s+"([^"]+)"\}\}'?`)
	indexDefaultRe := regexp.MustCompile(`'?\{\{index\s+\.\s+` + "`" + `([^` + "`" + `]+)` + "`" + `\s*\|\s*default\s+"([^"]+)"\}\}'?`)

	// Also find simple patterns without defaults for structure
	dotRe := regexp.MustCompile(`'?\{\{\.([a-zA-Z_][a-zA-Z0-9_]*(?:\.[a-zA-Z_][a-zA-Z0-9_]*)*)(?:\s*\|\s*default\s+"[^"]+")?\}\}'?`)
	indexRe := regexp.MustCompile(`'?\{\{index\s+\.\s+` + "`" + `([^` + "`" + `]+)` + "`" + `(?:\s*\|\s*default\s+"[^"]+")?\}\}'?`)

	// Extract dot notation with defaults
	dotDefaultMatches := dotDefaultRe.FindAllStringSubmatch(templateContent, -1)
	for _, match := range dotDefaultMatches {
		if len(match) > 2 {
			paramKey := match[1]
			defaultValue := match[2]
			setNested(defaults, paramKey, defaultValue)
		}
	}

	// Extract index notation with defaults
	indexDefaultMatches := indexDefaultRe.FindAllStringSubmatch(templateContent, -1)
	for _, match := range indexDefaultMatches {
		if len(match) > 2 {
			paramKey := match[1]
			defaultValue := match[2]
			defaults[paramKey] = defaultValue
		}
	}

	// Extract parameters without defaults to ensure structure exists
	paramKeys := make(map[string]bool)

	dotMatches := dotRe.FindAllStringSubmatch(templateContent, -1)
	for _, match := range dotMatches {
		if len(match) > 1 {
			paramKey := match[1]
			if strings.Contains(paramKey, ".") {
				parts := strings.Split(paramKey, ".")
				if len(parts) >= 2 {
					paramKeys[parts[0]] = true
				}
			} else {
				paramKeys[paramKey] = true
			}
		}
	}

	indexMatches := indexRe.FindAllStringSubmatch(templateContent, -1)
	for _, match := range indexMatches {
		if len(match) > 1 {
			paramKey := match[1]
			paramKeys[paramKey] = true
		}
	}

	// Ensure structure exists for parameters without explicit defaults
	for key := range paramKeys {
		if _, exists := defaults[key]; !exists {
			switch key {
			case "env":
				defaults[key] = map[string]any{}
			case "limits":
				defaults[key] = map[string]any{}
			case "buildArgs":
				defaults[key] = map[string]any{}
			default:
				defaults[key] = ""
			}
		}
	}

	return defaults
}

/* ---------- policy enforcement ---------------------------------------*/

func applyOverridePolicy(class map[string]any, buildParams map[string]any) map[string]any {
	spec, ok := class["spec"].(map[string]any)
	if !ok {
		return buildParams
	}

	pol, ok := spec["overridePolicy"].(map[string]any)
	if !ok {
		return buildParams
	}

	allow := true
	if v, ok := pol["allowParamOverride"].(bool); ok {
		allow = v
	}
	if !allow {
		return map[string]any{} // ignore all build overrides
	}

	// restricted
	restricted := map[string]struct{}{}
	if restrictedList, ok := pol["restrictedParams"].([]any); ok {
		for _, r := range restrictedList {
			restricted[fmt.Sprint(r)] = struct{}{}
		}
	}

	filtered := map[string]any{}
	var walk func(prefix string, src map[string]any, dst map[string]any)
	walk = func(prefix string, src, dst map[string]any) {
		for k, v := range src {
			key := k
			if prefix != "" {
				key = prefix + "." + k
			}
			if _, bad := restricted[key]; bad {
				continue // blocked
			}
			if vm, ok := v.(map[string]any); ok {
				inner := map[string]any{}
				walk(key, vm, inner)
				if len(inner) > 0 {
					dst[k] = inner
				}
			} else {
				dst[k] = v
			}
		}
	}
	walk("", buildParams, filtered)
	return filtered
}

func ensureRequired(class map[string]any, final map[string]any) {
	spec, ok := class["spec"].(map[string]any)
	if !ok {
		return
	}

	pol, ok := spec["overridePolicy"].(map[string]any)
	if !ok {
		return
	}

	if requiredList, ok := pol["requiredParams"].([]any); ok {
		for _, r := range requiredList {
			key := fmt.Sprint(r)
			if _, ok := getNested(final, key); !ok {
				panic(fmt.Sprintf("required parameter %q missing after merge", key))
			}
		}
	}
}

/* ---------- main -----------------------------------------------------*/
func main() {
	if len(os.Args) != 4 {
		fmt.Fprintln(os.Stderr, "usage: go run main.go build.yaml buildclass.yaml buildtemplate.yaml")
		os.Exit(1)
	}
	buildDoc := readMap(os.Args[1])
	classDoc := readMap(os.Args[2])
	tplBytes, _ := os.ReadFile(os.Args[3])

	// extract param maps - handle missing parameters gracefully
	var buildParams map[string]any
	if buildSpec, ok := buildDoc["spec"].(map[string]any); ok {
		if params, ok := buildSpec["parameters"].(map[string]any); ok {
			buildParams = params
		} else {
			buildParams = map[string]any{}
		}
	} else {
		buildParams = map[string]any{}
	}

	var classParams map[string]any
	if classSpec, ok := classDoc["spec"].(map[string]any); ok {
		if params, ok := classSpec["parameters"].(map[string]any); ok {
			classParams = params
		} else {
			classParams = map[string]any{}
		}
	} else {
		classParams = map[string]any{}
	}

	// Extract template defaults dynamically from the template content
	tplDefaults := extractTemplateDefaults(string(tplBytes))

	// enforce override policy
	buildFiltered := applyOverridePolicy(classDoc, buildParams)

	// merge: template <- class <- buildFiltered (build has highest priority)
	final := deepMerge(deepMerge(tplDefaults, classParams), buildFiltered)

	// ensure required present
	ensureRequired(classDoc, final)

	// --- render BuildTemplate ------------------------------------------
	funcs := sprig.TxtFuncMap()
	funcs["parameters"] = func() map[string]any { return final }

	tmpl, err := template.New("bt").
		Funcs(funcs).
		Option("missingkey=zero"). // Use zero value instead of error for missing keys
		Parse(string(tplBytes))
	if err != nil {
		panic(err)
	}
	var buf bytes.Buffer

	// Create template context with parameters accessible via {{.paramName}}
	templateData := map[string]any{
		"parameters": final,
	}

	// Also add all parameters at root level for direct access
	for k, v := range final {
		templateData[k] = v
	}

	if err := tmpl.Execute(&buf, templateData); err != nil {
		panic(err)
	}
	fmt.Print(buf.String())
}
