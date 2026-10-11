#!/usr/bin/env python3
"""Build an AUVC offset entry from Il2CppDumper's output for one Among Us build.

Il2CppDumper (https://github.com/Perfare/Il2CppDumper) reads GameAssembly.dll and
global-metadata.dat and writes dump.cs (classes and field offsets) and script.json
(where each class's TypeInfo lies in GameAssembly.dll). This script turns both into
the entry AUVC looks up by the SHA-256 of GameAssembly.dll, for 32-bit and 64-bit
builds (#198). Nothing is downloaded and the game is never touched.

    python scripts/offsets_from_il2cpp.py --dump dump.cs --script script.json \\
        --game-assembly "D:/SteamLibrary/steamapps/common/Among Us/GameAssembly.dll" \\
        --description "v2026.9.29s x64" --merge "%APPDATA%/AmongUsCapture/index.json"

Every value the script could not find is listed, and then no entry is written.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import re
import struct
import sys
from dataclasses import dataclass, field
from pathlib import Path


@dataclass(frozen=True)
class Layout:
    """What IL2CPP itself puts where, independent of Among Us."""
    pointer: int
    static_fields: int     # Il2CppClass::static_fields
    object_header: int     # klass + monitor; first instance field, m_CachedPtr of a Unity object
    list_items: int        # List<T>._items
    list_size: int         # List<T>._size
    array_data: int        # first element of a managed array
    string_length: int
    string_chars: int
    dictionary_entries: int  # Dictionary<TKey, TValue>._entries
    first_entry_value: int   # value of _entries[0] for an int-sized key, from the array start
    method_klass: int        # MethodInfo::klass


X86 = Layout(4, 0x5C, 0x8, 0x8, 0xC, 0x10, 0x8, 0xC, 0xC, 0x1C, 0x10)
X64 = Layout(8, 0xB8, 0x10, 0x10, 0x18, 0x20, 0x10, 0x14, 0x18, 0x30, 0x20)

# Class and field names as Among Us has used them; the first one present wins.
PLAYER_INFO = ("NetworkedPlayerInfo", "GameData.PlayerInfo", "PlayerInfo")
OUTFIT = ("NetworkedPlayerInfo.PlayerOutfit", "GameData.PlayerOutfit", "PlayerOutfit")
END_RESULT = ("EndGameResult", "TempData")
WINNER = ("CachedPlayerData", "WinningPlayerData")
TEXT = ("TMPro.TMP_Text", "TMP_Text")
REGION = ("StaticHttpRegionInfo", "DnsRegionInfo")


@dataclass
class Klass:
    name: str
    base: str | None
    fields: dict[str, int] = field(default_factory=dict)
    statics: dict[str, int] = field(default_factory=dict)


HEADER = re.compile(
    r"^(?:public|internal|private|protected)?[\w ]*?\b(?:class|struct)\s+([^\s:{]+)\s*(?::\s*([^\s,{/]+))?")
MODIFIERS = r"(?:public|private|protected|internal|static|readonly|volatile|new|unsafe|fixed)"
FIELD = re.compile(r"^\s*(?:\[[^\]]*\]\s*)*((?:" + MODIFIERS + r"\s+)*)(.+?)\s+([^\s;]+);\s*//\s*0x([0-9A-Fa-f]+)\s*$")


def parse_dump(text: str) -> dict[str, Klass]:
    """The classes in dump.cs, by name and by name with its namespace."""
    classes: dict[str, Klass] = {}
    namespace = ""
    current: Klass | None = None
    for line in text.splitlines():
        stripped = line.strip()
        if stripped.startswith("// Namespace:"):
            namespace = stripped[len("// Namespace:"):].strip()
            continue
        if current is None:
            match = HEADER.match(stripped)
            if match and not stripped.startswith("//"):
                base = match.group(2)
                current = Klass(match.group(1), base.split(".")[-1] if base else None)
                classes.setdefault(current.name, current)
                if namespace:
                    classes.setdefault(f"{namespace}.{current.name}", current)
            continue
        if stripped == "}":
            current = None
            continue
        match = FIELD.match(line)
        if match:
            modifiers, name, offset = match.group(1), match.group(3), int(match.group(4), 16)
            (current.statics if "static" in modifiers.split() else current.fields)[name] = offset
    return classes


def parse_script(text: str) -> dict[str, int]:
    """TypeInfo addresses by class name, and MethodInfo addresses as "Method$…",
    relative to GameAssembly.dll."""
    data = json.loads(text)
    found = {}
    for item in data.get("ScriptMetadataMethod", []):
        name = item.get("Name", "")
        if name.endswith(".get_Instance()"):
            found[name] = int(item["Address"])
    for item in data.get("ScriptMetadata", []):
        name = item.get("Name", "")
        if name.endswith("_TypeInfo"):
            full = name[:-len("_TypeInfo")]
            found[full] = int(item["Address"])
            # script.json names a class with its namespace; dump.cs mostly without.
            parts = full.split("<", 1)[0].split(".")
            for start in range(1, len(parts)):
                found.setdefault(".".join(parts[start:]) + full[len(full.split("<", 1)[0]):], int(item["Address"]))
    return found


class Missing(Exception):
    pass


def absent(what: str):
    raise Missing(what)


class Reader:
    def __init__(self, classes: dict[str, Klass], type_infos: dict[str, int], layout: Layout):
        self.classes, self.type_infos, self.layout = classes, type_infos, layout
        self.missing: list[str] = []

    def klass(self, names: tuple[str, ...]) -> Klass:
        for name in names:
            if name in self.classes:
                return self.classes[name]
        raise Missing(f"class {' / '.join(names)}")

    def field(self, names: tuple[str, ...], fields: tuple[str, ...]) -> int:
        klass = self.klass(names)
        seen = set()
        while klass is not None and klass.name not in seen:
            seen.add(klass.name)
            for name in fields:
                if name in klass.fields:
                    return klass.fields[name]
            klass = self.classes.get(klass.base) if klass.base else None
        raise Missing(f"field {' / '.join(fields)} in {names[0]}")

    def static(self, names: tuple[str, ...], fields: tuple[str, ...]) -> tuple[int, int]:
        """The TypeInfo address and the static field's offset."""
        for name in names:
            klass = self.classes.get(name)
            if klass and name in self.type_infos:
                for wanted in fields:
                    if wanted in klass.statics:
                        return self.type_infos[name], klass.statics[wanted]
        raise Missing(f"static {' / '.join(fields)} of {names[0]}")

    def instance(self, name: str) -> tuple[int, list[int]]:
        """Where a singleton's Instance is, as an address and the steps to it.

        A class with its own static Instance is reached through its TypeInfo. A
        DestroyableSingleton<T> keeps Instance in the generic class, which has no
        TypeInfo of its own in script.json; its get_Instance MethodInfo leads to it.
        """
        statics = self.layout.static_fields
        try:
            address, offset = self.static((name,), ("Instance", "_instance", "<Instance>k__BackingField"))
            return address, [statics, offset]
        except Missing:
            template = self.classes.get("DestroyableSingleton<T>")
            offset = next((template.statics[f] for f in ("_instance", "Instance") if template and f in template.statics), None)
            generic = f"DestroyableSingleton<{name}>"
            if offset is not None and generic in self.type_infos:
                return self.type_infos[generic], [statics, offset]
            method = f"Method${generic}.get_Instance()"
            if offset is not None and method in self.type_infos:
                return self.type_infos[method], [self.layout.method_klass, statics, offset]
            raise Missing(f"Instance of {name}")

    def take(self, what, fallback=0):
        try:
            return what()
        except Missing as missing:
            self.missing.append(str(missing))
            return fallback


PROCEEDING = re.compile(r"public const MeetingHud\.(?:MeetingStates|VoteStates) Proceeding = (\d+);")


def meeting_proceeding(dump: str) -> int | None:
    """The number of MeetingHud's "Proceeding": from it on the meeting is over."""
    match = PROCEEDING.search(dump)
    return int(match.group(1)) if match else None


def build_entry(classes: dict[str, Klass], type_infos: dict[str, int], layout: Layout,
                description: str, proceeding: int | None = None) -> tuple[dict, list[str]]:
    r = Reader(classes, type_infos, layout)
    L = layout
    take = r.take

    client, client_steps = take(lambda: r.instance("AmongUsClient"), (0, [0, 0]))
    data, data_steps = take(lambda: r.instance("GameData"), (0, [0, 0]))
    meeting, meeting_steps = take(lambda: r.instance("MeetingHud"), (0, [0, 0]))
    start, start_steps = take(lambda: r.instance("GameStartManager"), (0, [0, 0]))
    hud, _ = take(lambda: r.instance("HudManager"), (0, [0, 0]))
    server, server_steps = take(lambda: r.instance("ServerManager"), (0, [0, 0]))
    options, options_steps = take(lambda: r.instance("GameOptionsManager"), (0, [0, 0]))
    result, reason_static = take(lambda: r.static(END_RESULT, ("CachedGameOverReason", "EndReason")), (0, 0))
    _, winners_static = take(lambda: r.static(END_RESULT, ("CachedWinners", "winners")), (0, 0))

    player = lambda *names: take(lambda: r.field(PLAYER_INFO, names))
    outfit = lambda *names: take(lambda: r.field(OUTFIT, names))
    winner = lambda *names: take(lambda: r.field(WINNER, names))
    role = take(lambda: r.field(PLAYER_INFO, ("Role", "_role")))
    map_id = take(lambda: r.field((newest_options(classes) or "NormalGameOptionsV*",), ("MapId", "mapId", "<MapId>k__BackingField")))
    region_name = take(lambda: r.field(REGION, ("<Name>k__BackingField", "name", "Name")), L.object_header)
    statics = L.static_fields

    entry = {
        "Description": description,
        "AmongUsClientOffset": client,
        "GameDataOffset": data,
        "MeetingHudOffset": meeting,
        "GameStartManagerOffset": start,
        "HudManagerOffset": hud,
        "ServerManagerOffset": server,
        "TempDataOffset": result,
        "GameOptionsOffset": options,
        "MeetingHudPtr": [meeting, *meeting_steps],
        "MeetingHudCachePtrOffsets": [L.object_header],
        "MeetingHudStateOffsets": [take(lambda: r.field(("MeetingHud",), ("state",)))],
        "MeetingProceedingState": proceeding if proceeding is not None else
        take(lambda: absent("enum MeetingHud.MeetingStates.Proceeding")),
        "GameStateOffsets": [client, *client_steps,
                             take(lambda: r.field(("AmongUsClient", "InnerNetClient"), ("GameState", "gameState")))],
        "AllPlayerPtrOffsets": [data, *data_steps, take(lambda: r.field(("GameData",), ("AllPlayers",)))],
        "AllPlayersOffsets": [L.list_items],
        "PlayerCountOffsets": [L.list_size],
        "ExiledPlayerIdOffsets": [meeting, *meeting_steps,
                                  take(lambda: r.field(("MeetingHud",), ("exiledPlayer",))), player("PlayerId")],
        "RawGameOverReasonOffsets": [result, statics, reason_static],
        "WinningPlayersPtrOffsets": [result, statics, winners_static],
        "WinningPlayersOffsets": [L.list_items],
        "WinningPlayerCountOffsets": [L.list_size],
        "GameCodeOffsets": [start, *start_steps,
                            take(lambda: r.field(("GameStartManager",), ("GameRoomNameCode",))),
                            take(lambda: r.field(TEXT, ("m_text",)))],
        "PlayRegionOffsets": [server, *server_steps,
                              take(lambda: r.field(("ServerManager",), ("CurrentRegion", "currentRegion",
                                                                       "<CurrentRegion>k__BackingField"))),
                              region_name, L.string_length],
        "PlayMapOffsets": [options, *options_steps,
                           take(lambda: r.field(("GameOptionsManager",), ("currentGameOptions",
                                                                         "<CurrentGameOptions>k__BackingField"))),
                           map_id],
        "StringOffsets": [L.string_length, L.string_chars],
        "AddPlayerPtr": L.pointer,
        "PlayerListPtr": L.array_data,
        "PlayerInfoStructOffsets": {
            "PlayerIDOffset": player("PlayerId"),
            "OutfitsOffset": [player("Outfits"), L.dictionary_entries, L.first_entry_value],
            "PlayerLevelOffset": player("PlayerLevel"),
            "DisconnectedOffset": player("Disconnected"),
            "RoleTypeOffset": [role, take(lambda: r.field(("RoleBehaviour",), ("Role",)))],
            "RoleTeamTypeOffset": [role, take(lambda: r.field(("RoleBehaviour",), ("TeamType",)))],
            "TasksOffset": player("Tasks"),
            "IsDeadOffset": player("IsDead"),
            "ObjectOffset": player("_object"),
        },
        "WinningPlayerDataStructOffsets": {
            "PlayerNameOffset": winner("PlayerName", "Name"),
            "OutfitOffset": winner("Outfit"),
            "IsYouOffset": winner("IsYou"),
            "IsImposterOffset": winner("IsImpostor"),
            "IsDeadOffset": winner("IsDead"),
        },
        "PlayerOutfitStructOffsets": {
            "ColorIDOffset": outfit("ColorId"),
            "HatIDOffset": outfit("HatId"),
            "PetIDOffset": outfit("PetId"),
            "SkinIDOffset": outfit("SkinId"),
            "VisorIDOffset": outfit("VisorId"),
            "NamePlateIDOffset": outfit("NamePlateId"),
            "PlayerNameOffset": outfit("PlayerName", "preCensorName", "_playerName"),
        },
    }
    return entry, r.missing


def newest_options(classes: dict[str, Klass]) -> str | None:
    versions = [(int(m.group(1)), name) for name in classes
                if (m := re.fullmatch(r"NormalGameOptionsV(\d+)", name)) and
                any(f in classes[name].fields for f in ("MapId", "mapId", "<MapId>k__BackingField"))]
    return max(versions)[1] if versions else None


def architecture(dll: bytes) -> Layout:
    """x86 or x64 from the PE header of GameAssembly.dll."""
    if dll[:2] != b"MZ":
        raise ValueError("GameAssembly.dll is not a Windows program")
    pe = struct.unpack_from("<I", dll, 0x3C)[0]
    if dll[pe:pe + 4] != b"PE\0\0":
        raise ValueError("GameAssembly.dll has no PE header")
    machine = struct.unpack_from("<H", dll, pe + 4)[0]
    if machine == 0x8664:
        return X64
    if machine == 0x14C:
        return X86
    raise ValueError(f"unknown machine type 0x{machine:X}")


def load_index(text: str) -> dict:
    """An offset index as AUVC reads it: hexadecimal numbers and trailing commas allowed."""
    text = re.sub(r"\b0x([0-9A-Fa-f]+)\b", lambda m: str(int(m.group(1), 16)), text)
    text = re.sub(r",(\s*[}\]])", r"\1", text)
    return json.loads(text) if text.strip() else {}


def as_hex(entry: dict, hash_: str, indent: str = "  ") -> str:
    """The entry in the style of capture/Offsets.json, ready to paste."""
    body = json.dumps({hash_: entry}, indent=2)
    return re.sub(r"(?<=[\s\[,])(\d+)(?=[,\]\n])", lambda m: f"0x{int(m.group(1)):X}", body)


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--dump", required=True, type=Path, help="dump.cs from Il2CppDumper")
    parser.add_argument("--script", required=True, type=Path, help="script.json from Il2CppDumper")
    parser.add_argument("--game-assembly", type=Path, help="GameAssembly.dll: gives the hash and 32 or 64 bit")
    parser.add_argument("--hash", help="SHA-256 of GameAssembly.dll, if the file is not at hand")
    parser.add_argument("--arch", choices=("x86", "x64"), help="needed without --game-assembly")
    parser.add_argument("--description", required=True, help='as in Offsets.json, e.g. "v2026.9.29s x64"')
    parser.add_argument("--merge", type=Path, help="add the entry to this index.json (the local index)")
    args = parser.parse_args(argv)

    if args.game_assembly:
        dll = args.game_assembly.read_bytes()
        layout = architecture(dll)
        hash_ = hashlib.sha256(dll).hexdigest().upper()
    elif args.hash and args.arch:
        layout, hash_ = (X64 if args.arch == "x64" else X86), args.hash.upper()
    else:
        parser.error("give --game-assembly, or --hash with --arch")

    dump = args.dump.read_text(encoding="utf-8", errors="replace")
    type_infos = parse_script(args.script.read_text(encoding="utf-8"))
    entry, missing = build_entry(parse_dump(dump), type_infos, layout, args.description, meeting_proceeding(dump))
    if missing:
        for item in missing:
            print(f"MISSING: {item}", file=sys.stderr)
        print("No entry written. Look the names up in dump.cs and add them to this script.", file=sys.stderr)
        return 1

    if args.merge:
        path = Path(os.path.expandvars(str(args.merge)))
        index = load_index(path.read_text(encoding="utf-8")) if path.is_file() else {}
        index[hash_] = entry
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(json.dumps(index, indent=2) + "\n", encoding="utf-8")
        print(f"Added {hash_} ({args.description}) to {path}. In AUVC: Settings, Debug (Fehlersuche), Reload offsets.")
    print(as_hex(entry, hash_))
    return 0


if __name__ == "__main__":
    sys.exit(main())
