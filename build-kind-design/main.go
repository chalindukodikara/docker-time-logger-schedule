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

// readYAMLFile parses a YAML file into a map
func readYAMLFile(path string) map[string]interface{} {
	raw, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	var out map[string]interface{}
	if err := yaml.Unmarshal(raw, &out); err != nil {
		panic(err)
	}
	return out
}

// extractParameters follows nested keys to get to parameters
func extractParameters(m map[string]interface{}, path ...string) map[string]interface{} {
	for _, key := range path {
		if m == nil {
			return map[string]interface{}{}
		}
		m = m[key].(map[string]interface{})
	}
	return m
}

// deepMerge merges src into dst recursively
func deepMerge(dst, src map[string]interface{}) map[string]interface{} {
	for k, v := range src {
		if srcMap, ok := v.(map[string]interface{}); ok {
			if dstMap, ok := dst[k].(map[string]interface{}); ok {
				dst[k] = deepMerge(dstMap, srcMap)
			} else {
				dst[k] = deepMerge(make(map[string]interface{}), srcMap)
			}
		} else {
			dst[k] = v
		}
	}
	return dst
}

// getNested reads a dotted path like "env.BP_GO_VERSION"
func getNested(m map[string]interface{}, dotted string) (interface{}, bool) {
	parts := strings.Split(dotted, ".")
	for i, p := range parts {
		if i == len(parts)-1 {
			v, ok := m[p]
			return v, ok
		}
		if next, ok := m[p].(map[string]interface{}); ok {
			m = next
		} else {
			return nil, false
		}
	}
	return nil, false
}

// applyOverridePolicy filters buildParams based on overridePolicy.restrictedParams
func applyOverridePolicy(class map[string]interface{}, buildParams map[string]interface{}) map[string]interface{} {
	spec := extractParameters(class, "spec")
	policy := extractParameters(spec, "overridePolicy")

	allow := true
	if v, ok := policy["allowParamOverride"].(bool); ok {
		allow = v
	}
	if !allow {
		return map[string]interface{}{}
	}

	restricted := map[string]struct{}{}
	for _, r := range policy["restrictedParams"].([]interface{}) {
		restricted[fmt.Sprint(r)] = struct{}{}
	}

	filtered := map[string]interface{}{}
	var walk func(prefix string, src, dst map[string]interface{})
	walk = func(prefix string, src, dst map[string]interface{}) {
		for k, v := range src {
			key := k
			if prefix != "" {
				key = prefix + "." + k
			}
			if _, blocked := restricted[key]; blocked {
				continue
			}
			if vm, ok := v.(map[string]interface{}); ok {
				nested := map[string]interface{}{}
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
	return filtered
}

// ensureRequired checks that all requiredParams are present in the final parameter map
func ensureRequired(class map[string]interface{}, merged map[string]interface{}) {
	spec := extractParameters(class, "spec")
	policy := extractParameters(spec, "overridePolicy")

	for _, r := range policy["requiredParams"].([]interface{}) {
		key := fmt.Sprint(r)
		if _, ok := getNested(merged, key); !ok {
			panic(fmt.Sprintf("required parameter %q missing", key))
		}
	}
}

func main() {
	if len(os.Args) != 4 {
		fmt.Println("Usage: go run main.go build.yaml buildclass.yaml buildtemplate.yaml")
		os.Exit(1)
	}

	build := readYAMLFile(os.Args[1])
	class := readYAMLFile(os.Args[2])
	templateRaw, err := os.ReadFile(os.Args[3])
	if err != nil {
		panic(err)
	}

	buildParams := extractParameters(build, "spec", "parameters")
	classParams := extractParameters(class, "spec", "parameters")

	// Filter out restricted fields from build
	filteredBuild := applyOverridePolicy(class, buildParams)

	// Final = BuildClass + filtered(Build) — Build has higher priority
	merged := deepMerge(classParams, filteredBuild)

	// Validate required params
	ensureRequired(class, merged)

	// Render template using final merged values
	tmpl, err := template.New("buildtemplate").Funcs(sprig.TxtFuncMap()).Parse(string(templateRaw))
	if err != nil {
		panic(err)
	}

	var out bytes.Buffer
	if err := tmpl.Execute(&out, merged); err != nil {
		panic(err)
	}
	fmt.Print(out.String())
}
