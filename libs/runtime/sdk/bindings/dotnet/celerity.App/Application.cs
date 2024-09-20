
using celerity;
using System;

namespace celerity.App
{

    class TestHttpHandler : IHttpHandler
    {
        public void OnRequest(Response response)
        {
            response.SetStatus(200);
            response.Send("{\"message\":\"Order received\"}");
        }
    }

    internal class CelerityTestApp {
        static void Main(string[] args) {
            var runtime = new Runtime(new RuntimeConfig());
            var config = new CoreRuntimeConfig(
                "http-api.blueprint.yaml",
                8080,
                true
            );
            var application = new Application(config);
            var appConfig = application.Setup();
            appConfig.GetApiConfig().GetHttpConfig().ReceiveHandlers(handlerDefinitions =>
            {
                foreach (var handlerDefinition in handlerDefinitions)
                {
                    var httpHandler = new TestHttpHandler();
                    application.RegisterHttpHandler(httpHandler);
                }
            });
            application.Run(runtime, true);
        }
    }
}
