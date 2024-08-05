using System;
using Xunit;
using celerity;

namespace celerity.Tests
{
    public class ClassTest
    {
        [Fact]
        public void ConstructionDestructionTest()
        {
            Assert.Equal(0u, Application.ConstructionCounter());

            var application = new Application(41);
            Assert.Equal(1u, Application.ConstructionCounter());
            Assert.Equal(200u, application.GetValue());

            application.Shutdown();

            Assert.Equal(0u, Application.ConstructionCounter());
        }       
    }
}