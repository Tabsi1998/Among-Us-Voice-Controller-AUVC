using System;
using System.Collections.Generic;
using System.Runtime.InteropServices;
using System.Text;
using AUOffsetManager;

namespace AmongUsCapture.Memory.Structs
{

    public class WinningPlayerData
    {
        public string Name { get; }
        public int ColorId { get; }

        public bool IsYou { get; }
        public bool IsImpostor { get; }
        public bool IsDead { get; }

        public WinningPlayerData(IntPtr baseAddr, ProcessMemory MemInstance, GameOffsets CurrentOffsets)
        {
            unsafe
            {
                var baseAddrCopy = baseAddr;
                int last = MemInstance.OffsetAddress(ref baseAddrCopy, 0, 0);
                var flags = CurrentOffsets.WinningPlayerDataStructOffsets;
                int size = PlayerInfo.StructBytes(Math.Max(flags.IsDeadOffset, Math.Max(flags.IsYouOffset, flags.IsImposterOffset)) + 1);
                byte[] buffer = MemInstance.Read(baseAddrCopy + last, size);
                PlayerOutfitStructOffsets oOf = CurrentOffsets.PlayerOutfitStructOffsets;
                WinningPlayerDataStructOffsets pOf = CurrentOffsets.WinningPlayerDataStructOffsets;
                fixed (byte* ptr = buffer)
                {
                    var buffptr = (IntPtr)ptr;
                    Name = MemInstance.ReadString(MemInstance.Read<IntPtr>(baseAddrCopy, pOf.PlayerNameOffset), CurrentOffsets.StringOffsets[0], CurrentOffsets.StringOffsets[1]);
                    ColorId = MemInstance.Read<int>(MemInstance.Read<IntPtr>(baseAddrCopy, pOf.OutfitOffset), oOf.ColorIDOffset);
                    IsImpostor = Marshal.ReadByte(buffptr, pOf.IsImposterOffset) == 1;
                    IsDead = Marshal.ReadByte(buffptr, pOf.IsDeadOffset) > 0;
                    IsYou = Marshal.ReadByte(buffptr, pOf.IsYouOffset) == 1;
                }
            }
        }

        public string GetPlayerName()
        {
            return this.Name;
        }

        public string Display()
        {
            return this.GetPlayerName() + ":" + (this.IsImpostor ? "yes" : "no");
        }
    }
}