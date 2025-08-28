from typing import Optional, Callable, Awaitable, Any
from datetime import datetime
from enum import Enum


class RuntimePlatform(Enum):
    """
    The platform the application is running on.
    """
    AWS = 0
    AZURE = 1
    GCP = 2
    LOCAL = 3
    OTHER = 4


class CoreRuntimeConfig:
    """
    Configuration for running an application in the Celerity runtime.

    Attributes:
        blueprint_config_path: Path to the blueprint configuration file.
        service_name: The name of the service that will be used
                      for tracing and logging.
        server_port: Port to run the server on.
        server_loopback_only: Whether to only listen on localhost, defaults to True.
        use_custom_health_check: Whether to use a custom health check endpoint, defaults to False.
        trace_otlp_collector_endpoint: The endpoint to send trace data to, defaults to "http://otelcollector:4317".
        runtime_max_diagnostics_level: The maximum diagnostics level to use for logging and tracing, defaults to "info".
        platform: The platform the application is running on, defaults to CoreRuntimePlatform.OTHER.
        test_mode: Whether the runtime is running in test mode (e.g. integration tests).
        api_resource: The name of the API resource in the blueprint that should be used as the configuration source
                      for setting up API configuration and endpoints.
        consumer_app: The name of the consumer app in the blueprint that should be used as the configuration source
                      for setting up webhook endpoints (for push model message sources) or a polling consumer
                      (for pull model message sources).
                      This will either be a shared `celerity.app` annotation shared by multiple consumers
                      that are part of the same application or the name of an individual `celerity/consumer`
                      resource in the blueprint.
                      If not set, the runtime will use the first `celerity/consumer` resource defined
                      in the blueprint.
        schedule_app: The name of the schedule app in the blueprint that should be used as the configuration source
                      for setting up a polling consumer or webhook endpoint specifically for scheduled messages.
                      This will be either a shared `celerity.app` annotation shared by multiple schedules
                      that are part of the same application or the name of an individual `celerity/schedule`
                      resource in the blueprint.
                      If not set, the runtime will use the first `celerity/schedule` resource defined
                      in the blueprint.
        resource_store_verify_tls: Whether to verify TLS certificates when making requests to the resource store
                                   for requesting resources such as OpenID discovery documents and JSON Web Key Sets for
                                   JWT authentication.
                                   This must be true for any production environment, and can be set to false for
                                   development environments with self-signed certificates. This defaults to True.
        resource_store_cache_entry_ttl: The TTL in seconds for cache entries in the resource store.
                                        This defaults to 600 seconds (10 minutes).
        resource_store_cleanup_interval: The interval in seconds at which the resource store cleanup task should run.
                                         This defaults to 3600 seconds (1 hour).
    """

    blueprint_config_path: str
    service_name: str
    server_port: int
    server_loopback_only: Optional[bool]
    use_custom_health_check: Optional[bool]
    trace_otlp_collector_endpoint: str
    runtime_max_diagnostics_level: str
    platform: RuntimePlatform
    test_mode: bool
    api_resource: Optional[str]
    consumer_app: Optional[str]
    schedule_app: Optional[str]
    resource_store_verify_tls: bool
    resource_store_cache_entry_ttl: int
    resource_store_cleanup_interval: int

    def __init__(
        self,
        blueprint_config_path: str,
        service_name: str,
        server_port: int,
    ):
        """
        Initialises a new runtime configuration.

        Args:
            blueprint_config_path: Path to the blueprint configuration file.
            service_name: The name of the service that will be used
                          for tracing and logging.
            server_port: Port to run the server on.
        """

    @staticmethod
    def from_env() -> "CoreRuntimeConfig":
        """
        Creates a new runtime configuration from the current process environment variables.

        Returns:
            A CoreRuntimeConfig instance derived from the current environment
            that can be used to instantiate an application.
        """


class CoreRuntimeConfigBuilder:
    """
    Builder for manually creating core runtime configuration
    that can be used to instantiate an application.

    Methods:

    """

    def __init__(self, blueprint_config_path: str, service_name: str, server_port: int) -> None:
        """
        Initialises a new builder with required configuration
        elements.

        Args:
            blueprint_config_path: Path to the blueprint configuration file.
            service_name: The name of the service that will be used
                          for tracing and logging.
            server_port: Port to run the server on.
        """

    def set_server_loopback_only(self, server_loopback_only: bool) -> "CoreRuntimeConfigBuilder":
        """
        Set this to determine whether the HTTP/WebSocket server should only be exposed
        on the loopback interface (127.0.0.1).

        When running in an environment such as a docker container,
        this should be set to false so that the server can be accessed from
        outside the container.

        Defaults to True when not set.

        Args:
            server_loopback_only: Whether to only listen on localhost.

        Returns:
            A CoreRuntimeConfigBuilder instance with the configuration set.
        """

    def set_use_custom_health_check(self, use_custom_health_check: bool) -> "CoreRuntimeConfigBuilder":
        """
        Set this to determine whether the runtime should use a custom health check endpoint.

        Defaults to False when not set.

        The `GET /runtime/health/check` endpoint is set by the runtime
        to return a 200 OK status code when this is set to False.
        The default health check is not accessible under custom base paths
        defined for an API, and is only accessible from the root path.
        The health check endpoint exists to be called directly by a
        container/machine orchestrator service that has direct access
        to the instance of the runtime API via the exposed container port.

        Args:
            use_custom_health_check: Whether to use a custom health check endpoint.

        Returns:
            A CoreRuntimeConfigBuilder instance with the configuration set.
        """

    def set_trace_otlp_collector_endpoint(self, trace_otlp_collector_endpoint: str) -> "CoreRuntimeConfigBuilder":
        """
        Set this to determine the endpoint to be used for sending trace data to an OTLP collector.

        Defaults to "http://otelcollector:4317" when not set.

        The default value assumes the common use case of running the OpenTelemetry Collector
        in a sidecar container named "otelcollector" in the same container network as the runtime.

        Args:
            trace_otlp_collector_endpoint: The endpoint to send trace data to.

        Returns:
            A CoreRuntimeConfigBuilder instance with the configuration set.
        """

    def set_runtime_max_diagnostics_level(self, runtime_max_diagnostics_level: str) -> "CoreRuntimeConfigBuilder":
        """
        Set this to determine the maximum diagnostics level that the runtime should use for logging and tracing.

        This is used to control the verbosity of exported/captured traces and events
        in the runtime.

        Defaults to "info" when not set.

        Args:
            runtime_max_diagnostics_level: The maximum diagnostics level to use for logging and tracing.

        Returns:
            A CoreRuntimeConfigBuilder instance with the configuration set.
        """

    def set_platform(self, platform: RuntimePlatform) -> "CoreRuntimeConfigBuilder":
        """
        Set this to determine the platform the application is running on.

        Defaults to CoreRuntimePlatform.OTHER when not set.

        This is essential in determining which features are available in the current environment.
        For example, if the runtime platform is AWS, the runtime can set up telemetry to use an
        AWS X-Ray propagator to enrich traces and events with AWS-specific trace IDs.

        Args:
            platform: The platform the application is running on.

        Returns:
            A CoreRuntimeConfigBuilder instance with the configuration set.
        """

    def set_test_mode(self, test_mode: bool) -> "CoreRuntimeConfigBuilder":
        """
        Set this to determine whether the runtime is running in test mode (e.g. integration tests).

        Defaults to False when not set.

        Args:
            test_mode: Whether the runtime is running in test mode.

        Returns:
            A CoreRuntimeConfigBuilder instance with the configuration set.
        """

    def set_api_resource(self, api_resource: str) -> "CoreRuntimeConfigBuilder":
        """
        Set this to determine the name of the API resource in the blueprint that should be
        used as the configuration source for setting up API configuration and endpoints.

        Args:
            api_resource: The name of the API resource in the blueprint.

        Returns:
            A CoreRuntimeConfigBuilder instance with the configuration set.
        """

    def set_consumer_app(self, consumer_app: str) -> "CoreRuntimeConfigBuilder":
        """
        Set this to determine the name of the consumer app in the blueprint that should be
        used as the configuration source for setting up webhook endpoints (for push model message sources)
        or a polling consumer (for pull model message sources).
        This will be either a shared `celerity.app` annotation shared by multiple consumers
        that are part of the same application or the name of an individual `celerity/consumer`
        resource in the blueprint.
        If not set, the runtime will use the first `celerity/consumer` resource defined
        in the blueprint.

        Args:
            consumer_app: The name of the consumer app in the blueprint.

        Returns:
            A CoreRuntimeConfigBuilder instance with the configuration set.
        """

    def set_schedule_app(self, schedule_app: str) -> "CoreRuntimeConfigBuilder":
        """
        Set this to determine the name of the schedule app in the blueprint that should be
        used as the configuration source for setting up a polling consumer or webhook endpoint
        specifically for scheduled messages.

        This will be either a shared `celerity.app` annotation shared by multiple schedules
        that are part of the same application or the name of an individual `celerity/schedule`
        resource in the blueprint.
        If not set, the runtime will use the first `celerity/schedule` resource defined
        in the blueprint.

        Args:
            schedule_app: The name of the schedule app in the blueprint.

        Returns:
            A CoreRuntimeConfigBuilder instance with the configuration set.
        """

    def set_resource_store_verify_tls(self, resource_store_verify_tls: bool) -> "CoreRuntimeConfigBuilder":
        """
        Set this to determine whether to verify TLS certificates when making requests to the resource store
        for requesting resources such as OpenID discovery documents and JSON Web Key Sets for JWT authentication.

        This must be true for any production environment, and can be set to false for
        development environments with self-signed certificates. This defaults to True.

        Args:
            resource_store_verify_tls: Whether to verify TLS certificates when making requests to the resource store.

        Returns:
            A CoreRuntimeConfigBuilder instance with the configuration set.
        """

    def set_resource_store_cache_entry_ttl(self, resource_store_cache_entry_ttl: int) -> "CoreRuntimeConfigBuilder":
        """
        Set this to determine the TTL in seconds for cache entries in the resource store.

        This defaults to 600 seconds (10 minutes).

        Args:
            resource_store_cache_entry_ttl: The TTL in seconds for cache entries in the resource store.

        Returns:
            A CoreRuntimeConfigBuilder instance with the configuration set.
        """

    def set_resource_store_cleanup_interval(self, resource_store_cleanup_interval: int) -> "CoreRuntimeConfigBuilder":
        """
        Set this to determine the interval in seconds at which the resource store cleanup task should run.

        This defaults to 3600 seconds (1 hour).

        Args:
            resource_store_cleanup_interval: The interval in seconds at which the
                                             resource store cleanup task should run.

        Returns:
            A CoreRuntimeConfigBuilder instance with the configuration set.
        """

    def build(self) -> CoreRuntimeConfig:
        """
        Builds a new runtime configuration object from the configuration set
        with the builder.

        Returns:
            A CoreRuntimeConfig instance that can be used to instantiate an application.
        """


class CoreHttpHandlerDefinition:
    """
    Definition of an HTTP handler.

    Attributes:
        path: The path of the handler.
        method: The HTTP method of the handler.
        location: The location of the handler.
        handler: The handler function.
        timeout: The timeout in seconds for the handler.
    """

    path: str
    method: str
    location: str
    handler: str
    timeout: int


class CoreHttpConfig:
    """
    Configuration for HTTP endpoint handlers.

    Attributes:
        handlers: List of HTTP handler definitions.
    """

    handlers: list[CoreHttpHandlerDefinition]


class CoreWebSocketHandlerDefinition:
    """
    Definition of a WebSocket handler.

    Attributes:
        route: The route of the handler.
        location: The location of the handler.
        handler: The handler function.
        timeout: The timeout in seconds for the handler.
    """

    route: str
    location: str
    handler: str
    timeout: int


class CoreWebSocketConfig:
    """
    Configuration for WebSocket connection lifecycle and message handlers.

    Attributes:
        handlers: List of WebSocket handler definitions.
    """
    handlers: list[CoreWebSocketHandlerDefinition]


class CoreApiConfig:
    """
    Configuration for HTTP and WebSocket APIs.

    Attributes:
        http: Configuration for HTTP endpoints.
        websocket: Configuration for WebSocket connection lifecycle and message handlers.
    """
    http: Optional[CoreHttpConfig]
    websocket: Optional[CoreWebSocketConfig]


class CoreRuntimeAppConfig:
    """
    Configuration for the application running in the Celerity runtime.

    Attributes:
        api: Configuration for HTTP and WebSocket APIs.
    """
    api: Optional[CoreApiConfig]


class Response:
    """
    HTTP response to return to the client.

    Attributes:
        status: HTTP status code to return to the client.
        headers: Optional headers to return to the client.
        body: Optional response body to return to the client.
    """

    status: int
    headers: Optional[dict[str, str]]
    text_body: Optional[str]
    binary_body: Optional[bytes]


class ResponseBuilder:
    """
    Builder for creating HTTP responses.

    Methods:
        set_status: Set the HTTP status code to return to the client.
        set_headers: Set the HTTP headers to return to the client.
        set_text_body: Set the text body to return to the client.
        set_binary_body: Set the binary body to return to the client.
        build: Build the response.
    """

    def __init__(self) -> None:
        """
        Initialises a new builder.
        """

    def set_status(self, status: int) -> "ResponseBuilder":
        """
        Set the HTTP status code to return to the client.
        If not set, the default status code is 200 OK.

        Args:
            status: The HTTP status code to return to the client.

        Returns:
            A ResponseBuilder instance with the status code set.
        """

    def set_headers(self, headers: dict[str, str]) -> "ResponseBuilder":
        """
        Set the HTTP headers to return to the client.

        Args:
            headers: The HTTP headers to return to the client.

        Returns:
            A ResponseBuilder instance with the headers set.
        """

    def set_text_body(self, text_body: str) -> "ResponseBuilder":
        """
        Set the text body to return to the client.
        The `text/plain` content type will be set automatically
        when this is set.
        If the content type is set in the headers, that will take precedence.

        Args:
            text_body: The text body to return to the client.

        Returns:
            A ResponseBuilder instance with the text body set.
        """

    def set_json_body(self, json_body: Any) -> "ResponseBuilder":
        """
        Set the JSON body to return to the client.
        The `application/json` content type will be set automatically
        when this is set.
        If the content type is set in the headers, that will take precedence.

        Args:
            json_body: The JSON body to return to the client,
                       this must be a JSON serializable value.

        Returns:
            A ResponseBuilder instance with the JSON body set.
        """

    def set_binary_body(self, binary_body: bytes) -> "ResponseBuilder":
        """
        Set the binary body to return to the client.
        The `application/octet-stream` content type will be set automatically
        when this is set.
        If the content type is set in the headers, that will take precedence.

        Args:
            binary_body: The binary body to return to the client.

        Returns:
            A ResponseBuilder instance with the binary body set.
        """

    def build(self) -> Response:
        """
        Builds a new response object from the builder.

        Returns:
            A Response instance that can be returned to the client from a handler.

        Raises:
            Exception: If a JSON body is set and is not a JSON serializable value,
                       can be a generic exception, a ValueError or a TypeError.
        """


class WebSocketEventType(Enum):
    """
    The type of event that a handler receives for a WebSocket connection.
    """
    CONNECT = 0
    MESSAGE = 1
    DISCONNECT = 2


class HttpProtocolVersion(Enum):
    """
    The protocol version of an HTTP request.
    """
    HTTP1_1 = 0
    HTTP2 = 1
    HTTP3 = 2


class WebSocketMessageRequestContext:
    """
    Request context for a WebSocket message.
    This is the context of the initial HTTP request that was
    upgraded to a WebSocket connection.

    Attributes:
        request_id: The ID of the request.
        request_time: The time the request was received.
        path: The path of the request.
        protocol_version: The protocol of the original HTTP request.
        headers: A dictionary of HTTP headers.
        user_agent_header: The value of the User-Agent header.
        client_ip: The secure IP address of the client extracted from trusted
                   headers or directly from the socket when there are no proxies
                   in front of the runtime.
        query: A dictionary of query parameters.
        cookies: A dictionary of cookies.
        trace_context: A dictionary of trace context including a W3C Trace Context string
                       ( in the traceparent format) and platform specific trace IDs such as
                       an AWS X-Ray Trace ID.
    """

    request_id: str
    request_time: datetime
    path: str
    protocol_version: HttpProtocolVersion
    headers: dict[str, list[str]]
    user_agent_header: Optional[str]
    client_ip: str
    query: dict[str, list[str]]
    cookies: dict[str, str]
    trace_context: Optional[dict[str, str]]


class WebSocketMessageRequestContextBuilder:
    """
    Builder for creating WebSocket message request contexts.

    Methods:
        set_headers: Set the HTTP headers of the original request.
        set_user_agent: Set the user agent for the original request.
        set_client_ip: Set the client IP address for the original request.
        set_query: Set the query parameters for the original request.
        set_cookies: Set the cookies for the original request.
        set_trace_context: Set the trace context for the original request.
        build: Build the request context for a WebSocket message.
    """

    def __init__(
            self,
            request_id: str,
            request_time: datetime,
            path: str,
            protocol_version: HttpProtocolVersion,
    ):
        """
        Initialises a new builder.

        Args:
            request_id: The ID of the request.
            request_time: The time the request was received.
            path: The path of the request.
            protocol_version: The protocol version of the request.
        """

    def set_headers(self, headers: dict[str, list[str]]) -> "WebSocketMessageRequestContextBuilder":
        """
        Set the HTTP headers of the original request.
        An empty dictionary of headers will be used if not set.

        Args:
            headers: The HTTP headers of the original request.

        Returns:
            A WebSocketMessageRequestContextBuilder instance with the headers set.
        """

    def set_user_agent(self, user_agent: str) -> "WebSocketMessageRequestContextBuilder":
        """
        Set the user agent for the original request.
        An empty string will be used if not set.

        Args:
            user_agent: The user agent for the original request.

        Returns:
            A WebSocketMessageRequestContextBuilder instance with the user agent set.
        """

    def set_client_ip(self, client_ip: str) -> "WebSocketMessageRequestContextBuilder":
        """
        Set the client IP address for the original request.
        An empty string will be used if not set.

        Args:
            client_ip: The client IP address for the original request.

        Returns:
            A WebSocketMessageRequestContextBuilder instance with the client IP address set.
        """

    def set_query(self, query: dict[str, list[str]]) -> "WebSocketMessageRequestContextBuilder":
        """
        Set the query parameters for the original request.
        An empty dictionary of query parameters will be used if not set.

        Args:
            query: The query parameters for the original request.

        Returns:
            A WebSocketMessageRequestContextBuilder instance with the query parameters set.
        """

    def set_cookies(self, cookies: dict[str, str]) -> "WebSocketMessageRequestContextBuilder":
        """
        Set the cookies for the original request.
        An empty dictionary of cookies will be used if not set.

        Args:
            cookies: The cookies for the original request.

        Returns:
            A WebSocketMessageRequestContextBuilder instance with the cookies set.
        """

    def set_trace_context(self, trace_context: dict[str, str]) -> "WebSocketMessageRequestContextBuilder":
        """
        Set the trace context for the original request.
        An empty dictionary for a trace context will be used if not set.

        Args:
            trace_context: The trace context for the original request.

        Returns:
            A WebSocketMessageRequestContextBuilder instance with the trace context set.
        """

    def build(self) -> WebSocketMessageRequestContext:
        """
        Build the request context for a WebSocket message.

        Returns:
            A WebSocketMessageRequestContext instance.

        Raises:
            Exception: Unexpected error may occur if the request context
                       can't be built in the Rust-Python bridge.
        """


class WebSocketMessageInfo:
    """
    The handler input for when a WebSocket message is received.
    This includes context such as the message type and information about
    the original request upgraded to a WebSocket connection.

    Attributes:
        type: The type of the message.
        event_type: The type of event that the message is for .
        connection_id: The ID of the WebSocket connection that the message was received on.
        message_id: The ID of the message.
        json_body: The JSON body of the message, this will be set if the message type is `json`.
        binary_body: The binary body of the message, this will be set if the message type is `binary`.
        request_context: The context of the request that upgraded to a WebSocket connection.
        trace_context: A dictionary of trace context including a W3C Trace Context string
                       ( in the traceparent format) and platform specific trace IDs such as
                       an AWS X-Ray Trace ID.
    """

    type: "WebSocketMessageType"
    event_type: WebSocketEventType
    connection_id: str
    message_id: str
    json_body: Optional[Any]
    binary_body: Optional[bytes]
    request_context: Optional[WebSocketMessageRequestContext]
    trace_context: Optional[dict[str, str]]


class WebSocketMessageInfoBuilder:
    """
    Builder for creating WebSocket message info objects.

    Methods:
        set_json_body: Set the JSON body of the message.
        set_binary_body: Set the binary body of the message.
        set_request_context: Set the request context of the message.
        set_trace_context: Set the trace context of the message.
        build: Build the message info object.
    """

    def __init__(
        self,
        message_type: "WebSocketMessageType",
        event_type: WebSocketEventType,
        connection_id: str,
        message_id: str,
    ):
        """
        Initialises a new builder with required fields.

        Args:
            message_type: The type of the message.
            event_type: The type of event that the message is for .
            connection_id: The ID of the WebSocket connection that the message was received on.
            message_id: The ID of the message.
        """

    def set_json_body(self, json_body: Any) -> "WebSocketMessageInfoBuilder":
        """
        Set the JSON body of the message.
        This must only be set if the message type is `WebSocketMessageType.JSON`.

        Args:
            json_body: The JSON body of the message.

        Returns:
            A WebSocketMessageInfoBuilder instance with the JSON body set.
        """

    def set_binary_body(self, binary_body: bytes) -> "WebSocketMessageInfoBuilder":
        """
        Set the binary body of the message.
        This must only be set if the message type is `WebSocketMessageType.BINARY`.

        Args:
            binary_body: The binary body of the message.

        Returns:
            A WebSocketMessageInfoBuilder instance with the binary body set.
        """

    def set_request_context(self, request_context: WebSocketMessageRequestContext) -> "WebSocketMessageInfoBuilder":
        """
        Set the request context of the message.

        Args:
            request_context: The request context for the original HTTP request that
                             upgraded to a WebSocket connection.

        Returns:
            A WebSocketMessageInfoBuilder instance with the request context set.
        """

    def set_trace_context(self, trace_context: dict[str, str]) -> "WebSocketMessageInfoBuilder":
        """
        Set the trace context of the message.

        Args:
            trace_context: The trace context of the message.

        Returns:
            A WebSocketMessageInfoBuilder instance with the trace context set.
        """

    def build(self) -> WebSocketMessageInfo:
        """
        Build the message info object.

        Returns:
            A WebSocketMessageInfo instance.

        Raises:
            Exception: Unexpected error may occur if the message info
            can't be built in the Rust-Python bridge.
        """


class Request:
    """
    The handler input for when an HTTP request is received.

    Attributes:
        headers: A dictionary of HTTP headers, allowing for multiple values per header name.
        query: A dictionary of query parameters, allowing for multiple values per parameter name.
        text_body: The utf-8 encoded text body of the request.
        binary_body: The binary body of the request as bytes.
        content_type: The content type of the request.
        cookies: A dictionary of cookies, allowing for one value per cookie name.
        method: The HTTP method of the request.
        path: The path of the request.
        path_params: A dictionary of path parameters.
        protocol_version: The protocol version of the request.
    """
    text_body: Optional[str]
    binary_body: Optional[bytes]
    content_type: str
    headers: dict[str, list[str]]
    query: dict[str, list[str]]
    cookies: dict[str, str]
    method: str
    path: str
    path_params: dict[str, str]
    protocol_version: HttpProtocolVersion


class RequestBuilder:
    """
    Builder for creating Request objects.

    Methods:
        set_text_body: Set the text body of the request.
        set_binary_body: Set the binary body of the request.
        set_content_type: Set the content type of the request.
        set_headers: Set the HTTP headers of the request.
        set_query: Set the query parameters of the request.
        set_cookies: Set the cookies of the request.
        set_method: Set the HTTP method of the request.
        set_path: Set the path of the request.
        set_path_params: Set the path parameters of the request.
        set_protocol_version: Set the protocol version of the request.
        build: Build the request object.
    """

    def __init__(self):
        """
        Initialises a new request builder with an initial set
        of default values.
        """

    def set_text_body(self, text_body: str) -> "RequestBuilder":
        """
        Set the text body of the request.
        This will set the content type of the request to `text/plain`
        if it is not already set.

        Args:
            text_body: The text body of the request.

        Returns:
            A RequestBuilder instance with the text body set.
        """

    def set_binary_body(self, binary_body: bytes) -> "RequestBuilder":
        """
        Set the binary body of the request.
        This will set the content type of the request to `application/octet-stream`
        if it is not already set.

        Args:
            binary_body: The binary body of the request.

        Returns:
            A RequestBuilder instance with the binary body set.
        """

    def set_content_type(self, content_type: str) -> "RequestBuilder":
        """
        Set the content type of the request.
        The content type in headers will be used if not set,
        otherwise a default value will be used based on the body type.

        Args:
            content_type: The content type of the request.

        Returns:
            A RequestBuilder instance with the content type set.
        """

    def set_headers(self, headers: dict[str, list[str]]) -> "RequestBuilder":
        """
        Set the HTTP headers of the request.
        An empty dictionary of headers will be used if not set.

        Args:
            headers: The HTTP headers of the request.

        Returns:
            A RequestBuilder instance with the headers set.
        """

    def set_query(self, query: dict[str, list[str]]) -> "RequestBuilder":
        """
        Set the query parameters of the request.
        An empty dictionary of query parameters will be used if not set.

        Args:
            query: The query parameters of the request.

        Returns:
            A RequestBuilder instance with the query parameters set.
        """

    def set_cookies(self, cookies: dict[str, str]) -> "RequestBuilder":
        """
        Set the cookies of the request.
        An empty dictionary of cookies will be used if not set.

        Args:
            cookies: The cookies of the request.

        Returns:
            A RequestBuilder instance with the cookies set.
        """

    def set_method(self, method: str) -> "RequestBuilder":
        """
        Set the HTTP method of the request.
        `GET` will be used for the method if not set.

        Args:
            method: The HTTP method of the request.

        Returns:
            A RequestBuilder instance with the HTTP method set.
        """

    def set_path(self, path: str) -> "RequestBuilder":
        """
        Set the path of the request.
        `/` will be used for the path if not set.
        """

    def set_path_params(self, path_params: dict[str, str]) -> "RequestBuilder":
        """
        Set the path parameters of the request.
        An empty dictionary of path parameters will be used if not set.

        Args:
            path_params: The path parameters of the request.

        Returns:
            A RequestBuilder instance with the path parameters set.
        """

    def set_protocol_version(self, protocol_version: HttpProtocolVersion) -> "RequestBuilder":
        """
        Set the protocol version of the request.
        `HTTP1_1` will be used for the protocol version if not set.

        Args:
            protocol_version: The protocol version of the request.

        Returns:
            A RequestBuilder instance with the protocol version set.

        Raises:
            ValueError: If the protocol version is not a valid HTTP protocol version.
        """

    def build(self) -> Request:
        """
        Build the request object that can be passed into an HTTP handler.

        Returns:
            A Request instance.

        Raises:
            Exception: Unexpected error may occur if the request
                       can't be built in the Rust-Python bridge.
        """


class RequestContext:
    """
    Context for an HTTP request.

    Attributes:
        request_id: The ID of the request.
        request_time: The time the request was received.
        auth: Optional authentication information for the request that contains the result
              of an auth guard.
        trace_context: A dictionary of trace context including a W3C Trace Context string
                       (in the traceparent format) and platform specific trace IDs such as
                       an AWS X-Ray Trace ID.
    """
    request_id: str
    request_time: datetime
    auth: Optional[Any]
    trace_context: Optional[dict[str, str]]

    def __init__(
        self,
        request_id: str,
        request_time: datetime,
        auth: Optional[Any],
        trace_context: Optional[dict[str, str]],
    ):
        """
        Initialises a new request context.

        Args:
            request_id: The ID of the request.
            request_time: The time the request was received.
            auth: Optional authentication information for the request that contains the result
                  of an auth guard.
            trace_context: A dictionary of trace context including a W3C Trace Context string
                           (in the traceparent format) and platform specific trace IDs such as
                           an AWS X-Ray Trace ID.
        """


class SendContext:
    """
    Context for sending a message to a WebSocket connection.

    Attributes:
        caller: The name of the caller sending the message.
        wait_for_ack: Whether to wait for an acknowledgement from the client.
        inform_clients: List of client IDs to inform when a message is considered lost.
    """

    caller: Optional[str]
    wait_for_ack: bool
    inform_clients: list[str]

    def __init__(
        self,
        caller: Optional[str],
        wait_for_ack: bool,
        inform_clients: list[str],
    ):
        """
        Initialises a new send context.

        Args:
            caller: The name of the caller sending the message.
            wait_for_ack: Whether to wait for an acknowledgement from the client.
            inform_clients: List of client IDs to inform when a message is considered lost.
        """


class WebSocketMessageType(Enum):
    """
    The type of message to send to a WebSocket connection.
    """
    JSON = 0
    BINARY = 1


class WebSocketRegistry:
    """
    Registry for sending messages to WebSocket connections.

    Methods:
        send_message: Sends a message to a WebSocket connection.
    """

    def send_message(
        self,
        connection_id: str,
        message_id: str,
        message_type: WebSocketMessageType,
        message: str,
        ctx: Optional[SendContext],
    ) -> None:
        """
        Sends a message to a WebSocket connection that is either connected to the current
        node running the application or other nodes in a WebSocket API cluster.

        Args:
            connection_id: The ID of the WebSocket connection to send the message to.
            message_id: The ID of the message.
            message_type: The type of the message, this can be either JSON or binary.
                          The message will be a base64-encoded string if the message type is binary.
            message: The message to send.
            ctx: Optional context for the message.
        """


class CoreRuntimeApplication:
    """
    An application to run in the Celerity runtime.
    Depending on the blueprint configuration file, this can run HTTP APIs,
    WebSocket APIs and event source consumers for the current environment
    (e.g. SQS queue consumers when deployed to AWS).

    This class is not thread-safe, handlers will often run on multiple threads
    in the core runtime so therefore it is not safe to access an application
    instance directly from handler functions. When a handler needs to send messages
    to clients in a WebSocket API, a WebSocketRegistry instance should be retrieved
    from the application at the setup stage and made accessible to handlers. This also
    applies to other services, state or configuration that the application exposes.

    WebSocketRegistry instances returned from the `websocket_registry` method are
    thread-safe and can be used to send messages to clients in a WebSocket API
    from handler functions.

    Methods:
        setup: Sets up the application based on the configuration the application was
               instantiated with . This will return configuration such as HTTP handler
               definitions that should be used to register handlers.
        run: Runs the application including a HTTP/WebSocket server and event source consumers
             based on the configuration the application was instantiated with .
        register_http_handler: Registers a new HTTP handler with the application.
        register_websocket_handler: Registers a new WebSocket handler with the application.
        websocket_registry: Retrieves the WebSocket registry for the application
                            that allows sending messages to specific WebSocket connections
                            that are either connected to the current node running the application
                            or other nodes in a WebSocket API cluster.

    """

    def __init__(self, runtime_config: CoreRuntimeConfig):
        """
        Initialises a new application.

        Args:
            runtime_config: The configuration for the application including
                            the path of the blueprint configuration file.
        """

    def setup(self) -> CoreRuntimeAppConfig:
        """
        Sets up the application based on the configuration the application was
        instantiated with . This will return configuration such as HTTP handler
        definitions that should be used to register handlers.

        Returns:
            A CoreRuntimeAppConfig instance that can be used to register handlers
            with the application as defined in the blueprint file.
        """

    def run(self) -> None:
        """
        Runs the application including a HTTP/WebSocket server and event source consumers
        based on the configuration the application was instantiated with .

        This method will block until the application is stopped.
        """

    def register_http_handler(
        self,
        path: str,
        method: str,
        handler: Callable[[Request, RequestContext], Awaitable[Response]],
    ) -> None:
        """
        Registers a new HTTP handler with the application.
        A handler must be an async function(coroutine) that takes a Request object
        and returns a Response object.

        Args:
            path: The path of the handler.
            method: The HTTP method of the handler.
            handler: The handler function.

        Raises:
            TypeError: If the handler is not an asyncio.Future, coroutine or awaitable.
        """

    def register_websocket_handler(
        self,
        route: str,
        handler: Callable[[WebSocketMessageInfo], Awaitable[None]],
    ) -> None:
        """
        Registers a new WebSocket handler with the application.
        A handler must be an async function(coroutine) that takes a WebSocketMessageInfo object
        and returns None.

        Args:
            route: The route of the handler.
            handler: The handler function.

        Raises:
            TypeError: If the handler is not an asyncio.Future, coroutine or awaitable.
        """

    def websocket_registry(self) -> WebSocketRegistry:
        """
        Retrieves the WebSocket registry for the application
        that allows sending messages to specific WebSocket connections
        that are either connected to the current node running the application
        or other nodes in a WebSocket API cluster.

        Returns:
            A WebSocketRegistry instance that can be used to send messages to specific WebSocket connections.
        """
