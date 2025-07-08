// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"gopkg.in/yaml.v3"
)

// readYAMLFile parses a YAML file into a map.
func readYAMLFile(path string) (map[string]interface{}, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %q: %w", path, err)
	}
	var out map[string]interface{}
	fmt.Println("--- Raw content of", path, "---")
	fmt.Println(string(raw))
	fmt.Println("--- End of raw content ---")
	if err := yaml.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML from %q: %w", path, err)
	}
	return out, nil
}

// extractParameters follows a path of nested keys to extract a sub-map.
func extractParameters(m map[string]interface{}, path ...string) map[string]interface{} {
	for _, key := range path {
		if m == nil {
			return make(map[string]interface{})
		}
		val, ok := m[key]
		if !ok {
			return make(map[string]interface{})
		}
		m, ok = val.(map[string]interface{})
		if !ok {
			return make(map[string]interface{})
		}
	}
	return m
}

// deepMerge recursively merges multiple maps.
// The right-most map has the highest priority.
func deepMerge(maps ...map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for _, m := range maps {
		for k, v := range m {
			if v, ok := v.(map[string]interface{}); ok {
				if dst, ok := result[k].(map[string]interface{}); ok {
					result[k] = deepMerge(dst, v)
				} else {
					result[k] = deepMerge(v)
				}
			} else {
				result[k] = v
			}
		}
	}
	return result
}

// getNestedValue retrieves a value from a nested map using a dot-separated key.
func getNestedValue(m map[string]interface{}, dottedKey string) (interface{}, bool) {
	parts := strings.Split(dottedKey, ".")
	for i, p := range parts {
		if i == len(parts)-1 {
			v, ok := m[p]
			return v, ok
		}
		next, ok := m[p].(map[string]interface{})
		if !ok {
			return nil, false
		}
		m = next
	}
	return nil, false
}

// applyOverridePolicy filters the build parameters based on the override policy defined in the class.
func applyOverridePolicy(class, buildParams map[string]interface{}) (map[string]interface{}, error) {
	spec := extractParameters(class, "spec")
	policy := extractParameters(spec, "overridePolicy")

	allow, ok := policy["allowParamOverride"].(bool)
	if ok && !allow {
		return make(map[string]interface{}), nil // Overrides are disabled
	}

	restrictedParams, ok := policy["restrictedParams"].([]interface{})
	if !ok {
		return buildParams, nil // No restrictions
	}

	restricted := make(map[string]struct{})
	for _, r := range restrictedParams {
		restricted[fmt.Sprint(r)] = struct{}{}
	}

	filtered := make(map[string]interface{})
	var walk func(prefix string, src, dst map[string]interface{})
	walk = func(prefix string, src, dst map[string]interface{}) {
		for k, v := range src {
			key := k
			if prefix != "" {
				key = prefix + "." + k
			}
			if _, isRestricted := restricted[key]; isRestricted {
				continue
			}
			if vm, ok := v.(map[string]interface{}); ok {
				nested := make(map[string]interface{})
				walk(key, vm, nested)
				if len(nested) > 0 {
					dst[k] = nested
				}
			} else {
				dst[k] = v
			}
		}
	}
	walk("", buildParams, filtered)
	return filtered, nil
}

// ensureRequiredParameters checks if all required parameters are present in the final merged map.
func ensureRequiredParameters(class, merged map[string]interface{}) error {
	spec := extractParameters(class, "spec")
	policy := extractParameters(spec, "overridePolicy")

	requiredParams, ok := policy["requiredParams"].([]interface{})
	if !ok {
		return nil // No required parameters
	}

	for _, r := range requiredParams {
		key := fmt.Sprint(r)
		if _, ok := getNestedValue(merged, key); !ok {
			return fmt.Errorf("required parameter %q is missing", key)
		}
	}
	return nil
}

// renderTemplate executes the Go template with the given parameters.
func renderTemplate(templateString string, params map[string]interface{}) (string, error) {
	tmpl, err := template.New("buildtemplate").Funcs(sprig.TxtFuncMap()).Parse(templateString)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var out bytes.Buffer
	if err := tmpl.Execute(&out, params); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}
	return out.String(), nil
}

func run() error {
	if len(os.Args) != 4 {
		return fmt.Errorf("usage: go run main.go <build.yaml> <buildclass.yaml> <buildtemplate.yaml>")
	}

	buildFile, classFile, templateFile := os.Args[1], os.Args[2], os.Args[3]

	build, err := readYAMLFile(buildFile)
	if err != nil {
		return err
	}
	class, err := readYAMLFile(classFile)
	if err != nil {
		return err
	}

	templateBytes, err := os.ReadFile(templateFile)
	if err != nil {
		return fmt.Errorf("failed to read template file %q: %w", templateFile, err)
	}
	fmt.Println(string(templateBytes))

	templateData, err := readYAMLFile(templateFile)
	if err != nil {
		return err
	}

	// Extract parameters from each file
	buildParams := extractParameters(build, "spec", "parameters")
	classParams := extractParameters(class, "spec", "parameters")
	templateDefaults := extractParameters(templateData, "spec", "defaults")

	// Apply the override policy
	filteredBuildParams, err := applyOverridePolicy(class, buildParams)
	if err != nil {
		return err
	}

	// Merge parameters with the correct priority
	mergedParams := deepMerge(templateDefaults, classParams, filteredBuildParams)

	// Validate required parameters
	if err := ensureRequiredParameters(class, mergedParams); err != nil {
		return err
	}

	// Render the template
	output, err := renderTemplate(string(templateBytes), mergedParams)
	if err != nil {
		return err
	}

	fmt.Print(output)
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}