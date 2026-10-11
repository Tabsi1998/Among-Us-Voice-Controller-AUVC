import io
import json
import struct
import sys
import tempfile
import unittest
from contextlib import redirect_stderr, redirect_stdout
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import offsets_from_il2cpp as tool  # noqa: E402

# A cut-down dump.cs as Il2CppDumper writes it, with the field offsets of a 64-bit build.
DUMP = """\
// Namespace: UnityEngine
public class Object // TypeDefIndex: 1
{
\t// Fields
\tprivate IntPtr m_CachedPtr; // 0x10
}

// Namespace: InnerNet
public abstract class InnerNetClient : MonoBehaviour // TypeDefIndex: 2
{
\t// Fields
\tpublic const int NoClientId = -1; // Metadata: 0x00123456
\tpublic string networkAddress; // 0x28
\tpublic InnerNetClient.GameStates GameState; // 0xAC
\t// Methods

\t// RVA: 0x1000 Offset: 0x1000 VA: 0x180001000
\tpublic void .ctor() { }
}

// Namespace: InnerNet
public enum InnerNetClient.GameStates // TypeDefIndex: 3
{
\tpublic int value__; // 0x0
\tpublic const InnerNetClient.GameStates NotJoined = 0;
}

// Namespace:
public class AmongUsClient : InnerNetClient // TypeDefIndex: 4
{
\t// Fields
\tpublic static AmongUsClient Instance; // 0x0
\tpublic string OnlineScene; // 0x170
}

// Namespace:
public class DestroyableSingleton<T> : MonoBehaviour // TypeDefIndex: 5
{
\t// Fields
\tprivate static T _instance; // 0x0
\tpublic bool DontDestroy; // 0x18
}

// Namespace:
public class GameData : InnerNetObject // TypeDefIndex: 6
{
\t// Fields
\tpublic static GameData Instance; // 0x0
\tpublic List<NetworkedPlayerInfo> AllPlayers; // 0x20
}

// Namespace:
public class NetworkedPlayerInfo : InnerNetObject // TypeDefIndex: 7
{
\t// Fields
\tpublic byte PlayerId; // 0x38
\tpublic Dictionary<PlayerOutfitType, NetworkedPlayerInfo.PlayerOutfit> Outfits; // 0x58
\tpublic uint PlayerLevel; // 0x60
\tpublic bool Disconnected; // 0x64
\tpublic RoleBehaviour Role; // 0x68
\tpublic List<NetworkedPlayerInfo.TaskInfo> Tasks; // 0x70
\tpublic bool IsDead; // 0x78
\tprivate PlayerControl _object; // 0x80
}

// Namespace:
[Serializable]
public class NetworkedPlayerInfo.PlayerOutfit // TypeDefIndex: 8
{
\t// Fields
\tpublic int ColorId; // 0x10
\tpublic string HatId; // 0x18
\tpublic string PetId; // 0x20
\tpublic string SkinId; // 0x28
\tpublic string VisorId; // 0x30
\tpublic string NamePlateId; // 0x38
\tpublic string PlayerName; // 0x40
}

// Namespace:
public abstract class RoleBehaviour : MonoBehaviour // TypeDefIndex: 9
{
\t// Fields
\tpublic RoleTypes Role; // 0x20
\tpublic RoleTeamTypes TeamType; // 0x74
}

// Namespace:
public class MeetingHud : DestroyableSingleton<MeetingHud> // TypeDefIndex: 10
{
\t// Fields
\tpublic static MeetingHud Instance; // 0x0
\tpublic NetworkedPlayerInfo exiledPlayer; // 0xD8
\tpublic MeetingHud.VoteStates state; // 0xE0
}

// Namespace:
public enum MeetingHud.MeetingStates // TypeDefIndex: 21
{
	public int value__; // 0x0
	public const MeetingHud.MeetingStates Animating = 0;
	public const MeetingHud.MeetingStates Results = 4;
	public const MeetingHud.MeetingStates Proceeding = 5;
}

// Namespace:
public class GameStartManager : DestroyableSingleton<GameStartManager> // TypeDefIndex: 11
{
\t// Fields
\tpublic TextMeshPro GameRoomNameCode; // 0x70
}

// Namespace: TMPro
public abstract class TMP_Text : MaskableGraphic // TypeDefIndex: 12
{
\t// Fields
\tprotected string m_text; // 0xD8
}

// Namespace:
public class HudManager : DestroyableSingleton<HudManager> // TypeDefIndex: 13
{
}

// Namespace:
public class ServerManager : DestroyableSingleton<ServerManager> // TypeDefIndex: 14
{
\t// Fields
\tprivate IRegionInfo <CurrentRegion>k__BackingField; // 0x28
}

// Namespace:
public class StaticHttpRegionInfo : IRegionInfo // TypeDefIndex: 15
{
\t// Fields
\tprivate readonly string <Name>k__BackingField; // 0x10
}

// Namespace:
public class GameOptionsManager // TypeDefIndex: 16
{
\t// Fields
\tprivate static GameOptionsManager _instance; // 0x0
\tpublic IGameOptions currentGameOptions; // 0x30
}

// Namespace: AmongUs.GameOptions
public class NormalGameOptionsV09 : AbstractGameOptions // TypeDefIndex: 17
{
\t// Fields
\tprotected byte mapId; // 0x18
}

// Namespace: AmongUs.GameOptions
public class NormalGameOptionsV10 : AbstractGameOptions // TypeDefIndex: 18
{
\t// Fields
\tprotected byte mapId; // 0x1C
}

// Namespace:
public static class EndGameResult // TypeDefIndex: 19
{
\t// Fields
\tpublic static List<CachedPlayerData> CachedWinners; // 0x0
\tpublic static GameOverReason CachedGameOverReason; // 0x10
}

// Namespace:
public class CachedPlayerData // TypeDefIndex: 20
{
\t// Fields
\tpublic string PlayerName; // 0x10
\tpublic NetworkedPlayerInfo.PlayerOutfit Outfit; // 0x18
\tpublic bool IsYou; // 0x20
\tpublic bool IsImpostor; // 0x21
\tpublic bool IsDead; // 0x22
}
"""

SCRIPT = {
    "ScriptMethod": [],
    "ScriptMetadata": [
        {"Address": 0x3A00010, "Name": "AmongUsClient_TypeInfo", "Signature": "AmongUsClient_c*"},
        {"Address": 0x3A00020, "Name": "GameData_TypeInfo", "Signature": "GameData_c*"},
        {"Address": 0x3A00030, "Name": "MeetingHud_TypeInfo", "Signature": "MeetingHud_c*"},
        {"Address": 0x3A00040, "Name": "DestroyableSingleton<GameStartManager>_TypeInfo", "Signature": ""},
        {"Address": 0x3A00050, "Name": "DestroyableSingleton<HudManager>_TypeInfo", "Signature": ""},
        {"Address": 0x3A00070, "Name": "GameOptionsManager_TypeInfo", "Signature": ""},
        {"Address": 0x3A00080, "Name": "EndGameResult_TypeInfo", "Signature": ""},
        {"Address": 0x3A00090, "Name": "TMPro.TMP_Text_TypeInfo", "Signature": ""},
        {"Address": 0x3A000A0, "Name": "System.Collections.Generic.List<string>_TypeInfo", "Signature": ""},
    ],
    # As in Among Us 2026.9.29: a DestroyableSingleton<T> has no TypeInfo of its own,
    # only the MethodInfo of its get_Instance.
    "ScriptMetadataMethod": [
        {"Address": 0x3A00060, "Name": "Method$DestroyableSingleton<ServerManager>.get_Instance()", "MethodAddress": 0},
    ],
}


def pe(machine: int) -> bytes:
    header = bytearray(0x100)
    header[:2] = b"MZ"
    struct.pack_into("<I", header, 0x3C, 0x80)
    header[0x80:0x84] = b"PE\0\0"
    struct.pack_into("<H", header, 0x84, machine)
    return bytes(header)


class OffsetsFromIl2CppTests(unittest.TestCase):
    def entry(self, dump: str = DUMP, layout=tool.X64):
        return tool.build_entry(tool.parse_dump(dump), tool.parse_script(json.dumps(SCRIPT)), layout, "test x64",
                                tool.meeting_proceeding(dump))

    def test_a_64_bit_entry_has_every_chain_with_64_bit_steps(self):
        entry, missing = self.entry()

        self.assertEqual(missing, [])
        self.assertEqual(entry["GameStateOffsets"], [0x3A00010, 0xB8, 0x0, 0xAC])
        self.assertEqual(entry["AllPlayerPtrOffsets"], [0x3A00020, 0xB8, 0x0, 0x20])
        self.assertEqual(entry["AllPlayersOffsets"], [0x10])
        self.assertEqual(entry["PlayerCountOffsets"], [0x18])
        self.assertEqual(entry["MeetingHudPtr"], [0x3A00030, 0xB8, 0x0])
        self.assertEqual(entry["MeetingHudCachePtrOffsets"], [0x10])
        self.assertEqual(entry["MeetingHudStateOffsets"], [0xE0])
        self.assertEqual(entry["ExiledPlayerIdOffsets"], [0x3A00030, 0xB8, 0x0, 0xD8, 0x38])
        self.assertEqual(entry["StringOffsets"], [0x10, 0x14])
        self.assertEqual((entry["AddPlayerPtr"], entry["PlayerListPtr"]), (8, 0x20))

    def test_the_meeting_ends_with_proceeding_as_the_game_counts_it(self):
        entry, _ = self.entry()

        self.assertEqual(entry["MeetingProceedingState"], 5)
        self.assertIsNone(tool.meeting_proceeding("public enum Other {}"))

    def test_singletons_without_their_own_instance_use_destroyable_singleton(self):
        entry, _ = self.entry()

        self.assertEqual(entry["GameStartManagerOffset"], 0x3A00040)
        self.assertEqual(entry["GameCodeOffsets"], [0x3A00040, 0xB8, 0x0, 0x70, 0xD8])
        self.assertEqual(entry["PlayRegionOffsets"], [0x3A00060, 0x20, 0xB8, 0x0, 0x28, 0x10, 0x10])
        self.assertEqual(entry["HudManagerOffset"], 0x3A00050)

    def test_a_singleton_without_type_info_is_reached_through_get_instance(self):
        entry, missing = self.entry()

        self.assertEqual(missing, [])
        self.assertEqual(entry["ServerManagerOffset"], 0x3A00060)
        self.assertEqual(entry["PlayRegionOffsets"][:4], [0x3A00060, 0x20, 0xB8, 0x0])

    def test_the_newest_game_options_class_gives_the_map(self):
        entry, _ = self.entry()

        self.assertEqual(entry["PlayMapOffsets"], [0x3A00070, 0xB8, 0x0, 0x30, 0x1C])

    def test_the_round_result_and_the_player_structs(self):
        entry, _ = self.entry()

        self.assertEqual(entry["RawGameOverReasonOffsets"], [0x3A00080, 0xB8, 0x10])
        self.assertEqual(entry["WinningPlayersPtrOffsets"], [0x3A00080, 0xB8, 0x0])
        self.assertEqual(entry["PlayerInfoStructOffsets"], {
            "PlayerIDOffset": 0x38, "OutfitsOffset": [0x58, 0x18, 0x30], "PlayerLevelOffset": 0x60,
            "DisconnectedOffset": 0x64, "RoleTypeOffset": [0x68, 0x20], "RoleTeamTypeOffset": [0x68, 0x74],
            "TasksOffset": 0x70, "IsDeadOffset": 0x78, "ObjectOffset": 0x80,
        })
        self.assertEqual(entry["WinningPlayerDataStructOffsets"], {
            "PlayerNameOffset": 0x10, "OutfitOffset": 0x18, "IsYouOffset": 0x20,
            "IsImposterOffset": 0x21, "IsDeadOffset": 0x22,
        })
        self.assertEqual(entry["PlayerOutfitStructOffsets"]["ColorIDOffset"], 0x10)
        self.assertEqual(entry["PlayerOutfitStructOffsets"]["PlayerNameOffset"], 0x40)

    def test_32_bit_uses_the_32_bit_runtime_layout(self):
        entry, _ = self.entry(layout=tool.X86)

        self.assertEqual(entry["GameStateOffsets"][1], 0x5C)
        self.assertEqual(entry["StringOffsets"], [0x8, 0xC])
        self.assertEqual(entry["PlayerInfoStructOffsets"]["OutfitsOffset"], [0x58, 0xC, 0x1C])
        self.assertEqual((entry["AddPlayerPtr"], entry["PlayerListPtr"]), (4, 0x10))

    def test_a_renamed_field_is_reported_not_guessed(self):
        _, missing = self.entry(DUMP.replace("exiledPlayer", "exiled"))

        self.assertEqual(missing, ["field exiledPlayer in MeetingHud"])

    def test_constants_and_methods_are_not_fields(self):
        classes = tool.parse_dump(DUMP)

        self.assertNotIn("NoClientId", classes["InnerNetClient"].fields)
        self.assertEqual(classes["InnerNet.InnerNetClient"].fields["GameState"], 0xAC)
        self.assertEqual(classes["DestroyableSingleton<T>"].statics, {"_instance": 0})

    def test_the_architecture_comes_from_the_pe_header(self):
        self.assertIs(tool.architecture(pe(0x8664)), tool.X64)
        self.assertIs(tool.architecture(pe(0x14C)), tool.X86)
        with self.assertRaises(ValueError):
            tool.architecture(b"not a program")

    def test_the_hex_output_reads_back_as_the_same_entry(self):
        entry, _ = self.entry()

        text = tool.as_hex(entry, "AB" * 32)

        self.assertIn('"AmongUsClientOffset": 0x3A00010', text)
        self.assertEqual(tool.load_index(text), {"AB" * 32: entry})

    def test_the_index_reader_takes_offsets_json_as_it_is(self):
        index = tool.load_index(
            (Path(__file__).resolve().parents[1] / "capture" / "Offsets.json").read_text(encoding="utf-8"))

        self.assertGreaterEqual(len(index), 60)
        self.assertEqual(index["A85759A989B915BA233894EDFCD89FC2B7F60991C8A4C566087D0052498DF70A"]
                         ["GameStateOffsets"], [0x2AC7874, 0x5C, 0x0, 0x64])

    def test_merge_writes_the_local_index_and_keeps_what_was_there(self):
        with tempfile.TemporaryDirectory() as folder:
            folder = Path(folder)
            (folder / "dump.cs").write_text(DUMP, encoding="utf-8")
            (folder / "script.json").write_text(json.dumps(SCRIPT), encoding="utf-8")
            dll = pe(0x8664)
            (folder / "GameAssembly.dll").write_bytes(dll)
            (folder / "index.json").write_text('{"OLD": {"Description": "kept"}}', encoding="utf-8")

            with redirect_stdout(io.StringIO()):
                code = tool.main(["--dump", str(folder / "dump.cs"), "--script", str(folder / "script.json"),
                                  "--game-assembly", str(folder / "GameAssembly.dll"),
                                  "--description", "test x64", "--merge", str(folder / "index.json")])

            index = json.loads((folder / "index.json").read_text(encoding="utf-8"))
            self.assertEqual(code, 0)
            self.assertEqual(index["OLD"], {"Description": "kept"})
            import hashlib
            self.assertEqual(index[hashlib.sha256(dll).hexdigest().upper()]["AddPlayerPtr"], 8)

    def test_nothing_is_written_while_a_value_is_missing(self):
        with tempfile.TemporaryDirectory() as folder:
            folder = Path(folder)
            (folder / "dump.cs").write_text(DUMP.replace("GameRoomNameCode", "LobbyCode"), encoding="utf-8")
            (folder / "script.json").write_text(json.dumps(SCRIPT), encoding="utf-8")

            errors = io.StringIO()
            with redirect_stderr(errors), redirect_stdout(io.StringIO()):
                code = tool.main(["--dump", str(folder / "dump.cs"), "--script", str(folder / "script.json"),
                                  "--hash", "ab" * 32, "--arch", "x64", "--description", "test",
                                  "--merge", str(folder / "index.json")])

            self.assertEqual(code, 1)
            self.assertIn("MISSING: field GameRoomNameCode in GameStartManager", errors.getvalue())
            self.assertFalse((folder / "index.json").exists())


if __name__ == "__main__":
    unittest.main()
