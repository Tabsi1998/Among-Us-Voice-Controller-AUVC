using System.Net;
using AmongUsCapture;
using Xunit;

namespace AUVC.Capture.Tests
{
    /// <summary>
    /// The API server answers without a login, with game state and player names. It was
    /// bound to every network interface; it is for overlays on this PC only.
    /// </summary>
    public class ApiServerTests
    {
        [Fact]
        public void TheApiServerIsReachableFromThisPcOnly()
        {
            var server = ServerSocket.CreateServer();

            Assert.Equal(IPAddress.Loopback, server.Address);
            Assert.Equal(ServerSocket.Port, server.Port);
            Assert.False(server.IsSecure);
        }
    }
}
