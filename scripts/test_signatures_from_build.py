import sys
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import signatures_from_build as tool  # noqa: E402


class SignaturesFromBuildTests(unittest.TestCase):
    """Everything that depends on where code or data lies is masked, or the pattern
    would only ever match the build it came from."""

    def test_rip_operands_and_calls_are_masked(self):
        window = bytes.fromhex("488B0D11223344" "E855667788" "4885C0" "0F8499AABBCC" "C605DDEEFF0001")

        text = tool.as_text(tool.masked(window))

        self.assertEqual(text, "48 8B 0D ? ? ? ? E8 ? ? ? ? 48 85 C0 0F 84 ? ? ? ? C6 05 ? ? ? ? 01")

    def test_a_call_cut_off_at_the_end_is_masked_too(self):
        self.assertEqual(tool.as_text(tool.masked(bytes.fromhex("4885C0E81122"))), "48 85 C0 E8 ? ?")

    def test_a_load_is_found_by_the_address_it_reads(self):
        code = bytes(0x20) + bytes.fromhex("488B05") + (0x1000 - (0x20 + 7)).to_bytes(4, "little", signed=True)

        self.assertEqual(tool.rip_loads(code, 0, 0x1000), [0x20])
        self.assertEqual(tool.rip_loads(code, 0, 0x2000), [])

    def test_the_signature_is_unique_and_says_where_the_address_is(self):
        load = bytes.fromhex("488B0D") + (0x400 - (0x100 + 7)).to_bytes(4, "little", signed=True)
        code = bytearray(0x800)
        code[0x100:0x100 + 16] = load + bytes.fromhex("E8000000004885C07A")
        code[0x200:0x200 + 16] = bytes.fromhex("488B0D00000000E8000000004885C07B")

        found = tool.signature(bytes(code), 0, 0x400)

        self.assertEqual(found["displacement"], 3)
        self.assertTrue(found["sig"].startswith("48 8B 0D ? ? ? ? E8 ? ? ? ? 48 85 C0 7A"))


if __name__ == "__main__":
    unittest.main()
