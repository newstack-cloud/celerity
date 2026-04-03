package compose

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/newstack-cloud/bluelink/libs/blueprint/core"
	"github.com/newstack-cloud/bluelink/libs/blueprint/schema"
	"github.com/newstack-cloud/bluelink/libs/blueprint/substitutions"
	"github.com/newstack-cloud/celerity/apps/cli/internal/devstubs"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

// GenerateComposeConfig generates a Docker Compose configuration from a blueprint.
// It inspects blueprint resource types, maps each to a local emulator service,
// deduplicates shared services (e.g. Valkey), writes the compose file, and
// returns the resulting config including runtime environment variables.
//
// portOffset shifts all host port mappings by the given amount.
// Use 0 for dev run (default ports) and a positive value (e.g. 100) for
// dev test so both can run simultaneously without port conflicts.
func GenerateComposeConfig(
	bp *schema.Blueprint,
	deployTarget string,
	projectName string,
	appDir string,
	portOffset int,
	localAuth bool,
	logger *zap.Logger,
) (*ComposeConfig, error) {
	emptyConfig := &ComposeConfig{
		Services:       map[string]*ComposeService{},
		RuntimeEnvVars: map[string]string{},
		HostEnvVars:    map[string]string{},
		ProjectName:    projectName,
	}
	if bp.Resources == nil {
		return emptyConfig, nil
	}

	resourceTypes := collectResourceTypes(bp)
	if len(resourceTypes) == 0 {
		return emptyConfig, nil
	}

	services := map[string]*ComposeService{}
	runtimeEnvVars := map[string]string{}

	if resourceTypes[ResourceTypeDatastore] {
		if err := addDatastoreService(services, runtimeEnvVars, deployTarget, portOffset); err != nil {
			return nil, err
		}
	}

	if resourceTypes[ResourceTypeBucket] {
		addBucketService(services, runtimeEnvVars, portOffset)
	}

	if resourceTypes[ResourceTypeSqlDatabase] {
		engine := detectSqlEngine(bp)
		if err := addSqlDatabaseService(services, runtimeEnvVars, engine, portOffset); err != nil {
			return nil, err
		}
	}

	addValkeyServiceIfNeeded(services, runtimeEnvVars, resourceTypes, portOffset)
	addDevAuthServiceIfNeeded(services, runtimeEnvVars, bp, portOffset, localAuth)
	stubServices, err := addStubsServiceIfNeeded(services, runtimeEnvVars, appDir, portOffset, logger)
	if err != nil {
		return nil, err
	}
	runtimeEnvVars["CELERITY_MAX_DIAGNOSTICS_LEVEL"] = "info"
	streamEnabledTables, err := addLocalEventsServiceIfNeeded(services, bp, deployTarget, appDir, portOffset)
	if err != nil {
		return nil, err
	}

	composeFilePath := filepath.Join(appDir, ".celerity", "compose.generated.yaml")
	if err := writeComposeFile(composeFilePath, services); err != nil {
		return nil, err
	}

	logger.Debug("generated compose config",
		zap.String("path", composeFilePath),
		zap.Int("services", len(services)),
	)

	hostEnvVars := buildHostEnvVars(runtimeEnvVars, portOffset)

	return &ComposeConfig{
		Services:            services,
		RuntimeEnvVars:      runtimeEnvVars,
		HostEnvVars:         hostEnvVars,
		ProjectName:         projectName,
		FilePath:            composeFilePath,
		StreamEnabledTables: streamEnabledTables,
		StubServices:        stubServices,
	}, nil
}

func collectResourceTypes(bp *schema.Blueprint) map[string]bool {
	types := map[string]bool{}
	for _, resource := range bp.Resources.Values {
		if resource.Type != nil {
			types[resource.Type.Value] = true
		}
	}
	return types
}

func addDatastoreService(
	services map[string]*ComposeService,
	runtimeEnvVars map[string]string,
	deployTarget string,
	portOffset int,
) error {
	mapping, err := datastoreMappingForTarget(deployTarget)
	if err != nil {
		return err
	}

	service := &ComposeService{
		Image:       mapping.Image,
		Environment: mapping.Environment,
		Command:     mapping.Command,
	}
	service.Ports = formatPortsWithOffset(mapping.Ports, portOffset)

	if mapping.HealthCheck != nil {
		service.HealthCheck = toComposeHealth(mapping.HealthCheck)
	}

	services[ServiceNameDatastore] = service
	maps.Copy(runtimeEnvVars, mapping.RuntimeEnvVars)

	return nil
}

func datastoreMappingForTarget(deployTarget string) (*ServiceMapping, error) {
	switch deployTarget {
	case DeployTargetAWS, DeployTargetAWSServerless:
		return &ServiceMapping{
			Image: dynamoDBLocalImage,
			Ports: []PortMapping{
				{Host: dynamoDBLocalPort, Container: dynamoDBLocalPort},
			},
			Command: []string{"-jar", "DynamoDBLocal.jar", "-sharedDb"},
			HealthCheck: &HealthCheck{
				Test:        []string{"CMD-SHELL", "curl -s http://localhost:8000 >/dev/null 2>&1"},
				Interval:    "5s",
				Timeout:     "5s",
				Retries:     10,
				StartPeriod: "20s",
			},
			RuntimeEnvVars: map[string]string{
				EnvDatastoreEndpoint:             "http://" + ServiceNameDatastore + ":" + dynamoDBLocalPort,
				"CELERITY_AWS_DYNAMODB_ENDPOINT": "http://" + ServiceNameDatastore + ":" + dynamoDBLocalPort,
				"AWS_REGION":                     "us-east-1",
				"AWS_ACCESS_KEY_ID":              "local",
				"AWS_SECRET_ACCESS_KEY":          "local",
			},
		}, nil

	case DeployTargetGCloud, DeployTargetGCloudServerless:
		return nil, fmt.Errorf(
			"local datastore emulator for deploy target %q is not yet supported (planned for v1)",
			deployTarget,
		)

	case DeployTargetAzure, DeployTargetAzureServerless:
		return nil, fmt.Errorf(
			"local datastore emulator for deploy target %q is not yet supported (planned for v1)",
			deployTarget,
		)

	default:
		return nil, fmt.Errorf("unsupported deploy target %q for datastore emulator", deployTarget)
	}
}

func addBucketService(
	services map[string]*ComposeService,
	runtimeEnvVars map[string]string,
	portOffset int,
) {
	services[ServiceNameStorage] = &ComposeService{
		Image: minioImage,
		Ports: []string{
			offsetPort(minioPort, portOffset) + ":" + minioPort,
			offsetPort(minioAPIPort, portOffset) + ":" + minioAPIPort,
		},
		Environment: map[string]string{
			"MINIO_ROOT_USER":     defaultMinioCreds.AccessKey,
			"MINIO_ROOT_PASSWORD": defaultMinioCreds.SecretKey,
		},
		Command: []string{"server", "/data", "--console-address", ":" + minioAPIPort},
		HealthCheck: &ComposeHealth{
			Test:        []string{"CMD", "mc", "ready", "local"},
			Interval:    "5s",
			Timeout:     "3s",
			Retries:     3,
			StartPeriod: "10s",
		},
	}

	runtimeEnvVars[EnvBucketEndpoint] = "http://" + ServiceNameStorage + ":" + minioPort
	runtimeEnvVars[EnvBucketAccessKey] = defaultMinioCreds.AccessKey
	runtimeEnvVars[EnvBucketSecretKey] = defaultMinioCreds.SecretKey
	runtimeEnvVars["CELERITY_AWS_S3_ENDPOINT"] = "http://" + ServiceNameStorage + ":" + minioPort
}

func addSqlDatabaseService(
	services map[string]*ComposeService,
	runtimeEnvVars map[string]string,
	engine string,
	portOffset int,
) error {
	mapping, err := sqlDatabaseMappingForEngine(engine)
	if err != nil {
		return err
	}

	services[ServiceNameSqlDatabase] = &ComposeService{
		Image:       mapping.Image,
		Ports:       formatPortsWithOffset(mapping.Ports, portOffset),
		Environment: mapping.Environment,
		HealthCheck: toComposeHealth(mapping.HealthCheck),
	}

	maps.Copy(runtimeEnvVars, mapping.RuntimeEnvVars)
	return nil
}

func sqlDatabaseMappingForEngine(engine string) (*ServiceMapping, error) {
	switch engine {
	// An empty or missing engine defaults to Postgres.
	case "postgres", "":
		return &ServiceMapping{
			Image: postgresImage,
			Ports: []PortMapping{
				{Host: postgresPort, Container: postgresPort},
			},
			Environment: map[string]string{
				"POSTGRES_USER":     defaultPostgresCreds.User,
				"POSTGRES_PASSWORD": defaultPostgresCreds.Password,
				"POSTGRES_DB":       defaultPostgresCreds.Database,
			},
			HealthCheck: &HealthCheck{
				Test:        []string{"CMD-SHELL", "pg_isready -U " + defaultPostgresCreds.User},
				Interval:    "5s",
				Timeout:     "5s",
				Retries:     5,
				StartPeriod: "10s",
			},
			RuntimeEnvVars: map[string]string{
				EnvSqlDatabaseEndpoint: fmt.Sprintf(
					"postgres://%s:%s@%s:%s/%s?sslmode=disable",
					defaultPostgresCreds.User,
					defaultPostgresCreds.Password,
					ServiceNameSqlDatabase,
					postgresPort,
					defaultPostgresCreds.Database,
				),
			},
		}, nil

	case "mysql":
		return nil, fmt.Errorf(
			"SQL database engine %q is not yet supported (planned for v1)", engine,
		)

	default:
		return nil, fmt.Errorf("unsupported SQL database engine %q", engine)
	}
}

func detectSqlEngine(bp *schema.Blueprint) string {
	if bp.Resources == nil {
		return ""
	}
	for _, resource := range bp.Resources.Values {
		if resource.Type == nil || resource.Type.Value != ResourceTypeSqlDatabase {
			continue
		}
		engine := specStringFieldFromNode(resource.Spec, "engine")
		if engine != "" {
			return engine
		}
	}
	return ""
}

func addValkeyServiceIfNeeded(
	services map[string]*ComposeService,
	runtimeEnvVars map[string]string,
	resourceTypes map[string]bool,
	portOffset int,
) {
	valkeyURL := "redis://" + ServiceNameValkey + ":" + valkeyPort
	needsValkey := false
	for resType, envVar := range valkeyResourceTypes {
		if resourceTypes[resType] {
			needsValkey = true
			runtimeEnvVars[envVar] = valkeyURL
		}
	}

	if !needsValkey {
		return
	}

	runtimeEnvVars["CELERITY_REDIS_ENDPOINT"] = valkeyURL
	ensureValkeyService(services, portOffset)
}

func ensureValkeyService(services map[string]*ComposeService, portOffset int) {
	if _, ok := services[ServiceNameValkey]; ok {
		return
	}
	services[ServiceNameValkey] = &ComposeService{
		Image: valkeyImage,
		Ports: []string{offsetPort(valkeyPort, portOffset) + ":" + valkeyPort},
		HealthCheck: &ComposeHealth{
			Test:        []string{"CMD", "valkey-cli", "ping"},
			Interval:    "5s",
			Timeout:     "3s",
			Retries:     3,
			StartPeriod: "5s",
		},
	}
}

// When localAuth is false (--no-local-auth), the service is still added
// for convenience but the blueprint issuer is not patched by the resolver.
func addDevAuthServiceIfNeeded(
	services map[string]*ComposeService,
	runtimeEnvVars map[string]string,
	bp *schema.Blueprint,
	portOffset int,
	localAuth bool,
) {
	if !hasJWTAuth(bp) {
		return
	}

	hostPort := offsetPort(devAuthPort, portOffset)
	issuer := "http://host.docker.internal:" + hostPort

	services[ServiceNameDevAuth] = &ComposeService{
		Image: devAuthImage,
		Ports: []string{hostPort + ":" + devAuthPort},
		Environment: map[string]string{
			"PORT":              devAuthPort,
			"DEV_AUTH_ISSUER":   issuer,
			"DEV_AUTH_AUDIENCE": resolveJWTAudience(bp),
			"LOG_LEVEL":         "info",
		},
		HealthCheck: &ComposeHealth{
			Test:        []string{"CMD-SHELL", "curl -sf http://localhost:" + devAuthPort + "/health || exit 1"},
			Interval:    "5s",
			Timeout:     "3s",
			Retries:     3,
			StartPeriod: "5s",
		},
	}

	runtimeEnvVars[EnvDevAuthBaseURL] = "http://" + ServiceNameDevAuth + ":" + devAuthPort
	// Store localAuth flag and issuer for the resolver to use when patching
	// the merged blueprint.
	if localAuth {
		runtimeEnvVars["CELERITY_DEV_AUTH_ISSUER"] = issuer
	}
}

func hasJWTAuth(bp *schema.Blueprint) bool {
	return findFirstJWTGuard(bp) != nil
}

func resolveJWTAudience(bp *schema.Blueprint) string {
	guard := findFirstJWTGuard(bp)
	if guard == nil {
		return "celerity-test-app"
	}

	audNode := guard.Fields["audience"]
	if audNode == nil {
		return "celerity-test-app"
	}
	// audience can be a scalar string or a sequence.
	if audNode.Scalar != nil {
		return audNode.Scalar.ToString()
	}
	if len(audNode.Items) > 0 && audNode.Items[0].Scalar != nil {
		return audNode.Items[0].Scalar.ToString()
	}
	return "celerity-test-app"
}

func findFirstJWTGuard(bp *schema.Blueprint) *core.MappingNode {
	if bp.Resources == nil {
		return nil
	}
	for _, resource := range bp.Resources.Values {
		if resource.Type == nil || resource.Type.Value != ResourceTypeAPI {
			continue
		}
		guard := findJWTGuardInResource(resource)
		if guard != nil {
			return guard
		}
	}
	return nil
}

func findJWTGuardInResource(resource *schema.Resource) *core.MappingNode {
	if resource.Spec == nil || resource.Spec.Fields == nil {
		return nil
	}
	authNode := resource.Spec.Fields["auth"]
	if authNode == nil || authNode.Fields == nil {
		return nil
	}
	guardsNode := authNode.Fields["guards"]
	if guardsNode == nil || guardsNode.Fields == nil {
		return nil
	}
	for _, guardNode := range guardsNode.Fields {
		if guardNode == nil || guardNode.Fields == nil {
			continue
		}
		if core.StringValue(guardNode.Fields["type"]) == "jwt" {
			return guardNode
		}
	}
	return nil
}

func addStubsServiceIfNeeded(
	services map[string]*ComposeService,
	runtimeEnvVars map[string]string,
	appDir string,
	portOffset int,
	logger *zap.Logger,
) ([]StubServiceInfo, error) {
	loaded, err := devstubs.LoadStubs(appDir)
	if err != nil {
		return nil, fmt.Errorf("loading stubs: %w", err)
	}
	if len(loaded) == 0 {
		return nil, nil
	}

	if errs := devstubs.ValidateStubs(loaded); len(errs) > 0 {
		for _, e := range errs {
			logger.Error("stub validation error", zap.String("detail", e.Error()))
		}
		return nil, fmt.Errorf("stub validation failed with %d error(s)", len(errs))
	}

	// Collect all imposter ports and build the port mapping list.
	ports := []string{
		offsetPort(mountebankAPIPort, portOffset) + ":" + mountebankAPIPort,
	}
	var stubInfos []StubServiceInfo
	for _, svc := range loaded {
		imposterPort := strconv.Itoa(svc.Config.Port)
		ports = append(ports, offsetPort(imposterPort, portOffset)+":"+imposterPort)

		// Set per-service URL env var (Docker network address for the app container).
		envKey := "CELERITY_STUB_" + toEnvName(svc.Name) + "_URL"
		runtimeEnvVars[envKey] = fmt.Sprintf("http://%s:%d", ServiceNameStubs, svc.Config.Port)

		stubInfos = append(stubInfos, StubServiceInfo{
			Name:            svc.Name,
			Port:            svc.Config.Port,
			HostPort:        svc.Config.Port + portOffset,
			ConfigKey:       svc.Config.ConfigKey,
			ConfigNamespace: svc.Config.ConfigNamespace,
		})
	}

	services[ServiceNameStubs] = &ComposeService{
		Image:   mountebankImage,
		Command: []string{"--loglevel", "debug"},
		Ports:   ports,
		HealthCheck: &ComposeHealth{
			Test:        []string{"CMD-SHELL", "wget -q -O /dev/null http://localhost:" + mountebankAPIPort + "/ || exit 1"},
			Interval:    "5s",
			Timeout:     "3s",
			Retries:     3,
			StartPeriod: "5s",
		},
	}

	runtimeEnvVars[EnvStubsAPIURL] = "http://" + ServiceNameStubs + ":" + mountebankAPIPort

	logger.Debug("added mountebank stubs service",
		zap.Int("services", len(loaded)),
		zap.Int("ports", len(ports)),
	)

	return stubInfos, nil
}

func toEnvName(name string) string {
	return strings.ToUpper(strings.ReplaceAll(name, "-", "_"))
}

func formatPortsWithOffset(ports []PortMapping, portOffset int) []string {
	result := make([]string, len(ports))
	for i, p := range ports {
		result[i] = offsetPort(p.Host, portOffset) + ":" + p.Container
	}
	return result
}

func offsetPort(port string, portOffset int) string {
	if portOffset == 0 {
		return port
	}
	p, err := strconv.Atoi(port)
	if err != nil {
		return port
	}
	return strconv.Itoa(p + portOffset)
}

// serviceHostReplacements maps compose service name patterns to their
// localhost equivalents. Most use "://service:" but PostgreSQL uses "@service:".
var serviceHostReplacements = []struct{ from, to string }{
	{"://" + ServiceNameDatastore + ":", "://localhost:"},
	{"://" + ServiceNameStorage + ":", "://localhost:"},
	{"://" + ServiceNameValkey + ":", "://localhost:"},
	{"://" + ServiceNameDevAuth + ":", "://localhost:"},
	{"://" + ServiceNameStubs + ":", "://localhost:"},
	{"@" + ServiceNameSqlDatabase + ":", "@localhost:"},
}

var knownContainerPorts = []string{
	dynamoDBLocalPort, minioPort, minioAPIPort,
	postgresPort, valkeyPort, devAuthPort, mountebankAPIPort,
}

// buildHostEnvVars rewrites RuntimeEnvVars (compose service names + container ports)
// into host-accessible endpoints (localhost + offset ports).
func buildHostEnvVars(runtimeEnvVars map[string]string, portOffset int) map[string]string {
	host := make(map[string]string, len(runtimeEnvVars))
	for k, v := range runtimeEnvVars {
		rewritten := v
		for _, r := range serviceHostReplacements {
			rewritten = strings.ReplaceAll(rewritten, r.from, r.to)
		}
		if portOffset != 0 {
			for _, port := range knownContainerPorts {
				rewritten = offsetPortInURL(rewritten, port, portOffset)
			}
		}
		host[k] = rewritten
	}
	return host
}

func offsetPortInURL(url string, containerPort string, portOffset int) string {
	hostPort := offsetPort(containerPort, portOffset)
	return strings.ReplaceAll(url, ":"+containerPort, ":"+hostPort)
}

func toComposeHealth(hc *HealthCheck) *ComposeHealth {
	return &ComposeHealth{
		Test:        hc.Test,
		Interval:    hc.Interval,
		Timeout:     hc.Timeout,
		Retries:     hc.Retries,
		StartPeriod: hc.StartPeriod,
	}
}

type bridgeConfig struct {
	Type      string              `json:"type"`
	Schedules []scheduleEntry     `json:"schedules,omitempty"`
	Source    any                 `json:"source,omitempty"`
	Target    any                 `json:"target,omitempty"`
	Targets   []topicBridgeTarget `json:"targets,omitempty"`
}

type topicBridgeSource struct {
	Channel string `json:"channel"`
}

type topicBridgeTarget struct {
	Stream string `json:"stream"`
}

type dynamoDBStreamSource struct {
	Endpoint  string `json:"endpoint"`
	Region    string `json:"region"`
	TableName string `json:"tableName"`
}

type minIONotificationSource struct {
	Endpoint  string   `json:"endpoint"`
	AccessKey string   `json:"accessKey"`
	SecretKey string   `json:"secretKey"`
	Bucket    string   `json:"bucketName"`
	Events    []string `json:"events"`
}

type streamTarget struct {
	Stream string `json:"stream"`
}

type scheduleEntry struct {
	ID       string `json:"id"`
	Schedule string `json:"schedule"`
	Stream   string `json:"stream"`
	Input    any    `json:"input"`
}

func addLocalEventsServiceIfNeeded(
	services map[string]*ComposeService,
	bp *schema.Blueprint,
	deployTarget string,
	appDir string,
	portOffset int,
) (map[string]bool, error) {
	bridges, streamEnabledTables := collectLocalEventsBridges(bp, deployTarget)
	if len(bridges) == 0 {
		return streamEnabledTables, nil
	}

	configPath := filepath.Join(appDir, ".celerity", "local-events-config.json")
	if err := writeLocalEventsConfigFile(configPath, bridges); err != nil {
		return nil, err
	}

	ensureValkeyService(services, portOffset)

	deps := map[string]ServiceDependency{
		ServiceNameValkey: {Condition: "service_healthy"},
	}
	if hasBridgeType(bridges, "dynamodb_stream") {
		deps[ServiceNameDatastore] = ServiceDependency{Condition: "service_healthy"}
	}
	if hasBridgeType(bridges, "minio_notification") {
		deps[ServiceNameStorage] = ServiceDependency{Condition: "service_healthy"}
	}

	services[ServiceNameLocalEvents] = &ComposeService{
		Image: localEventsImage,
		Environment: map[string]string{
			"CELERITY_LOCAL_REDIS_URL": "redis://" + ServiceNameValkey + ":" + valkeyPort,
			"LOG_LEVEL":                "debug",
		},
		Volumes: []string{
			configPath + ":/etc/celerity/local-events-config.json:ro",
		},
		DependsOn: deps,
	}
	return streamEnabledTables, nil
}

func hasBridgeType(bridges []bridgeConfig, bridgeType string) bool {
	for _, b := range bridges {
		if b.Type == bridgeType {
			return true
		}
	}
	return false
}

func writeLocalEventsConfigFile(path string, bridges []bridgeConfig) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating directory for local events config: %w", err)
	}

	data, err := json.MarshalIndent(bridges, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling local events config: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing local events config %s: %w", path, err)
	}

	return nil
}

func collectLocalEventsBridges(bp *schema.Blueprint, deployTarget string) ([]bridgeConfig, map[string]bool) {
	var bridges []bridgeConfig

	if schedBridge := collectScheduleBridgeConfig(bp); schedBridge != nil {
		bridges = append(bridges, *schedBridge)
	}

	bridges = append(bridges, collectTopicBridgeConfigs(bp)...)

	datastoreBridges, streamEnabledTables := collectDatastoreBridgeConfigs(bp, deployTarget)
	bridges = append(bridges, datastoreBridges...)

	bridges = append(bridges, collectBucketBridgeConfigs(bp)...)

	return bridges, streamEnabledTables
}

const celerityTopicPrefix = "celerity::topic::"

func collectTopicBridgeConfigs(bp *schema.Blueprint) []bridgeConfig {
	if bp.Resources == nil {
		return nil
	}

	// Group consumer resource names by topic name.
	// topicName -> []consumerResourceName
	topicConsumers := map[string][]string{}

	for name, resource := range bp.Resources.Values {
		if resource.Type == nil || resource.Type.Value != ResourceTypeConsumer {
			continue
		}
		sourceID := specStringFieldFromNode(resource.Spec, "sourceId")
		if !strings.HasPrefix(sourceID, celerityTopicPrefix) {
			continue
		}
		topicName := strings.TrimPrefix(sourceID, celerityTopicPrefix)
		topicConsumers[topicName] = append(topicConsumers[topicName], name)
	}

	var bridges []bridgeConfig
	for topicName, consumers := range topicConsumers {
		var targets []topicBridgeTarget
		for _, consumerName := range consumers {
			targets = append(targets, topicBridgeTarget{
				Stream: fmt.Sprintf("celerity:topic:%s:%s", topicName, consumerName),
			})
		}

		bridges = append(bridges, bridgeConfig{
			Type: "topic_bridge",
			Source: &topicBridgeSource{
				Channel: fmt.Sprintf("celerity:topic:channel:%s", topicName),
			},
			Targets: targets,
		})
	}

	return bridges
}

func collectScheduleBridgeConfig(bp *schema.Blueprint) *bridgeConfig {
	if bp.Resources == nil {
		return nil
	}

	var schedules []scheduleEntry
	for name, resource := range bp.Resources.Values {
		if resource.Type == nil || resource.Type.Value != ResourceTypeSchedule {
			continue
		}
		scheduleExpr := specStringFieldFromNode(resource.Spec, "schedule")
		if scheduleExpr == "" {
			continue
		}

		schedules = append(schedules, scheduleEntry{
			ID:       name,
			Schedule: scheduleExpr,
			Stream:   fmt.Sprintf("celerity:schedules:%s", name),
			Input:    extractSpecJSONValue(resource.Spec, "input"),
		})
	}

	if len(schedules) == 0 {
		return nil
	}

	return &bridgeConfig{
		Type:      "schedule",
		Schedules: schedules,
	}
}

func collectDatastoreBridgeConfigs(
	bp *schema.Blueprint,
	deployTarget string,
) ([]bridgeConfig, map[string]bool) {
	if bp.Resources == nil {
		return nil, nil
	}

	// Only AWS targets use DynamoDB Local — other providers are not yet supported.
	if deployTarget != DeployTargetAWS && deployTarget != DeployTargetAWSServerless {
		return nil, nil
	}

	// Collect unique datastore names that have at least one consumer linked from them.
	streamEnabledTables := map[string]bool{}
	for _, resource := range bp.Resources.Values {
		if resource.Type == nil || resource.Type.Value != ResourceTypeConsumer {
			continue
		}

		linked := findResourcesLinkingTo(resource, bp, ResourceTypeDatastore)
		for datastoreName := range linked {
			streamEnabledTables[datastoreName] = true
		}
	}

	var bridges []bridgeConfig
	for datastoreName := range streamEnabledTables {
		// The DynamoDB table name comes from spec.name (infrastructure name),
		// but the target stream must use the blueprint resource name to match
		// the Core runtime's stream naming convention.
		tableName := datastoreName
		if res, ok := bp.Resources.Values[datastoreName]; ok {
			tableName = resolveInfraName(res, datastoreName)
		}
		bridges = append(bridges, bridgeConfig{
			Type: "dynamodb_stream",
			Source: dynamoDBStreamSource{
				Endpoint:  "http://" + ServiceNameDatastore + ":" + dynamoDBLocalPort,
				Region:    "local",
				TableName: tableName,
			},
			Target: streamTarget{
				Stream: fmt.Sprintf("celerity:datastore:%s", datastoreName),
			},
		})
	}

	return bridges, streamEnabledTables
}

type bucketInfo struct {
	resourceName string
	infraName    string
	events       map[string]bool
}

func collectBucketBridgeConfigs(bp *schema.Blueprint) []bridgeConfig {
	if bp.Resources == nil {
		return nil
	}

	buckets := collectBucketConsumerEvents(bp)

	var bridges []bridgeConfig
	for _, info := range buckets {
		bridges = append(bridges, bucketBridgeFromInfo(info))
	}
	return bridges
}

func collectBucketConsumerEvents(bp *schema.Blueprint) map[string]*bucketInfo {
	buckets := map[string]*bucketInfo{}
	for _, resource := range bp.Resources.Values {
		if resource.Type == nil || resource.Type.Value != ResourceTypeConsumer {
			continue
		}
		linked := findResourcesLinkingTo(resource, bp, ResourceTypeBucket)
		for name, bucketResource := range linked {
			info, ok := buckets[name]
			if !ok {
				info = &bucketInfo{
					resourceName: name,
					infraName:    resolveInfraName(bucketResource, name),
					events:       map[string]bool{},
				}
				buckets[name] = info
			}
			eventsStr := annotationValueFromNode(resource, "celerity.consumer.bucket.events")
			if eventsStr == "" {
				continue
			}
			info.events = extractBucketEvents(eventsStr)
		}
	}
	return buckets
}

func extractBucketEvents(eventsStr string) map[string]bool {
	events := map[string]bool{}

	for e := range strings.SplitSeq(eventsStr, ",") {
		if trimmed := strings.TrimSpace(e); trimmed != "" {
			events[trimmed] = true
		}
	}

	return events
}

var defaultBucketEvents = []string{"s3:ObjectCreated:*", "s3:ObjectRemoved:*"}

func bucketBridgeFromInfo(info *bucketInfo) bridgeConfig {
	events := make([]string, 0, len(info.events))
	for e := range info.events {
		events = append(events, mapBucketEventToS3(e))
	}
	if len(events) == 0 {
		events = defaultBucketEvents
	}

	return bridgeConfig{
		Type: "minio_notification",
		Source: minIONotificationSource{
			Endpoint:  "http://" + ServiceNameStorage + ":" + minioPort,
			AccessKey: defaultMinioCreds.AccessKey,
			SecretKey: defaultMinioCreds.SecretKey,
			Bucket:    info.infraName,
			Events:    events,
		},
		Target: streamTarget{
			Stream: fmt.Sprintf("celerity:bucket:%s", info.resourceName),
		},
	}
}

var bucketEventToS3 = map[string]string{
	"created":         "s3:ObjectCreated:*",
	"deleted":         "s3:ObjectRemoved:*",
	"metadataUpdated": "s3:ObjectTagging:*",
}

func mapBucketEventToS3(event string) string {
	if s3Event, ok := bucketEventToS3[event]; ok {
		return s3Event
	}
	return event
}

func findResourcesLinkingTo(
	targetResource *schema.Resource,
	bp *schema.Blueprint,
	sourceType string,
) map[string]*schema.Resource {
	targetLabels := resourceLabels(targetResource)
	if len(targetLabels) == 0 {
		return nil
	}

	result := map[string]*schema.Resource{}
	for name, resource := range bp.Resources.Values {
		if resource.Type == nil || resource.Type.Value != sourceType {
			continue
		}
		selectorLabels := linkSelectorLabels(resource)
		if len(selectorLabels) == 0 {
			continue
		}
		if labelsMatch(selectorLabels, targetLabels) {
			result[name] = resource
		}
	}
	return result
}

func resourceLabels(r *schema.Resource) map[string]string {
	if r.Metadata == nil || r.Metadata.Labels == nil {
		return nil
	}
	return r.Metadata.Labels.Values
}

func linkSelectorLabels(r *schema.Resource) map[string]string {
	if r.LinkSelector == nil || r.LinkSelector.ByLabel == nil {
		return nil
	}
	return r.LinkSelector.ByLabel.Values
}

func labelsMatch(selector, labels map[string]string) bool {
	for k, v := range selector {
		if labels[k] != v {
			return false
		}
	}
	return true
}

func resolveInfraName(resource *schema.Resource, resourceName string) string {
	if resource.Spec != nil {
		if name := specStringFieldFromNode(resource.Spec, "name"); name != "" {
			return name
		}
	}
	return resourceName
}

func annotationValueFromNode(resource *schema.Resource, key string) string {
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

func specStringFieldFromNode(spec *core.MappingNode, field string) string {
	if spec == nil || spec.Fields == nil {
		return ""
	}
	return core.StringValue(spec.Fields[field])
}

func extractSpecJSONValue(spec *core.MappingNode, field string) any {
	if spec == nil || spec.Fields == nil {
		return nil
	}
	node := spec.Fields[field]
	if node == nil {
		return nil
	}
	if node.Scalar != nil {
		return node.Scalar.ToString()
	}
	// For complex values, marshal and re-parse to get a plain any.
	data, err := json.Marshal(node)
	if err != nil {
		return nil
	}
	var result any
	_ = json.Unmarshal(data, &result)
	return result
}

func writeComposeFile(path string, services map[string]*ComposeService) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating directory for compose file: %w", err)
	}

	file := composeFile{Services: services}
	data, err := yaml.Marshal(&file)
	if err != nil {
		return fmt.Errorf("marshalling compose config: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing compose file %s: %w", path, err)
	}

	return nil
}
