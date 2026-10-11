using System;
using System.Runtime.InteropServices;
using AUOffsetManager;

namespace AmongUsCapture
{

    public class PlayerInfo
    {
        public byte PlayerId;

        public string PlayerName;
        public PlayerColor ColorId;
        // Among Us names cosmetics by string, such as "hat_police" (#155).
        public string HatId;
        public string PetId;
        public string SkinId;
        public string VisorId;

        public uint PlayerLevel;
        public bool Disconnected;

        public uint RoleType;
        public uint RoleTeamType;

        public IntPtr Tasks;
        public bool IsImpostor;
        public bool IsDead;

        public IntPtr _object; //Assume this always has largest offset

        public PlayerInfo(IntPtr baseAddr, ProcessMemory MemInstance, GameOffsets CurrentOffsets)
        {
            unsafe
            {
                var baseAddrCopy = baseAddr;
                int last = MemInstance.OffsetAddress(ref baseAddrCopy, 0, 0);
                var intPtrSize = MemInstance.is64Bit ? 8 : 4;
                int size = StructBytes(CurrentOffsets.PlayerInfoStructOffsets.ObjectOffset + intPtrSize);
                byte[] buffer = MemInstance.Read(baseAddrCopy + last, size);
                PlayerInfoStructOffsets pOf = CurrentOffsets.PlayerInfoStructOffsets;
                PlayerOutfitStructOffsets oOf = CurrentOffsets.PlayerOutfitStructOffsets;
                var outfit = MemInstance.Read<IntPtr>(baseAddrCopy, pOf.OutfitsOffset);
                fixed (byte* ptr = buffer)
                {
                    var buffptr = (IntPtr)ptr;
                    PlayerId = Marshal.ReadByte(buffptr, pOf.PlayerIDOffset);
                    Disconnected = Marshal.ReadByte(buffptr, pOf.DisconnectedOffset) > 0;
                    // A pointer in the game is as wide as the game. Marshal.ReadIntPtr reads as
                    // wide as AUVC, which is 64 bit: it took the four bytes after the pointer
                    // with it, and at the last field those four bytes were past the buffer (#189).
                    Tasks = ReadPointer(buffptr, pOf.TasksOffset, MemInstance.is64Bit);
                    IsDead = Marshal.ReadByte(buffptr, pOf.IsDeadOffset) > 0;
                    _object = ReadPointer(buffptr, pOf.ObjectOffset, MemInstance.is64Bit);

                    // Read from Role
                    RoleType = (uint)MemInstance.Read<int>(baseAddrCopy, pOf.RoleTypeOffset);
                    RoleTeamType = (uint)MemInstance.Read<int>(baseAddrCopy, pOf.RoleTeamTypeOffset);
                    IsImpostor = RoleTeamType == 1;

                    // Read from PlayerOutfit
                    PlayerName = MemInstance.ReadString(MemInstance.Read<IntPtr>(outfit, oOf.PlayerNameOffset), CurrentOffsets.StringOffsets[0], CurrentOffsets.StringOffsets[1]);
                    ColorId = (PlayerColor)(uint)MemInstance.Read<int>(outfit, oOf.ColorIDOffset);
                    HatId = ReadId(MemInstance, outfit, oOf.HatIDOffset, CurrentOffsets);
                    PetId = ReadId(MemInstance, outfit, oOf.PetIDOffset, CurrentOffsets);
                    SkinId = ReadId(MemInstance, outfit, oOf.SkinIDOffset, CurrentOffsets);
                    VisorId = ReadId(MemInstance, outfit, oOf.VisorIDOffset, CurrentOffsets);
                }
            }
        }
        /// <summary>
        /// A cosmetic's id out of the outfit. An offset of 0 is one the index does not know:
        /// there sits the object's class, not a string.
        /// </summary>
        private static string ReadId(ProcessMemory memory, IntPtr outfit, int offset, GameOffsets offsets) =>
            offset == 0 || outfit == IntPtr.Zero
                ? ""
                : memory.ReadString(memory.Read<IntPtr>(outfit, offset), offsets.StringOffsets[0], offsets.StringOffsets[1]) ?? "";

        /// <summary>
        /// Bytes to read so a struct ends behind its last field, rounded up to a multiple of
        /// eight. The integer division used to round down, and on v18 the last pointer lay
        /// past the buffer.
        /// </summary>
        public static int StructBytes(int end) => (end + 7) / 8 * 8;

        /// <summary>A pointer out of the read struct, as wide as the game stored it.</summary>
        private static IntPtr ReadPointer(IntPtr buffer, int offset, bool is64Bit) =>
            is64Bit ? (IntPtr)Marshal.ReadInt64(buffer, offset) : (IntPtr)Marshal.ReadInt32(buffer, offset);

        public string GetPlayerName()
        {
            return PlayerName;
        }
        public bool GetIsDead()
        {
            return IsDead;
        }

        public bool GetIsImposter()
        {
            return IsImpostor;
        }

        public PlayerColor GetPlayerColor()
        {
            return ColorId;
        }

        public bool GetIsDisconnected()
        {
            return Disconnected;
        }
    }
}
