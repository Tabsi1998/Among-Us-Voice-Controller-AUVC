using System;
using AUVC.Transport;
using Xunit;

namespace AUVC.Capture.Tests
{
    public class BotAddressTests
    {
        [Theory]
        [InlineData("127.0.0.1:8123", "http://127.0.0.1:8123/")]
        [InlineData("  http://127.0.0.1:8123  ", "http://127.0.0.1:8123/")]
        [InlineData("https://bot.example.org", "https://bot.example.org/")]
        [InlineData("wss://bot.example.org:8443", "https://bot.example.org:8443/")]
        [InlineData("ws://192.168.1.20:8123", "http://192.168.1.20:8123/")]
        [InlineData("http://127.0.0.1:8123/capture/link", "http://127.0.0.1:8123/")]
        public void AnAddressIsReducedToWhereTheBotListens(string typed, string expected)
        {
            Assert.True(BotAddress.TryParse(typed, out var address, out var problem), problem);
            Assert.Equal(new Uri(expected), address);
        }

        [Theory]
        [InlineData("")]
        [InlineData("   ")]
        [InlineData(null)]
        [InlineData("ftp://bot.example.org")]
        [InlineData("http://")]
        public void WhatIsNotAnAddressSaysWhy(string? typed)
        {
            Assert.False(BotAddress.TryParse(typed, out _, out var problem));
            Assert.False(string.IsNullOrWhiteSpace(problem));
        }

        [Fact]
        public void TheDefaultIsAnAddress()
        {
            Assert.True(BotAddress.TryParse(BotAddress.Default, out _, out _));
        }

        /// <summary>
        /// Plain HTTP to another machine carries the credential in the clear,
        /// which the person pairing should be told. On the same PC it never
        /// crosses a network.
        /// </summary>
        [Theory]
        [InlineData("http://192.168.1.20:8123", true)]
        [InlineData("http://bot.example.org", true)]
        [InlineData("http://127.0.0.1:8123", false)]
        [InlineData("http://localhost:8123", false)]
        [InlineData("https://bot.example.org", false)]
        public void OnlyPlainHttpToAnotherMachineSendsTheCredentialInTheClear(string address, bool inClear)
        {
            Assert.Equal(inClear, BotAddress.SendsCredentialInClear(new Uri(address)));
        }
    }
}
