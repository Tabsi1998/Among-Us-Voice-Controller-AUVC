#!/usr/bin/env python3
"""Write byte signatures that find the class addresses of a known build in another one.

The Epic and Microsoft Store builds of an Among Us version are the same C# code
as the Steam build, compiled into a GameAssembly.dll of their own: every class
lies at another address, but the code that reaches it reads the same. For each
class address of a known entry in capture/Offsets.json this finds an instruction
that loads it, and keeps the bytes around it with every address-dependent part
masked, until the pattern occurs once only. AUVC searches the running game for
those patterns when it does not know its hash (OffsetManager.Derive).

    python scripts/signatures_from_build.py --game-assembly GameAssembly.dll \\
        --hash 615A95C9E09B6FFD45570883CB8459311C2B457533A38B37ED1704D6205AEDF9

Only 64-bit builds; a 32-bit build addresses its globals absolutely.
"""

from __future__ import annotations

import argparse
import json
import re
import struct
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import offsets_from_il2cpp as offsets  # noqa: E402

ROOT = Path(__file__).resolve().parents[1]
INDEX = ROOT / "capture" / "Offsets.json"
OUTPUT = ROOT / "capture" / "AUOffsetManager" / "Signatures.json"

CLASSES = ("AmongUsClientOffset", "GameDataOffset", "MeetingHudOffset", "GameStartManagerOffset",
           "HudManagerOffset", "ServerManagerOffset", "TempDataOffset", "GameOptionsOffset")
LONGEST = 64


def sections(dll: bytes) -> list[tuple[str, int, int, int, int]]:
    pe = struct.unpack_from("<I", dll, 0x3C)[0]
    count = struct.unpack_from("<H", dll, pe + 6)[0]
    optional = struct.unpack_from("<H", dll, pe + 20)[0]
    found = []
    for i in range(count):
        at = pe + 24 + optional + 40 * i
        name = dll[at:at + 8].rstrip(b"\0").decode()
        virtual_size, rva, raw_size, raw = struct.unpack_from("<IIII", dll, at + 8)
        found.append((name, rva, virtual_size, raw, raw_size))
    return found


def image(dll: bytes) -> bytes:
    """The DLL as Windows maps it: every section at its address."""
    parts = sections(dll)
    size = max(rva + max(virtual, raw_size) for _, rva, virtual, _, raw_size in parts)
    mapped = bytearray(size)
    mapped[:0x1000] = dll[:0x1000]
    for _, rva, _, raw, raw_size in parts:
        mapped[rva:rva + raw_size] = dll[raw:raw + raw_size]
    return bytes(mapped)


def rip_loads(code: bytes, start: int, target: int) -> list[int]:
    """Where an instruction with a REX prefix loads [rip + x] that is target."""
    hits = []
    for match in re.finditer(rb"[\x48\x4C][\x8B\x8D][\x05\x0D\x15\x1D\x25\x2D\x35\x3D]", code):
        at = match.start()
        disp = struct.unpack_from("<i", code, at + 3)[0]
        if start + at + 7 + disp == target:
            hits.append(at)
    return hits


def masked(window: bytes) -> list[int | None]:
    """The bytes with everything that depends on where code or data lies masked:
    relative calls and jumps, and every [rip + x] operand. Cut-off ones at the end
    are masked too. Masking a byte too many costs nothing, one too few makes the
    pattern miss in another build."""
    out: list[int | None] = list(window)

    def mask(start: int, count: int = 4) -> None:
        out[start:min(start + count, len(window))] = [None] * (min(start + count, len(window)) - start)

    for i, byte in enumerate(window):
        if byte in (0xE8, 0xE9):
            mask(i + 1)
        elif byte == 0x0F and i + 1 < len(window) and 0x80 <= window[i + 1] <= 0x8F:
            mask(i + 2)
        # [REX] opcode modrm with mod 00 and rm 101: a 32-bit displacement from rip.
        for opcode_length in (1, 2):
            modrm = i + opcode_length
            if modrm < len(window) and (window[modrm] & 0xC7) == 0x05 and (opcode_length == 1 or byte == 0x0F) \
                    and byte not in (0xE8, 0xE9):
                mask(modrm + 1)
    return out


def as_regex(pattern: list[int | None]) -> re.Pattern:
    return re.compile(b"".join(b"." if b is None else re.escape(bytes([b])) for b in pattern), re.S)


def as_text(pattern: list[int | None]) -> str:
    return " ".join("?" if b is None else f"{b:02X}" for b in pattern)


def signature(code: bytes, start: int, target: int) -> dict | None:
    """The shortest unique pattern around a load of target, with where its displacement is."""
    best = None
    for at in rip_loads(code, start, target)[:6]:
        begin = at
        for length in range(16, LONGEST + 1, 4):
            window = code[begin:begin + length]
            pattern = masked(window)
            if len(as_regex(pattern).findall(code)) == 1:
                candidate = {"sig": as_text(pattern), "displacement": at - begin + 3}
                if best is None or len(candidate["sig"]) < len(best["sig"]):
                    best = candidate
                break
    return best


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--game-assembly", required=True, type=Path)
    parser.add_argument("--hash", required=True, help="the entry in capture/Offsets.json for this DLL")
    args = parser.parse_args(argv)

    dll = args.game_assembly.read_bytes()
    if offsets.architecture(dll) is not offsets.X64:
        parser.error("only 64-bit builds")
    entry = offsets.load_index(INDEX.read_text(encoding="utf-8"))[args.hash.upper()]
    # The whole image, as AUVC reads it from the game: unique here is unique there.
    code_start, code = 0, image(dll)

    found, missing = {}, []
    for name in CLASSES:
        result = signature(code, code_start, entry[name])
        if result is None:
            missing.append(name)
        else:
            found[name] = result
    if missing:
        print("No unique signature for: " + ", ".join(missing), file=sys.stderr)
        return 1

    document = {"x64": {"template": args.hash.upper(), "description": entry["Description"], "classes": found}}
    OUTPUT.write_text(json.dumps(document, indent=2) + "\n", encoding="utf-8")
    print(f"Wrote {len(found)} signatures for {entry['Description']} to {OUTPUT.relative_to(ROOT)}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
