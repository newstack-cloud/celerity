
using System;
using System.Diagnostics;
using System.Net.Http;
using System.Net.Http.Json;
using System.Threading.Tasks;
using Xunit;

namespace celerity.Tests
{

    record class Order(
        int? Id = null
    );

    public class ApplicationTest
    {
        [Fact]
        public async Task HttpApiTest()
        {
            try
            {
                Process process = new();
                process.StartInfo.FileName = "dotnet";
                process.StartInfo.Arguments = "run --project celerity.App";
                process.StartInfo.UseShellExecute = false;
                process.StartInfo.CreateNoWindow = true;
                // Current directory is sdk/bindings/dotnet/celerity.Tests/bin/*/net8.0
                // Run the project from the sdk/bindings/dotnet directory so dotnet
                // can find the celerity.App project.
                process.StartInfo.WorkingDirectory = "../../../../";
                process.Start();

                // Wait for the server to start.
                await Task.Delay(2000);

                var client = new HttpClient();
                var resp = await client.PostAsJsonAsync(
                    "http://localhost:8080/orders/1",
                    new Order(Id: 1)
                );

                Assert.Equal(System.Net.HttpStatusCode.OK, resp.StatusCode);
                var content = await resp.Content.ReadAsStringAsync();
                Assert.Equal("{\"message\":\"Order received\"}", content);
            }
            catch (Exception e)
            {
                Console.WriteLine(e);
            }
            finally
            {
                // Process is a parent process of the actual serverprocess that we need to kill.
                foreach (var currentProcess in Process.GetProcessesByName("celerity.App"))
                {
                    currentProcess.Kill();
                    await currentProcess.WaitForExitAsync();
                }
            }
        }
    }
}
