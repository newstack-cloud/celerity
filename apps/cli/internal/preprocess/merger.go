package preprocess

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/newstack-cloud/bluelink/libs/blueprint/core"
	"github.com/newstack-cloud/bluelink/libs/blueprint/schema"
	"github.com/newstack-cloud/bluelink/libs/blueprint/substitutions"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

const resourceTypeHandler = "celerity/handler"
const resourceTypeApi = "celerity/api"

// linkLabelKey is the standard Celerity label key used to link
// handler resources to an API resource via linkSelector.byLabel.
const linkLabelKey = "application"

// customGuardAnnotation is the annotation key the extraction CLI sets
// on handler resources decorated with @Guard("name").
const customGuardAnnotation = "celerity.handler.guard.custom"

// Merge enriches a blueprint with extracted handler metadata.
// For each handler in the manifest:
//   - If a matching resource exists (by resource name): fill in missing annotations and spec
//   - If no matching resource exists: create a new handler resource
func Merge(
	bp *schema.Blueprint,
	manifest *HandlerManifest,
	logger *zap.Logger,
) (*schema.Blueprint, error) {
	if bp.Resources == nil {
		bp.Resources = &schema.ResourceMap{
			Values: map[string]*schema.Resource{},
		}
	}

	for _, handler := range manifest.Handlers {
		mergeHandler(bp, handler.ResourceName, handler.Annotations, handler.Spec)
		logger.Debug("merged class handler",
			zap.String("resource", handler.ResourceName),
			zap.String("class", handler.ClassName),
		)
	}

	for _, handler := range manifest.FunctionHandlers {
		mergeHandler(bp, handler.ResourceName, handler.Annotations, handler.Spec)
		logger.Debug("merged function handler",
			zap.String("resource", handler.ResourceName),
			zap.String("export", handler.ExportName),
		)
	}

	for _, guard := range manifest.GuardHandlers {
		mergeHandler(bp, guard.ResourceName, guard.Annotations, guard.Spec)
		logger.Debug("merged guard handler",
			zap.String("resource", guard.ResourceName),
			zap.String("guard", guard.GuardName),
			zap.String("guardType", guard.GuardType),
		)
	}

	// Wire each handler into the resources it belongs to, through the labels
	// a link selector matches on.
	if manifest.AllHandlers() > 0 {
		linkHandlersToApi(bp, manifest)
		linkHandlersToConsumers(bp, manifest, logger)
		linkHandlersToSchedules(bp, manifest, logger)
		linkHandlersToResources(bp, manifest, logger)
	}

	return bp, nil
}

// WriteMerged serializes the blueprint to the staging location.
// The output format matches the original blueprint format.
func WriteMerged(
	bp *schema.Blueprint,
	format schema.SpecFormat,
	outputDir string,
) (string, error) {
	return WriteMergedWithMode(bp, format, outputDir, 0o644)
}

// WriteMergedWithMode is WriteMerged with the file permissions stated.
//
// A separate entry point because the two callers need different modes. A dev
// session's merged blueprint is ordinary generated output, while a deploy's
// carries the environment's configuration values, secrets among them, seeded
// so a first deploy can stand its store up, and that one must not be world
// readable.
func WriteMergedWithMode(
	bp *schema.Blueprint,
	format schema.SpecFormat,
	outputDir string,
	mode os.FileMode,
) (string, error) {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return "", fmt.Errorf("creating output directory: %w", err)
	}

	var (
		data     []byte
		err      error
		filename string
	)

	switch format {
	case schema.JWCCSpecFormat:
		data, err = json.MarshalIndent(bp, "", "  ")
		filename = "merged.blueprint.jsonc"
	default:
		data, err = yaml.Marshal(bp)
		filename = "merged.blueprint.yaml"
	}

	if err != nil {
		return "", fmt.Errorf("serializing merged blueprint: %w", err)
	}

	outPath := filepath.Join(outputDir, filename)
	tmpPath := outPath + ".tmp"

	if err := os.WriteFile(tmpPath, data, mode); err != nil {
		return "", fmt.Errorf("writing merged blueprint: %w", err)
	}

	// Set explicitly as well as passed to WriteFile, which applies the
	// process umask and so cannot be relied on to produce the mode asked for.
	if err := os.Chmod(tmpPath, mode); err != nil {
		return "", fmt.Errorf("setting merged blueprint permissions: %w", err)
	}

	if err := os.Rename(tmpPath, outPath); err != nil {
		return "", fmt.Errorf("renaming merged blueprint: %w", err)
	}

	return outPath, nil
}

func mergeHandler(
	bp *schema.Blueprint,
	resourceName string,
	annotations map[string]any,
	spec HandlerSpec,
) {
	existing, exists := bp.Resources.Values[resourceName]
	if exists {
		fillExistingResource(existing, resourceName, annotations, spec)
		return
	}

	bp.Resources.Values[resourceName] = buildNewResource(resourceName, annotations, spec)
}

// Blueprint-defined values take precedence for infrastructure config
// (e.g. timeout); extracted values take precedence for routing annotations.
func fillExistingResource(
	resource *schema.Resource,
	resourceName string,
	annotations map[string]any,
	spec HandlerSpec,
) {
	ensureMetadata(resource)
	if resource.Metadata.DisplayName == nil {
		resource.Metadata.DisplayName = stringToSubstitutions(resourceName)
	}

	for key, value := range annotations {
		if _, exists := resource.Metadata.Annotations.Values[key]; !exists {
			resource.Metadata.Annotations.Values[key] = stringToSubstitutions(formatAnnotationValue(value))
		}
	}

	ensureSpec(resource)
	setSpecFieldIfEmpty(resource.Spec, "handlerName", spec.HandlerName)
	setSpecFieldIfEmpty(resource.Spec, "codeLocation", spec.CodeLocation)
	setSpecFieldIfEmpty(resource.Spec, "handler", spec.Handler)
}

func buildNewResource(
	resourceName string,
	annotations map[string]any,
	spec HandlerSpec,
) *schema.Resource {
	resource := &schema.Resource{
		Type: &schema.ResourceTypeWrapper{
			Value: resourceTypeHandler,
		},
	}

	ensureMetadata(resource)
	resource.Metadata.DisplayName = stringToSubstitutions(resourceName)
	for key, value := range annotations {
		resource.Metadata.Annotations.Values[key] = stringToSubstitutions(formatAnnotationValue(value))
	}

	resource.Spec = &core.MappingNode{
		Fields: map[string]*core.MappingNode{},
	}

	setSpecField(resource.Spec, "handlerName", spec.HandlerName)
	setSpecField(resource.Spec, "codeLocation", spec.CodeLocation)
	setSpecField(resource.Spec, "handler", spec.Handler)

	if spec.Timeout != nil {
		resource.Spec.Fields["timeout"] = &core.MappingNode{
			Scalar: core.ScalarFromInt(*spec.Timeout),
		}
	}

	return resource
}

func ensureMetadata(resource *schema.Resource) {
	if resource.Metadata == nil {
		resource.Metadata = &schema.Metadata{}
	}
	if resource.Metadata.Annotations == nil {
		resource.Metadata.Annotations = &schema.StringOrSubstitutionsMap{
			Values: map[string]*substitutions.StringOrSubstitutions{},
		}
	}
}

func ensureSpec(resource *schema.Resource) {
	if resource.Spec == nil {
		resource.Spec = &core.MappingNode{
			Fields: map[string]*core.MappingNode{},
		}
	}
	if resource.Spec.Fields == nil {
		resource.Spec.Fields = map[string]*core.MappingNode{}
	}
}

func setSpecFieldIfEmpty(spec *core.MappingNode, key string, value string) {
	if value == "" {
		return
	}
	if existing := core.StringValue(spec.Fields[key]); existing != "" {
		return
	}
	setSpecField(spec, key, value)
}

func setSpecField(spec *core.MappingNode, key string, value string) {
	if value == "" {
		return
	}
	spec.Fields[key] = &core.MappingNode{
		Scalar: core.ScalarFromString(value),
	}
}

// Labels every extracted handler resource for the blueprint's celerity/api
// resource, so the runtime resolves them by label matching.
func linkHandlersToApi(bp *schema.Blueprint, manifest *HandlerManifest) {
	apiName := ""
	for name, resource := range bp.Resources.Values {
		if resource.Type != nil && resource.Type.Value == resourceTypeApi {
			apiName = name
			if resource.LinkSelector == nil {
				resource.LinkSelector = &schema.LinkSelector{
					ByLabel: &schema.StringMap{
						Values: map[string]string{linkLabelKey: apiName},
					},
				}
			}
			break
		}
	}
	if apiName == "" {
		return
	}

	// Collect handler resource names that belong to the API (HTTP, WebSocket, guards).
	// Consumer and schedule handlers are linked to their own source resources instead.
	handlerNames := make(map[string]bool, manifest.AllHandlers())
	for _, h := range manifest.Handlers {
		if h.HandlerType == "consumer" || h.HandlerType == "schedule" {
			continue
		}
		handlerNames[h.ResourceName] = true
	}
	for _, h := range manifest.FunctionHandlers {
		handlerNames[h.ResourceName] = true
	}
	for _, h := range manifest.GuardHandlers {
		handlerNames[h.ResourceName] = true
	}

	apiResource := bp.Resources.Values[apiName]
	for name, resource := range bp.Resources.Values {
		if handlerNames[name] {
			ensureLabels(resource)
			resource.Metadata.Labels.Values[linkLabelKey] = apiName
		}
	}

	mergeCustomGuards(apiResource, bp)
}

// Each handler carrying customGuardAnnotation gets a { type: custom } entry in
// the API's spec.auth.guards.
func mergeCustomGuards(apiResource *schema.Resource, bp *schema.Blueprint) {
	ensureSpec(apiResource)

	for _, resource := range bp.Resources.Values {
		if resource.Type == nil || resource.Type.Value != resourceTypeHandler {
			continue
		}
		guardName := annotationStringValue(resource, customGuardAnnotation)
		if guardName == "" {
			continue
		}

		guards := ensureSpecPath(apiResource.Spec, "auth", "guards")
		if _, exists := guards.Fields[guardName]; exists {
			continue
		}
		guards.Fields[guardName] = &core.MappingNode{
			Fields: map[string]*core.MappingNode{
				"type": {Scalar: core.ScalarFromString("custom")},
			},
		}
	}
}

func annotationStringValue(resource *schema.Resource, key string) string {
	if resource.Metadata == nil || resource.Metadata.Annotations == nil {
		return ""
	}
	ann, ok := resource.Metadata.Annotations.Values[key]
	if !ok || ann == nil {
		return ""
	}
	s, err := substitutions.SubstitutionsToString("", ann)
	if err != nil {
		return ""
	}
	return s
}

// Navigates nested MappingNode fields, creating any that are absent.
func ensureSpecPath(spec *core.MappingNode, keys ...string) *core.MappingNode {
	current := spec
	for _, key := range keys {
		if current.Fields == nil {
			current.Fields = map[string]*core.MappingNode{}
		}
		child, ok := current.Fields[key]
		if !ok || child == nil {
			child = &core.MappingNode{
				Fields: map[string]*core.MappingNode{},
			}
			current.Fields[key] = child
		}
		if child.Fields == nil {
			child.Fields = map[string]*core.MappingNode{}
		}
		current = child
	}
	return current
}

const resourceTypeConsumer = "celerity/consumer"
const resourceTypeSchedule = "celerity/schedule"

// consumerSourceAnnotation is the annotation key set by the extraction CLI
// on handler resources decorated with @Consumer("consumerName").
const consumerSourceAnnotation = "celerity.handler.consumer.source"

// scheduleSourceAnnotation is the annotation key set by the extraction CLI
// on handler methods decorated with @ScheduleHandler("scheduleName").
const scheduleSourceAnnotation = "celerity.handler.schedule.source"

// Links consumer-type handler resources to their celerity/consumer resource,
// so the runtime resolves them by label matching.
func linkHandlersToConsumers(bp *schema.Blueprint, manifest *HandlerManifest, logger *zap.Logger) {
	linkHandlersToSourceResources(bp, manifest, resourceTypeConsumer, consumerSourceAnnotation, logger)
}

// Links schedule-type handler resources to their celerity/schedule resource.
func linkHandlersToSchedules(bp *schema.Blueprint, manifest *HandlerManifest, logger *zap.Logger) {
	linkHandlersToSourceResources(bp, manifest, resourceTypeSchedule, scheduleSourceAnnotation, logger)
}

// Shared by consumer and schedule linking. Handlers are grouped by their
// source annotation value, which is the resource name of the consumer or
// schedule, and each source gets a linkSelector its handlers are labelled to
// match.
func linkHandlersToSourceResources(
	bp *schema.Blueprint,
	manifest *HandlerManifest,
	sourceResourceType string,
	sourceAnnotationKey string,
	logger *zap.Logger,
) {
	sourceToHandlers := map[string][]string{}
	for _, h := range manifest.Handlers {
		source, ok := h.Annotations[sourceAnnotationKey]
		if !ok {
			continue
		}
		sourceStr := fmt.Sprint(source)
		sourceToHandlers[sourceStr] = append(sourceToHandlers[sourceStr], h.ResourceName)
	}
	for _, h := range manifest.FunctionHandlers {
		if h.Annotations == nil {
			continue
		}
		source, ok := h.Annotations[sourceAnnotationKey]
		if !ok {
			continue
		}
		sourceStr := fmt.Sprint(source)
		sourceToHandlers[sourceStr] = append(sourceToHandlers[sourceStr], h.ResourceName)
	}

	if len(sourceToHandlers) == 0 {
		return
	}

	for sourceName, handlerNames := range sourceToHandlers {
		sourceResource, exists := bp.Resources.Values[sourceName]
		if !exists {
			logger.Warn("handler references unknown source resource",
				zap.String("source", sourceName),
				zap.String("sourceType", sourceResourceType),
			)
			continue
		}
		if sourceResource.Type == nil || sourceResource.Type.Value != sourceResourceType {
			continue
		}

		labelKey := "sourceConsumer"
		if sourceResourceType == resourceTypeSchedule {
			labelKey = "sourceSchedule"
		}

		if sourceResource.LinkSelector == nil {
			sourceResource.LinkSelector = &schema.LinkSelector{
				ByLabel: &schema.StringMap{
					Values: map[string]string{labelKey: sourceName},
				},
			}
		}

		labelValue := sourceName
		if sourceResource.LinkSelector.ByLabel != nil {
			// Use existing label key/value if linkSelector was already set.
			for k, v := range sourceResource.LinkSelector.ByLabel.Values {
				labelKey = k
				labelValue = v
				break
			}
		}

		for _, handlerName := range handlerNames {
			handlerResource, ok := bp.Resources.Values[handlerName]
			if !ok {
				continue
			}
			ensureLabels(handlerResource)
			handlerResource.Metadata.Labels.Values[labelKey] = labelValue
			logger.Debug("linked handler to source resource",
				zap.String("handler", handlerName),
				zap.String("source", sourceName),
				zap.String("label", labelKey+"="+labelValue),
			)
		}
	}
}

func ensureLabels(resource *schema.Resource) {
	ensureMetadata(resource)
	if resource.Metadata.Labels == nil {
		resource.Metadata.Labels = &schema.StringMap{
			Values: map[string]string{},
		}
	}
}

// JSON arrays from the extraction CLI are joined with commas to match the
// comma-separated format the runtime expects (e.g. "admin,jwt").
func formatAnnotationValue(value any) string {
	switch v := value.(type) {
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			parts = append(parts, fmt.Sprint(item))
		}
		return strings.Join(parts, ",")
	case []string:
		return strings.Join(v, ",")
	default:
		return fmt.Sprint(value)
	}
}

func stringToSubstitutions(s string) *substitutions.StringOrSubstitutions {
	return &substitutions.StringOrSubstitutions{
		Values: []*substitutions.StringOrSubstitution{
			{StringValue: &s},
		},
	}
}
