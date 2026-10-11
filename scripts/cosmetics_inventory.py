#!/usr/bin/env python3
"""Describe the cosmetics bundles of an Among Us install, small enough to upload.

The pictures of hats, skins, pets and visors lie in Unity Addressables bundles
under "Among Us_Data/StreamingAssets/aa", far too large to send. This writes
what is needed to plan reading them: every file with its size, the catalog's
keys that name cosmetics, and, with UnityPy installed (pip install UnityPy),
which objects each cosmetics bundle holds. Nothing is extracted or uploaded.

    python scripts/cosmetics_inventory.py "D:/SteamLibrary/steamapps/common/Among Us" cosmetics.json
"""

from __future__ import annotations

import argparse
import json
import re
import sys
from collections import Counter
from pathlib import Path

COSMETIC = re.compile(r"(hat|skin|pet|visor|nameplate)s?[_/]", re.IGNORECASE)
NAMES_PER_TYPE = 40


def catalog_keys(folder: Path) -> list[str]:
    """Catalog entries that name a cosmetic, such as "hat_police"."""
    keys: set[str] = set()
    for catalog in folder.glob("catalog*.json"):
        try:
            data = json.loads(catalog.read_text(encoding="utf-8"))
        except (OSError, ValueError):
            continue
        for key in data.get("m_InternalIds", []) + data.get("m_Keys", []) if isinstance(data, dict) else []:
            if isinstance(key, str) and COSMETIC.search(key):
                keys.add(key)
    return sorted(keys)[:2000]


def bundle_objects(path: Path) -> dict:
    """Object types in a bundle and the first names of the ones that matter."""
    import UnityPy  # noqa: PLC0415 - optional

    env = UnityPy.load(str(path))
    types: Counter = Counter()
    names: dict[str, list[str]] = {}
    for obj in env.objects:
        kind = obj.type.name
        types[kind] += 1
        if kind in ("Sprite", "Texture2D", "MonoBehaviour", "SpriteAtlas") and len(names.setdefault(kind, [])) < NAMES_PER_TYPE:
            try:
                names[kind].append(obj.read().m_Name)
            except Exception as error:  # noqa: BLE001 - a broken object is worth a line, not a crash
                names[kind].append(f"<{type(error).__name__}>")
    return {"types": dict(types), "names": names}


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("game", type=Path, help="the Among Us folder")
    parser.add_argument("output", type=Path, help="the JSON file to write")
    args = parser.parse_args(argv)

    folder = args.game / "Among Us_Data" / "StreamingAssets" / "aa"
    if not folder.is_dir():
        parser.error(f"no {folder}")
    try:
        import UnityPy  # noqa: F401, PLC0415
        deep = True
    except ImportError:
        deep = False
        print("UnityPy is not installed; files and catalog only (pip install UnityPy for more).", file=sys.stderr)

    files = sorted(path for path in folder.rglob("*") if path.is_file())
    report = {
        "files": [{"path": str(path.relative_to(folder)).replace("\\", "/"), "bytes": path.stat().st_size} for path in files],
        "catalog_keys": catalog_keys(folder),
        "bundles": {},
    }
    if deep:
        for path in files:
            if path.suffix == ".bundle" and COSMETIC.search(path.name + "_"):
                try:
                    report["bundles"][path.name] = bundle_objects(path)
                except Exception as error:  # noqa: BLE001
                    report["bundles"][path.name] = {"error": f"{type(error).__name__}: {error}"}

    args.output.write_text(json.dumps(report, indent=1), encoding="utf-8")
    print(f"{len(files)} files, {len(report['catalog_keys'])} cosmetic keys, {len(report['bundles'])} bundles read: {args.output}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
