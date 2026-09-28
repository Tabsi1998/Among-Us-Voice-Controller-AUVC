using System;
using System.Collections.Generic;
using AUOffsetManager;

namespace AUVC.Capture.Tests.Recorded
{
    /// <summary>
    /// The lobby this recording holds, and the layout it was written in (#146).
    ///
    /// <para>The offsets are those of a real game version, taken from the bundled
    /// <c>Offsets.json</c> (v2024.3.5s): four-byte pointers, the player list behind
    /// <c>AllPlayerPtr</c>, one slot every four bytes.</para>
    ///
    /// <para>The lobby is the one that used to break the reader: the second slot holds a
    /// player who is still loading and has no name yet. Stepping only past usable players
    /// read that slot again for everybody after it, and those players vanished as if they
    /// had left the lobby.</para>
    /// </summary>
    public static class LobbyRecording
    {
        // Where the pieces live in the recording. Round numbers, because a recording is
        // read by address and nothing here depends on them being the game's own.
        public const long GameAssembly = 0x10000000;
        public const long AllPlayersPtr = 0x20000000;
        public const long AllPlayers = 0x20100000;
        private const long FirstStruct = 0x30000000;
        private const long StructStep = 0x1000;

        /// <summary>The offsets of the recorded game version.</summary>
        public static GameOffsets Offsets() => new()
        {
            Description = "recorded lobby (offsets of v2024.3.5s)",
            AllPlayerPtrOffsets = new[] { 0x0, 0x0 },
            AllPlayersOffsets = new[] { 0x8 },
            PlayerCountOffsets = new[] { 0xC },
            StringOffsets = new[] { 0x8, 0xC },
            AddPlayerPtr = 0x4,
            PlayerListPtr = 0x10,
            PlayerInfoStructOffsets = new PlayerInfoStructOffsets
            {
                PlayerIDOffset = 0x8,
                OutfitsOffset = new[] { 0x1C, 0xC, 0x1C },
                PlayerLevelOffset = 0x20,
                DisconnectedOffset = 0x24,
                RoleTypeOffset = new[] { 0x28, 0xC },
                RoleTeamTypeOffset = new[] { 0x28, 0x3C },
                TasksOffset = 0x2C,
                IsDeadOffset = 0x30,
                ObjectOffset = 0x34,
            },
            PlayerOutfitStructOffsets = new PlayerOutfitStructOffsets
            {
                ColorIDOffset = 0xC,
                PlayerNameOffset = 0x28,
            },
        };

        /// <summary>One player as the recording holds them.</summary>
        public sealed record Player(byte Id, string Name, int Color, bool Dead = false, bool Disconnected = false,
                                    int RoleTeam = 0);

        /// <summary>The lobby in the recording: three players, and one still loading.</summary>
        public static IReadOnlyList<Player> Lobby => new[]
        {
            new Player(1, "Paula", 0),
            new Player(2, "", 0),                       // still loading: no name yet
            new Player(3, "Leon", 3, Dead: true),
            new Player(4, "Mira", 7, RoleTeam: 1),
        };

        /// <summary>Writes the lobby into a fresh recording.</summary>
        /// <remarks>
        /// The blocks are written whole, because that is how they lie in a real process: the
        /// reader takes the player struct in one read of 56 bytes, not field by field.
        /// </remarks>
        public static RecordedMemory Build()
        {
            var offsets = Offsets();
            var players = Lobby;
            var memory = new RecordedMemory();

            // The chain the reader walks: GameAssembly -> AllPlayerPtr -> AllPlayers, and the count beside it.
            memory.WritePointer(GameAssembly, AllPlayersPtr);
            var header = new Block(0x20);
            header.Pointer(offsets.AllPlayersOffsets[0], AllPlayers);
            header.Int(offsets.PlayerCountOffsets[0], players.Count);
            memory.Write(AllPlayersPtr, header.Bytes);

            var slots = new Block(players.Count * offsets.AddPlayerPtr + 4);
            for (var index = 0; index < players.Count; index++)
            {
                var player = players[index];
                var structAt = FirstStruct + index * StructStep;
                var roleAt = structAt + 0x200;
                var outfitHolder = structAt + 0x400;
                var outfitCarrier = structAt + 0x500;
                var outfit = structAt + 0x600;
                var nameAt = structAt + 0x700;
                var info = offsets.PlayerInfoStructOffsets;

                // The slot holds the player's address; PlayerInfo dereferences it once.
                slots.Pointer(index * offsets.AddPlayerPtr, structAt);

                var block = new Block(0x40);
                block.Byte(info.PlayerIDOffset, player.Id);
                block.Byte(info.DisconnectedOffset, (byte)(player.Disconnected ? 1 : 0));
                block.Byte(info.IsDeadOffset, (byte)(player.Dead ? 1 : 0));
                block.Pointer(info.TasksOffset, 0);
                block.Pointer(info.ObjectOffset, 0);
                block.Pointer(info.OutfitsOffset[0], outfitHolder);
                block.Pointer(info.RoleTypeOffset[0], roleAt);
                memory.Write(structAt, block.Bytes);

                // The role says which team the player is on.
                var role = new Block(0x40);
                role.Int(info.RoleTypeOffset[1], 0);
                role.Int(info.RoleTeamTypeOffset[1], player.RoleTeam);
                memory.Write(roleAt, role.Bytes);

                // The outfit sits behind two more pointers; the name behind one more.
                var holder = new Block(0x20);
                holder.Pointer(info.OutfitsOffset[1], outfitCarrier);
                memory.Write(outfitHolder, holder.Bytes);

                var carrier = new Block(0x40);
                carrier.Pointer(info.OutfitsOffset[2], outfit);
                memory.Write(outfitCarrier, carrier.Bytes);

                var clothes = new Block(0x40);
                clothes.Int(offsets.PlayerOutfitStructOffsets.ColorIDOffset, player.Color);
                clothes.Pointer(offsets.PlayerOutfitStructOffsets.PlayerNameOffset, nameAt);
                memory.Write(outfit, clothes.Bytes);

                memory.WriteString(nameAt, player.Name, offsets.StringOffsets[0], offsets.StringOffsets[1]);
            }
            memory.Write(AllPlayers + offsets.PlayerListPtr, slots.Bytes);
            return memory;
        }

        /// <summary>A block of memory being written, field by field.</summary>
        private sealed class Block
        {
            public Block(int size) => Bytes = new byte[size];

            public byte[] Bytes { get; }

            public void Byte(int at, byte value) => Bytes[at] = value;

            public void Int(int at, int value) => BitConverter.GetBytes(value).CopyTo(Bytes, at);

            /// <summary>A four-byte pointer, as the recorded 32-bit process stored it.</summary>
            public void Pointer(int at, long target) => BitConverter.GetBytes((int)target).CopyTo(Bytes, at);
        }

        /// <summary>Where the player slots start, as the reader computes it.</summary>
        public static long FirstSlot() => AllPlayers + Offsets().PlayerListPtr;
    }
}
