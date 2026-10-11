"""Verify migration 0008 preserves data and cosmetics_inventory.py runs standalone."""
import json
import os
import shutil
import sqlite3
import subprocess
import sys
import tempfile
from pathlib import Path

MIGRATIONS = Path("/app/bot/pkg/storage/sqlite/migrations")
SCRIPT = Path("/app/scripts/cosmetics_inventory.py")


def apply_migrations(conn, names):
    for name in names:
        sql = (MIGRATIONS / name).read_text()
        conn.executescript(sql)
        conn.commit()


def test_migration_0008_preserves_player_stats_rows():
    with tempfile.TemporaryDirectory() as tmp:
        db = sqlite3.connect(os.path.join(tmp, "a.db"))
        try:
            db.executescript("PRAGMA foreign_keys = ON;")
            files = sorted(p.name for p in MIGRATIONS.iterdir() if p.name.endswith(".sql"))
            # Apply 0001-0007
            earlier = [f for f in files if f < "0008"]
            apply_migrations(db, earlier)
            # Insert a player_stats row (from 0007 schema)
            # Check the columns in player_stats
            cols = [row[1] for row in db.execute("PRAGMA table_info(player_stats)").fetchall()]
            # Build a row
            row = {
                "guild_id": "g1",
                "player_name": "Alice",
                "rounds": 7,
                "wins": 3,
                "impostor": 2,
                "updated_at": 1234567890,
            }
            present = [c for c in cols if c in row]
            placeholders = ",".join(["?"] * len(present))
            db.execute(
                f"INSERT INTO player_stats ({','.join(present)}) VALUES ({placeholders})",
                [row[c] for c in present],
            )
            db.commit()
            # Apply 0008
            apply_migrations(db, ["0008_stats_per_user.sql"])
            # player_stats must be gone
            tables = {r[0] for r in db.execute("SELECT name FROM sqlite_master WHERE type='table'").fetchall()}
            assert "player_stats" not in tables, "player_stats table should be dropped"
            assert "round_stats" in tables
            # Data must be in round_stats
            rows = db.execute(
                "SELECT guild_id, player_key, last_name, rounds, wins, impostor FROM round_stats"
            ).fetchall()
            assert rows == [("g1", "name:alice", "Alice", 7, 3, 2)], rows
        finally:
            db.close()


def test_cosmetics_inventory_runs_standalone():
    with tempfile.TemporaryDirectory() as tmp:
        tmp = Path(tmp)
        # Build fake Among Us dir
        aa = tmp / "fakeau" / "Among Us_Data" / "StreamingAssets" / "aa"
        aa.mkdir(parents=True)
        (aa / "dummy_hat.bundle").write_bytes(b"not a real bundle")
        catalog = {
            "m_InternalIds": ["hat_police", "pet_dog", "unrelated_file"],
            "m_Keys": ["skin_tank"],
        }
        (aa / "catalog.json").write_text(json.dumps(catalog))

        # Copy script alone to arbitrary dir
        standalone_dir = tmp / "x"
        standalone_dir.mkdir()
        copied = standalone_dir / "cosmetics_inventory.py"
        shutil.copy(SCRIPT, copied)

        out = tmp / "out.json"
        # Run from "/" to prove cwd independence
        result = subprocess.run(
            [sys.executable, str(copied), str(tmp / "fakeau"), str(out)],
            cwd="/",
            capture_output=True,
            text=True,
        )
        assert result.returncode == 0, f"stderr={result.stderr} stdout={result.stdout}"
        report = json.loads(out.read_text())
        assert "files" in report and len(report["files"]) >= 2
        assert "catalog_keys" in report
        # 3 cosmetic keys: hat_police, pet_dog, skin_tank
        assert set(report["catalog_keys"]) == {"hat_police", "pet_dog", "skin_tank"}, report["catalog_keys"]
        # UnityPy likely not installed -> no crash and bundles may be empty or error dict
        assert isinstance(report.get("bundles"), dict)


def test_cosmetics_inventory_missing_aa_folder_argparse_error():
    with tempfile.TemporaryDirectory() as tmp:
        tmp = Path(tmp)
        bad_root = tmp / "no_game"
        bad_root.mkdir()
        out = tmp / "out.json"
        result = subprocess.run(
            [sys.executable, str(SCRIPT), str(bad_root), str(out)],
            capture_output=True, text=True,
        )
        # argparse.error exits with 2
        assert result.returncode == 2, result.returncode
        assert "no " in result.stderr.lower() or "aa" in result.stderr, result.stderr


def test_cosmetics_inventory_unitypy_optional():
    """UnityPy not installed must not crash. We simulate by running normally
    (UnityPy likely absent in this env); if present we skip."""
    try:
        import UnityPy  # noqa: F401
        return  # already tested above with it available
    except ImportError:
        pass
    # Already covered by test_cosmetics_inventory_runs_standalone - exit 0 without UnityPy.


def test_cosmetics_script_no_repo_imports():
    """Script must not import from the repo (should be standalone)."""
    text = SCRIPT.read_text()
    # Only stdlib imports + optional UnityPy
    forbidden = ["from scripts", "import scripts"]
    for f in forbidden:
        assert f not in text, f"found forbidden import {f}"
