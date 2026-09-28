using System;
using System.Collections.Generic;
using System.Linq;
using System.Runtime.CompilerServices;
using System.Runtime.InteropServices;
using System.Text;
using System.Text.Json;
using System.Text.Json.Serialization;
using AmongUsCapture;

namespace AUVC.Capture.Tests.Recorded
{
    /// <summary>
    /// A recording of the game's memory, played back instead of a running game (#146).
    ///
    /// <para>The reader talks to <see cref="ProcessMemory"/>, so a recording is one more
    /// implementation of it. What it answers are the bytes that were written down; a read
    /// outside them is a hole in the recording and says so, because a silent zero would
    /// look like a player who left.</para>
    /// </summary>
    public sealed class RecordedMemory : ProcessMemory
    {
        private readonly SortedDictionary<long, byte[]> regions = new();

        public RecordedMemory(bool is64Bit = false)
        {
            this.is64Bit = is64Bit;
            IsHooked = true;
        }

        /// <summary>Puts bytes at an address, the way the game had them.</summary>
        public RecordedMemory Write(long address, params byte[] bytes)
        {
            regions[address] = bytes;
            return this;
        }

        public RecordedMemory WriteInt(long address, int value) => Write(address, BitConverter.GetBytes(value));

        /// <summary>A pointer as the recorded process stored it: four bytes, or eight on 64 bit.</summary>
        public RecordedMemory WritePointer(long address, long target) =>
            Write(address, is64Bit ? BitConverter.GetBytes(target) : BitConverter.GetBytes((int)target));

        /// <summary>A string the way Among Us keeps it: length, then UTF-16.</summary>
        public RecordedMemory WriteString(long address, string value, int lengthOffset = 0x8, int rawOffset = 0xC)
        {
            WriteInt(address + lengthOffset, value.Length);
            return Write(address + rawOffset, Encoding.Unicode.GetBytes(value));
        }

        // ------------------------------------------------------------ reading

        /// <summary>The bytes at an address, or a hole in the recording.</summary>
        public override byte[] Read(IntPtr address, int numBytes)
        {
            var wanted = address.ToInt64();
            var buffer = new byte[numBytes];
            var filled = new bool[numBytes];
            foreach (var (start, bytes) in regions)
            {
                var overlapFrom = Math.Max(start, wanted);
                var overlapTo = Math.Min(start + bytes.Length, wanted + numBytes);
                for (var at = overlapFrom; at < overlapTo; at++)
                {
                    buffer[at - wanted] = bytes[at - start];
                    filled[at - wanted] = true;
                }
            }
            if (filled.Any(byteFilled => !byteFilled) && !ZeroFill)
            {
                throw new RecordingGap(wanted, numBytes);
            }
            return buffer;
        }

        /// <summary>
        /// Whether a read outside the recording answers zeros instead of saying so. The game
        /// itself answers zeros for unmapped memory; a test wants to hear about the hole.
        /// </summary>
        public bool ZeroFill { get; set; }

        public override T Read<T>(IntPtr address, params int[] offsets) => ReadWithDefault<T>(address, default, offsets);

        public override T ReadWithDefault<T>(IntPtr address, T defaultParam, params int[] offsets)
        {
            if (address == IntPtr.Zero)
            {
                return defaultParam;
            }
            var last = OffsetAddress(ref address, offsets);
            if (address == IntPtr.Zero)
            {
                return defaultParam;
            }
            // A pointer is as wide as the recorded process, not as wide as this one.
            if (typeof(T) == typeof(IntPtr))
            {
                var pointer = Read(address + last, is64Bit ? 8 : 4);
                var target = is64Bit ? (IntPtr)BitConverter.ToInt64(pointer, 0) : (IntPtr)BitConverter.ToInt32(pointer, 0);
                return (T)(object)target;
            }
            return MemoryMarshal.Read<T>(Read(address + last, Unsafe.SizeOf<T>()));
        }

        public override string ReadString(IntPtr address, int lengthOffset = 0x8, int rawOffset = 0xC)
        {
            if (address == IntPtr.Zero)
            {
                return default;
            }
            var length = Read<int>(address + lengthOffset);
            return Encoding.Unicode.GetString(Read(address + rawOffset, length << 1));
        }

        public override IntPtr[] ReadArray(IntPtr address, int size)
        {
            var bytes = Read(address, size * 4);
            return Enumerable.Range(0, size).Select(index => (IntPtr)BitConverter.ToUInt32(bytes, index * 4)).ToArray();
        }

        public override int OffsetAddress(ref IntPtr address, params int[] offsets)
        {
            for (var index = 0; index < offsets.Length - 1; index++)
            {
                var step = Read(address + offsets[index], is64Bit ? 8 : 4);
                address = is64Bit ? (IntPtr)BitConverter.ToUInt64(step, 0) : (IntPtr)BitConverter.ToUInt32(step, 0);
                if (address == IntPtr.Zero)
                {
                    break;
                }
            }
            return offsets.Length > 0 ? offsets[^1] : 0;
        }

        // ------------------------------------------------- not part of a recording

        public override bool HookProcess(string name) => true;

        public override void LoadModules()
        {
        }

        // ------------------------------------------------------------ the file

        private sealed record Stored
        {
            [JsonPropertyName("is_64_bit")] public bool Is64Bit { get; init; }

            /// <summary>Address in hexadecimal to the bytes at it, base64 encoded.</summary>
            [JsonPropertyName("regions")] public Dictionary<string, string> Regions { get; init; } = new();
        }

        private static readonly JsonSerializerOptions Format = new() { WriteIndented = true };

        public string ToJson() => JsonSerializer.Serialize(new Stored
        {
            Is64Bit = is64Bit,
            Regions = regions.ToDictionary(region => "0x" + region.Key.ToString("X"), region => Convert.ToBase64String(region.Value)),
        }, Format);

        public static RecordedMemory FromJson(string json)
        {
            var stored = JsonSerializer.Deserialize<Stored>(json) ?? new Stored();
            var memory = new RecordedMemory(stored.Is64Bit);
            foreach (var (address, bytes) in stored.Regions)
            {
                memory.Write(Convert.ToInt64(address.Replace("0x", ""), 16), Convert.FromBase64String(bytes));
            }
            return memory;
        }
    }

    /// <summary>A read the recording has nothing for. Named, so a gap is never a silent zero.</summary>
    public sealed class RecordingGap : Exception
    {
        public RecordingGap(long address, int numBytes)
            : base($"the recording has no bytes for 0x{address:X} ({numBytes} bytes)")
        {
        }
    }
}
