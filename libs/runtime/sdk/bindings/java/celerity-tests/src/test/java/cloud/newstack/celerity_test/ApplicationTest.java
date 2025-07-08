package cloud.newstack.celerity_test;

import cloud.newstack.celerity.Application;
import cloud.newstack.celerity.ApplicationStartupException;
import cloud.newstack.celerity.CoreRuntimeConfig;
import cloud.newstack.celerity.HttpHandler;
import cloud.newstack.celerity.HttpHandlerDefinition;
import cloud.newstack.celerity.HttpHandlersReceiver;
import cloud.newstack.celerity.Response;
import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectMapper;
import cloud.newstack.celerity.AppConfig;
import cloud.newstack.celerity.Runtime;
import cloud.newstack.celerity.RuntimeConfig;

import org.junit.jupiter.api.Test;

import java.io.IOException;
import java.net.URI;
import java.net.URISyntaxException;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.net.http.HttpClient.Redirect;
import java.net.http.HttpClient.Version;
import java.util.HashMap;
import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;
import static org.joou.Unsigned.ushort;
import static org.junit.jupiter.api.Assertions.fail;

class ApplicationTest {

    static class TestHttpHandler implements HttpHandler {

        @Override
        public void onRequest(Response response) {
            response.setStatus(ushort(200));
            response.send("{\"message\":\"Order received\"}");
        }
    }

    static class TestHandlerRegisterCallback implements HttpHandlersReceiver {

        private Application application;

        public TestHandlerRegisterCallback(Application application) {
            this.application = application;
        }

        @Override
        public void onHttpHandlerDefinitions(List<HttpHandlerDefinition> handlerDefinitions) {
            for (int i = 0; i < handlerDefinitions.size(); i += 1) {
                TestHttpHandler handler = new TestHttpHandler();
                this.application.registerHttpHandler(handler);
            }
        }
    }

    @Test
    void RunHttpApiTest() {
        final Runtime runtime = new Runtime(new RuntimeConfig());

        CoreRuntimeConfig config = new CoreRuntimeConfig(
                "http-api.blueprint.yaml",
                8080,
                true);
        Application application = new Application(config);
        AppConfig appConfig;
        try {
            appConfig = application.setup();
            HttpHandlersReceiver handlerRegisterCallback = new TestHandlerRegisterCallback(application);
            appConfig.getApiConfig().getHttpConfig().receiveHandlers(handlerRegisterCallback);

            application.run(runtime, false);

            try {
                HttpClient client = HttpClient.newBuilder()
                        .version(Version.HTTP_2)
                        .followRedirects(Redirect.NORMAL)
                        .build();

                String requestBody = prepareHttpRequest();
                HttpRequest request = HttpRequest.newBuilder()
                        .uri(new URI("http://localhost:8080/orders/1"))
                        .POST(HttpRequest.BodyPublishers.ofString(requestBody))
                        .header("Content-Type", "application/json")
                        .header("Accept", "application/json")
                        .build();

                HttpResponse<String> response = client.send(request,
                        HttpResponse.BodyHandlers.ofString());

                assertThat(response.body()).isEqualTo("{\"message\":\"Order received\"}");

            } catch (IOException | URISyntaxException | InterruptedException e) {
                fail(e.toString());
            }
        } catch (ApplicationStartupException e) {
            fail(e.toString());
        }
    }

    private String prepareHttpRequest() throws JsonProcessingException {
        HashMap<String, String> values = new HashMap<String, String>() {
            {
                put("id", "1");
            }
        };

        ObjectMapper objectMapper = new ObjectMapper();
        return objectMapper.writeValueAsString(values);
    }
}
