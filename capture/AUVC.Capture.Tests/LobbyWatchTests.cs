using System.Collections.Generic;
using AmongUsCapture;
using Xunit;

namespace AUVC.Capture.Tests
{
    /// <summary>
    /// When capture tells the bot about the lobby. A lobby that is not told again
    /// leaves the crewmate message on the wrong map; one told on every pass would
    /// redraw it every quarter second.
    /// </summary>
    public class LobbyWatchTests
    {
        private static LobbyEventArgs Lobby(string code, PlayMap map, PlayRegion region = PlayRegion.Europe) =>
            new() { LobbyCode = code, Map = map, Region = region };

        /// <summary>A fake game: what each read returns, noting how often it was read.</summary>
        private sealed class Game
        {
            public LobbyEventArgs? Current { get; set; }

            public int Reads { get; private set; }

            public LobbyEventArgs? Read()
            {
                Reads++;
                return Current;
            }
        }

        /// <summary>Runs passes of the reader and returns what was told, in order.</summary>
        private static List<LobbyEventArgs?> Passes(LobbyWatch watch, Game game, params (GameState Old, GameState State, LobbyEventArgs? Lobby)[] passes)
        {
            var told = new List<LobbyEventArgs?>();
            foreach (var (old, state, lobby) in passes)
            {
                game.Current = lobby;
                told.Add(watch.Next(old, state, game.Read!));
            }
            return told;
        }

        [Fact]
        public void EnteringALobbyTellsItsCodeAndMap()
        {
            var lobby = Lobby("ABCDEF", PlayMap.Polus);

            var told = Passes(new LobbyWatch(), new Game(), (GameState.MENU, GameState.LOBBY, lobby));

            Assert.Same(lobby, Assert.Single(told));
        }

        [Fact]
        public void AnUnchangedLobbyIsNotToldAgain()
        {
            var told = Passes(new LobbyWatch(), new Game(),
                (GameState.MENU, GameState.LOBBY, Lobby("ABCDEF", PlayMap.Polus)),
                (GameState.LOBBY, GameState.LOBBY, Lobby("ABCDEF", PlayMap.Polus)),
                (GameState.LOBBY, GameState.LOBBY, Lobby("ABCDEF", PlayMap.Polus)));

            Assert.Equal(new[] { false, true, true }, told.ConvertAll(lobby => lobby is null));
        }

        // The owner's report from the live test: the picture stayed on the first map.
        [Fact]
        public void AnotherMapPickedInTheLobbyIsToldOnce()
        {
            var airship = Lobby("ABCDEF", PlayMap.Airship);

            var told = Passes(new LobbyWatch(), new Game(),
                (GameState.MENU, GameState.LOBBY, Lobby("ABCDEF", PlayMap.Skeld)),
                (GameState.LOBBY, GameState.LOBBY, airship),
                (GameState.LOBBY, GameState.LOBBY, Lobby("ABCDEF", PlayMap.Airship)));

            Assert.Same(airship, told[1]);
            Assert.Null(told[2]);
        }

        [Fact]
        public void GoingBackToTheFirstMapIsToldToo()
        {
            var skeldAgain = Lobby("ABCDEF", PlayMap.Skeld);

            var told = Passes(new LobbyWatch(), new Game(),
                (GameState.MENU, GameState.LOBBY, Lobby("ABCDEF", PlayMap.Skeld)),
                (GameState.LOBBY, GameState.LOBBY, Lobby("ABCDEF", PlayMap.Fungle)),
                (GameState.LOBBY, GameState.LOBBY, skeldAgain));

            Assert.Same(skeldAgain, told[2]);
        }

        [Fact]
        public void AnotherCodeOrRegionIsToldToo()
        {
            var newCode = Lobby("GHIJKL", PlayMap.Skeld);
            var newRegion = Lobby("GHIJKL", PlayMap.Skeld, PlayRegion.Asia);

            var told = Passes(new LobbyWatch(), new Game(),
                (GameState.MENU, GameState.LOBBY, Lobby("ABCDEF", PlayMap.Skeld)),
                (GameState.LOBBY, GameState.LOBBY, newCode),
                (GameState.LOBBY, GameState.LOBBY, newRegion));

            Assert.Same(newCode, told[1]);
            Assert.Same(newRegion, told[2]);
        }

        // Back in the lobby after a round, the bot is told again even though nothing
        // about the lobby changed, exactly as before this watch existed.
        [Fact]
        public void EveryLobbyEnteredIsToldEvenTheOneTheRoundStartedFrom()
        {
            var again = Lobby("ABCDEF", PlayMap.Polus);

            var told = Passes(new LobbyWatch(), new Game(),
                (GameState.MENU, GameState.LOBBY, Lobby("ABCDEF", PlayMap.Polus)),
                (GameState.LOBBY, GameState.TASKS, Lobby("ABCDEF", PlayMap.Polus)),
                (GameState.TASKS, GameState.LOBBY, again));

            Assert.Same(again, told[2]);
        }

        [Fact]
        public void NothingIsReadDuringARound()
        {
            var game = new Game();
            var watch = new LobbyWatch();
            Passes(watch, game, (GameState.MENU, GameState.LOBBY, Lobby("ABCDEF", PlayMap.Polus)));

            var told = Passes(watch, game,
                (GameState.LOBBY, GameState.TASKS, Lobby("ABCDEF", PlayMap.Mira)),
                (GameState.TASKS, GameState.DISCUSSION, Lobby("ABCDEF", PlayMap.Mira)),
                (GameState.DISCUSSION, GameState.MENU, Lobby("ABCDEF", PlayMap.Mira)));

            Assert.Equal(1, game.Reads);
            Assert.All(told, Assert.Null);
        }

        [Fact]
        public void ABlankCodeIsReadAgainUntilItArrives()
        {
            var lobby = Lobby("ABCD", PlayMap.Skeld);

            var told = Passes(new LobbyWatch(), new Game(),
                (GameState.MENU, GameState.LOBBY, null),
                (GameState.LOBBY, GameState.LOBBY, null),
                (GameState.LOBBY, GameState.LOBBY, lobby));

            Assert.Equal(new LobbyEventArgs?[] { null, null, lobby }, told);
        }

        // A round that starts before the code could be read still gets its lobby,
        // as the reader did before this watch existed.
        [Fact]
        public void ACodeStillBlankWhenTheRoundStartsIsReadDuringTheRound()
        {
            var lobby = Lobby("ABCD", PlayMap.Skeld);
            var game = new Game();

            var told = Passes(new LobbyWatch(), game,
                (GameState.MENU, GameState.LOBBY, null),
                (GameState.LOBBY, GameState.TASKS, lobby),
                (GameState.TASKS, GameState.TASKS, lobby));

            Assert.Same(lobby, told[1]);
            Assert.Null(told[2]);
            Assert.Equal(2, game.Reads);
        }
    }
}
