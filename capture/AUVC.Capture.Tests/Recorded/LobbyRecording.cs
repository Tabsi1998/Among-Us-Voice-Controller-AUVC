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

        /// <summary>
        /// The layout of v18s (build 7238), whose struct ends in a pointer at 0x58. The
        /// reader rounded its read of the struct down to 88 bytes and took that pointer from
        /// past the buffer.
        /// </summary>
        public static GameOffsets OffsetsV18() => new()
        {
            Description = "recorded lobby (offsets of v18s)",
            AllPlayerPtrOffsets = new[] { 0x0, 0x0 },
            AllPlayersOffsets = new[] { 0x8 },
            PlayerCountOffsets = new[] { 0xC },
            StringOffsets = new[] { 0x8, 0xC },
            AddPlayerPtr = 0x4,
            PlayerListPtr = 0x10,
            PlayerInfoStructOffsets = new PlayerInfoStructOffsets
            {
                PlayerIDOffset = 0x28,
                OutfitsOffset = new[] { 0x40, 0xC, 0x1C },
                PlayerLevelOffset = 0x44,
                DisconnectedOffset = 0x48,
                RoleTypeOffset = new[] { 0x4C, 0x10 },
                RoleTeamTypeOffset = new[] { 0x4C, 0x50 },
                TasksOffset = 0x50,
                IsDeadOffset = 0x54,
                ObjectOffset = 0x58,
            },
            PlayerOutfitStructOffsets = new PlayerOutfitStructOffsets
            {
                ColorIDOffset = 0x8,
                PlayerNameOffset = 0x20,
            },
        };

        /// <summary>
        /// A 64-bit layout (#198): eight-byte pointers, IL2CPP's 64-bit headers for lists,
        /// arrays, dictionaries and strings, and the player struct as BetterCrewLink reads
        /// it on the 64-bit V18. Not the offsets of a real build, but the shape of one.
        /// </summary>
        public static GameOffsets Offsets64() => new()
        {
            Description = "recorded lobby (64-bit layout)",
            AllPlayerPtrOffsets = new[] { 0x0, 0x0 },
            AllPlayersOffsets = new[] { 0x10 },
            PlayerCountOffsets = new[] { 0x18 },
            StringOffsets = new[] { 0x10, 0x14 },
            AddPlayerPtr = 0x8,
            PlayerListPtr = 0x20,
            PlayerInfoStructOffsets = new PlayerInfoStructOffsets
            {
                PlayerIDOffset = 0x38,
                OutfitsOffset = new[] { 0x58, 0x18, 0x30 },
                PlayerLevelOffset = 0x60,
                DisconnectedOffset = 0x64,
                RoleTypeOffset = new[] { 0x68, 0x20 },
                RoleTeamTypeOffset = new[] { 0x68, 0x74 },
                TasksOffset = 0x70,
                IsDeadOffset = 0x78,
                ObjectOffset = 0x80,
            },
            PlayerOutfitStructOffsets = new PlayerOutfitStructOffsets
            {
                ColorIDOffset = 0x10,
                HatIDOffset = 0x18,
                PetIDOffset = 0x20,
                SkinIDOffset = 0x28,
                VisorIDOffset = 0x30,
                PlayerNameOffset = 0x40,
            },
        };

        /// <summary>
        /// The two pointers the player struct carries, recorded with recognisable values.
        /// The field right behind Tasks is „is dead“: a pointer read wider than the game
        /// stored it takes that byte with it (#189).
        /// </summary>
        public const long TasksPointer = 0x0A0B0C0D;

        public const long ObjectPointer = 0x11223344;

        /// <summary>
        /// On 64 bit the same pointers sit above four gigabytes, as they do in a real 64-bit
        /// process, so a pointer read only four bytes wide loses its upper half and shows.
        /// </summary>
        public const long High = 0x7FF6_0000_0000;

        /// <summary>One player as the recording holds them.</summary>
        public sealed record Player(byte Id, string Name, int Color, bool Dead = false, bool Disconnected = false,
                                    int RoleTeam = 0, string Hat = "", string Pet = "", string Skin = "", string Visor = "");

        /// <summary>The lobby in the recording: three players, and one still loading.</summary>
        public static IReadOnlyList<Player> Lobby => new[]
        {
            new Player(1, "Paula", 0),
            new Player(2, "", 0),                       // still loading: no name yet
            new Player(3, "Leon", 3, Dead: true),
            new Player(4, "Mira", 7, RoleTeam: 1, Hat: "hat_police", Pet: "pet_EmptyPet", Skin: "skin_Mech", Visor: "visor_EmptyVisor"),
        };

        /// <summary>Writes the lobby into a fresh recording.</summary>
        /// <remarks>
        /// The blocks are written whole, because that is how they lie in a real process: the
        /// reader takes the player struct in one read of 56 bytes, not field by field.
        /// </remarks>
        public static RecordedMemory Build() => Build(Offsets(), is64Bit: false);

        public static RecordedMemory Build(GameOffsets offsets, bool is64Bit)
        {
            var players = Lobby;
            var memory = new RecordedMemory(is64Bit);
            var pointer = is64Bit ? 8 : 4;

            // The chain the reader walks: GameAssembly -> AllPlayerPtr -> AllPlayers, and the count beside it.
            memory.WritePointer(GameAssembly, AllPlayersPtr);
            var header = new Block(0x20, pointer);
            header.Pointer(offsets.AllPlayersOffsets[0], AllPlayers);
            header.Int(offsets.PlayerCountOffsets[0], players.Count);
            memory.Write(AllPlayersPtr, header.Bytes);

            var slots = new Block(players.Count * offsets.AddPlayerPtr + pointer, pointer);
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

                var block = new Block(0x100, pointer);
                block.Byte(info.PlayerIDOffset, player.Id);
                block.Byte(info.DisconnectedOffset, (byte)(player.Disconnected ? 1 : 0));
                block.Byte(info.IsDeadOffset, (byte)(player.Dead ? 1 : 0));
                block.Pointer(info.TasksOffset, is64Bit ? High | TasksPointer : TasksPointer);
                block.Pointer(info.ObjectOffset, is64Bit ? High | ObjectPointer : ObjectPointer);
                block.Pointer(info.OutfitsOffset[0], outfitHolder);
                block.Pointer(info.RoleTypeOffset[0], roleAt);
                memory.Write(structAt, block.Bytes);

                // The role says which team the player is on.
                var role = new Block(0x100, pointer);
                role.Int(info.RoleTypeOffset[1], 0);
                role.Int(info.RoleTeamTypeOffset[1], player.RoleTeam);
                memory.Write(roleAt, role.Bytes);

                // The outfit sits behind two more pointers; the name behind one more.
                var holder = new Block(0x40, pointer);
                holder.Pointer(info.OutfitsOffset[1], outfitCarrier);
                memory.Write(outfitHolder, holder.Bytes);

                var carrier = new Block(0x40, pointer);
                carrier.Pointer(info.OutfitsOffset[2], outfit);
                memory.Write(outfitCarrier, carrier.Bytes);

                var clothes = new Block(0x80, pointer);
                clothes.Int(offsets.PlayerOutfitStructOffsets.ColorIDOffset, player.Color);
                clothes.Pointer(offsets.PlayerOutfitStructOffsets.PlayerNameOffset, nameAt);

                memory.WriteString(nameAt, player.Name, offsets.StringOffsets[0], offsets.StringOffsets[1]);

                // Cosmetics are strings too, each behind its own pointer in the outfit (#155).
                var cosmetics = offsets.PlayerOutfitStructOffsets;
                var cosmeticAt = structAt + 0x800;
                foreach (var (offset, id) in new[] { (cosmetics.HatIDOffset, player.Hat), (cosmetics.PetIDOffset, player.Pet),
                                                     (cosmetics.SkinIDOffset, player.Skin), (cosmetics.VisorIDOffset, player.Visor) })
                {
                    if (offset == 0) continue;
                    clothes.Pointer(offset, cosmeticAt);
                    memory.WriteString(cosmeticAt, id, offsets.StringOffsets[0], offsets.StringOffsets[1]);
                    cosmeticAt += 0x100;
                }
                memory.Write(outfit, clothes.Bytes);
            }
            memory.Write(AllPlayers + offsets.PlayerListPtr, slots.Bytes);
            return memory;
        }

        /// <summary>A block of memory being written, field by field.</summary>
        private sealed class Block
        {
            private readonly int pointer;

            public Block(int size, int pointer)
            {
                Bytes = new byte[size];
                this.pointer = pointer;
            }

            public byte[] Bytes { get; }

            public void Byte(int at, byte value) => Bytes[at] = value;

            public void Int(int at, int value) => BitConverter.GetBytes(value).CopyTo(Bytes, at);

            /// <summary>A pointer as wide as the recorded process stored it.</summary>
            public void Pointer(int at, long target) =>
                (pointer == 8 ? BitConverter.GetBytes(target) : BitConverter.GetBytes((int)target)).CopyTo(Bytes, at);
        }

        /// <summary>Where the player slots start, as the reader computes it.</summary>
        public static long FirstSlot() => FirstSlot(Offsets());

        public static long FirstSlot(GameOffsets offsets) => AllPlayers + offsets.PlayerListPtr;
    }
}
