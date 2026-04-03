package blueprint

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/newstack-cloud/bluelink/libs/blueprint/core"
	"github.com/newstack-cloud/bluelink/libs/blueprint/schema"
	"github.com/newstack-cloud/bluelink/libs/blueprint/substitutions"
)

const (
	resourceTypeHandler       = "celerity/handler"
	resourceTypeHandlerConfig = "celerity/handlerConfig"
)

// Supported runtime-to-image-repo mappings.
var runtimeImageRepos = map[string]string{
	"nodejs24.x": "ghcr.io/newstack-cloud/celerity-runtime-nodejs-24",
	"python3.13": "ghcr.io/newstack-cloud/celerity-runtime-python-3-13",
}

// HandlerInfo holds extracted metadata about a handler resource.
type HandlerInfo struct {
	ResourceName string
	HandlerName  string
	Method       string
	Path         string
	HandlerType  string
	CodeLocation string
	Runtime      string
}

// LoadForDev loads a blueprint from a file path in parse-only mode
// (no provider/transformer validation). Returns the parsed blueprint
// and the detected format so the merger can serialize in the same format.
func LoadForDev(path string) (*schema.Blueprint, schema.SpecFormat, error) {
	format := detectFormat(path)
	bp, err := schema.Load(path, format)
	if err != nil {
		return nil, "", fmt.Errorf("loading blueprint %s: %w", path, err)
	}
	return bp, format, nil
}

// DetectRuntime scans handler resources for the runtime field.
// Returns the runtime string (e.g. "nodejs24.x") or an error
// if no handlers exist or multiple different runtimes are found.
func DetectRuntime(bp *schema.Blueprint) (string, error) {
	if bp.Resources == nil {
		return "", fmt.Errorf("blueprint has no resources")
	}

	var detected string
	for name, resource := range bp.Resources.Values {
		if !isHandler(resource) {
			continue
		}

		runtime := specStringField(resource, "runtime")
		if runtime == "" {
			continue
		}

		if detected == "" {
			detected = runtime
			continue
		}

		if detected != runtime {
			return "", fmt.Errorf(
				"multiple runtimes found: %q (from %s) and %q; all handlers must use the same runtime",
				detected, name, runtime,
			)
		}
	}

	if detected == "" {
		detected = detectRuntimeFromHandlerConfigs(bp)
	}
	if detected == "" {
		detected = metadataSharedRuntime(bp)
	}

	if detected == "" {
		return "", fmt.Errorf(
			"no runtime found in handler resources, handlerConfig resources, or metadata.sharedHandlerConfig",
		)
	}

	return detected, nil
}

// projectFileRuntimes maps project files to their default runtime identifiers.
// Used as a fallback when no handler resources exist in the blueprint
// (decorator-driven projects where handlers are extracted from source code).
var projectFileRuntimes = []struct {
	File    string
	Runtime string
}{
	{"package.json", "nodejs24.x"},
	{"pyproject.toml", "python3.13"},
}

// DetectRuntimeFromProject infers the runtime from project files in appDir.
// This is a fallback for decorator-driven projects where the blueprint has
// no handler resources yet (they are created by the extraction step).
func DetectRuntimeFromProject(appDir string) (string, error) {
	for _, pf := range projectFileRuntimes {
		if _, err := os.Stat(filepath.Join(appDir, pf.File)); err == nil {
			return pf.Runtime, nil
		}
	}

	return "", fmt.Errorf(
		"cannot detect runtime: no handler resources in blueprint and no recognised "+
			"project files (package.json, pyproject.toml) found in %s",
		appDir,
	)
}

// ResolveRuntimeImage maps a blueprint runtime identifier to a full Docker image reference.
// The image repo encodes the language and major version; the tag encodes the dev image version.
func ResolveRuntimeImage(runtime string, imageVersion string) (string, error) {
	repo, ok := runtimeImageRepos[runtime]
	if !ok {
		supported := make([]string, 0, len(runtimeImageRepos))
		for k := range runtimeImageRepos {
			supported = append(supported, k)
		}
		return "", fmt.Errorf(
			"unsupported runtime %q; supported runtimes: %s",
			runtime, strings.Join(supported, ", "),
		)
	}
	return repo + ":dev-" + imageVersion, nil
}

// CollectHandlerInfo extracts handler metadata from the blueprint
// for display in startup output and state tracking.
func CollectHandlerInfo(bp *schema.Blueprint) []HandlerInfo {
	if bp.Resources == nil {
		return nil
	}

	var handlers []HandlerInfo
	for name, resource := range bp.Resources.Values {
		if !isHandler(resource) {
			continue
		}

		info := HandlerInfo{
			ResourceName: name,
			HandlerName:  specStringField(resource, "handlerName"),
			CodeLocation: specStringField(resource, "codeLocation"),
			Runtime:      specStringField(resource, "runtime"),
		}

		if info.HandlerName == "" {
			info.HandlerName = name
		}

		info.Method = annotationValue(resource, "celerity.handler.http.method")
		info.Path = annotationValue(resource, "celerity.handler.http.path")
		info.HandlerType = resolveHandlerType(resource)

		handlers = append(handlers, info)
	}

	return handlers
}

// detectRuntimeFromHandlerConfigs scans celerity/handlerConfig resources
// for a runtime field. Used as a fallback when no handler has spec.runtime set.
func detectRuntimeFromHandlerConfigs(bp *schema.Blueprint) string {
	if bp.Resources == nil {
		return ""
	}
	for _, resource := range bp.Resources.Values {
		if resource.Type == nil || resource.Type.Value != resourceTypeHandlerConfig {
			continue
		}
		runtime := specStringField(resource, "runtime")
		if runtime != "" {
			return runtime
		}
	}
	return ""
}

// metadataSharedRuntime reads the runtime field from metadata.sharedHandlerConfig
// in the blueprint. Used as a final fallback when neither handlers nor
// handlerConfig resources specify a runtime.
func metadataSharedRuntime(bp *schema.Blueprint) string {
	if bp.Metadata == nil || bp.Metadata.Fields == nil {
		return ""
	}
	shc, ok := bp.Metadata.Fields["sharedHandlerConfig"]
	if !ok || shc == nil || shc.Fields == nil {
		return ""
	}
	return core.StringValue(shc.Fields["runtime"])
}

func isHandler(resource *schema.Resource) bool {
	return resource.Type != nil && resource.Type.Value == resourceTypeHandler
}

func specStringField(resource *schema.Resource, field string) string {
	if resource.Spec == nil || resource.Spec.Fields == nil {
		return ""
	}
	return core.StringValue(resource.Spec.Fields[field])
}

func annotationValue(resource *schema.Resource, key string) string {
	if resource.Metadata == nil || resource.Metadata.Annotations == nil {
		return ""
	}

	ann, ok := resource.Metadata.Annotations.Values[key]
	if !ok || ann == nil {
		return ""
	}

	str, err := substitutions.SubstitutionsToString("", ann)
	if err != nil {
		return ""
	}
	return str
}

func resolveHandlerType(resource *schema.Resource) string {
	if annotationValue(resource, "celerity.handler.http.method") != "" {
		return "http"
	}
	if annotationValue(resource, "celerity.handler.websocket") != "" {
		return "websocket"
	}
	if annotationValue(resource, "celerity.handler.schedule") != "" {
		return "schedule"
	}
	if annotationValue(resource, "celerity.handler.consumer.sourceType") != "" {
		return "consumer"
	}
	return "http"
}

func detectFormat(path string) schema.SpecFormat {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".jsonc", ".json":
		return schema.JWCCSpecFormat
	default:
		return schema.YAMLSpecFormat
	}
}
