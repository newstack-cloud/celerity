package preprocess

import "fmt"

// HandlerManifest is the output of the language-specific extraction CLI,
// matching the handler-manifest.v1.schema.json format.
type HandlerManifest struct {
	Version          string            `json:"version"`
	Handlers         []ClassHandler    `json:"handlers"`
	FunctionHandlers []FunctionHandler `json:"functionHandlers"`
	GuardHandlers    []GuardHandler    `json:"guardHandlers"`
}

// ClassHandler describes a handler discovered from a class-based decorator
// (e.g. @Controller + @Get in the Node.js SDK).
type ClassHandler struct {
	ResourceName string         `json:"resourceName"`
	ClassName    string         `json:"className"`
	MethodName   string         `json:"methodName"`
	SourceFile   string         `json:"sourceFile"`
	HandlerType  string         `json:"handlerType"` // "http", "websocket", "consumer", "schedule"
	Annotations  map[string]any `json:"annotations"`
	Spec         HandlerSpec    `json:"spec"`
}

// FunctionHandler describes a handler discovered from a function-based export.
type FunctionHandler struct {
	ResourceName string         `json:"resourceName"`
	ExportName   string         `json:"exportName"`
	SourceFile   string         `json:"sourceFile"`
	Annotations  map[string]any `json:"annotations,omitempty"`
	Spec         HandlerSpec    `json:"spec"`
}

// GuardHandler describes a custom auth guard discovered from a @Guard decorator
// or a function-based guard export.
type GuardHandler struct {
	ResourceName string         `json:"resourceName"`
	GuardName    string         `json:"guardName"`
	SourceFile   string         `json:"sourceFile"`
	GuardType    string         `json:"guardType"` // "class" or "function"
	ClassName    string         `json:"className,omitempty"`
	ExportName   string         `json:"exportName,omitempty"`
	Annotations  map[string]any `json:"annotations"`
	Spec         HandlerSpec    `json:"spec"`
}

// HandlerSpec holds the blueprint spec fields for a handler resource.
type HandlerSpec struct {
	HandlerName  string `json:"handlerName"`
	CodeLocation string `json:"codeLocation"`
	Handler      string `json:"handler"`
	Timeout      *int   `json:"timeout,omitempty"`
}

// AllHandlers returns a combined count of class, function, and guard handlers.
func (m *HandlerManifest) AllHandlers() int {
	return len(m.Handlers) + len(m.FunctionHandlers) + len(m.GuardHandlers)
}

// Equal compares two manifests by handler count and resource names
// to detect structural changes that require a container restart.
func (m *HandlerManifest) Equal(other *HandlerManifest) bool {
	if m == nil || other == nil {
		return m == other
	}

	if len(m.Handlers) != len(other.Handlers) ||
		len(m.FunctionHandlers) != len(other.FunctionHandlers) ||
		len(m.GuardHandlers) != len(other.GuardHandlers) {
		return false
	}

	mNames := m.resourceNameSet()
	oNames := other.resourceNameSet()
	if len(mNames) != len(oNames) {
		return false
	}
	for name := range mNames {
		if !oNames[name] {
			return false
		}
	}

	return m.annotationsEqual(other)
}

func (m *HandlerManifest) resourceNameSet() map[string]bool {
	names := make(map[string]bool, m.AllHandlers())
	for _, h := range m.Handlers {
		names[h.ResourceName] = true
	}
	for _, h := range m.FunctionHandlers {
		names[h.ResourceName] = true
	}
	for _, h := range m.GuardHandlers {
		names[h.ResourceName] = true
	}
	return names
}

func (m *HandlerManifest) annotationsEqual(other *HandlerManifest) bool {
	mAnn := m.annotationsByResource()
	oAnn := other.annotationsByResource()

	if len(mAnn) != len(oAnn) {
		return false
	}

	for name, ann := range mAnn {
		otherAnn, ok := oAnn[name]
		if !ok {
			return false
		}
		if len(ann) != len(otherAnn) {
			return false
		}
		for key, val := range ann {
			if otherVal, exists := otherAnn[key]; !exists || val != otherVal {
				return false
			}
		}
	}
	return true
}

func (m *HandlerManifest) annotationsByResource() map[string]map[string]string {
	result := make(map[string]map[string]string, m.AllHandlers())
	for _, h := range m.Handlers {
		result[h.ResourceName] = stringifyAnnotations(h.Annotations)
	}
	for _, h := range m.FunctionHandlers {
		result[h.ResourceName] = stringifyAnnotations(h.Annotations)
	}
	for _, h := range m.GuardHandlers {
		result[h.ResourceName] = stringifyAnnotations(h.Annotations)
	}
	return result
}

func stringifyAnnotations(ann map[string]any) map[string]string {
	if ann == nil {
		return map[string]string{}
	}
	result := make(map[string]string, len(ann))
	for k, v := range ann {
		result[k] = stringify(v)
	}
	return result
}

func stringify(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}
