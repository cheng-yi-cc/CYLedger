"""Backup acceptance tests. Test evidence is retained under .runtime/backup-tests-*.

Run: python -m unittest discover -s scripts -p test_backup.py -v
"""
import configparser
import json
from pathlib import Path
import sqlite3
import tempfile
import unittest
import zipfile

from backup import create_backup, restore_backup, verify_backup
from runtime_config import configure


class CompleteBackupTest(unittest.TestCase):
    def setUp(self):
        self.root = Path(__file__).resolve().parents[1]
        evidence = self.root / ".runtime"
        evidence.mkdir(exist_ok=True)
        self.work = Path(tempfile.mkdtemp(prefix="backup-tests-", dir=evidence))
        self.runtime = self.work / "original"
        self.config = configure(self.root, self.runtime, 18081, "127.0.0.1", self.root / "public")
        self.db = self.runtime / "data" / "cyledger.db"
        with sqlite3.connect(self.db) as conn:
            conn.executescript("""
                CREATE TABLE accounts (id INTEGER PRIMARY KEY, balance INTEGER, system_role TEXT);
                INSERT INTO accounts VALUES (1, 1098300, ''), (2, -901700, 'investment_settlement');
                CREATE TABLE investment_events (id TEXT PRIMARY KEY, quantity TEXT, version INTEGER);
                INSERT INTO investment_events VALUES ('buy', '0.020000000000000001', 1), ('sell', '0.005', 2);
                CREATE TABLE positions (id TEXT, quantity TEXT, cost TEXT, cost_known INTEGER);
                INSERT INTO positions VALUES ('btc', '0.015000000000000001', '9759.750000000000000001000000000000000001', 1);
                CREATE TABLE investment_transaction_links (event_id TEXT, transaction_id INTEGER);
                INSERT INTO investment_transaction_links VALUES ('sell', 321);
                CREATE TABLE valuation_snapshots (total TEXT, complete INTEGER);
                INSERT INTO valuation_snapshots VALUES ('22983.0000000000000008', 1);
                PRAGMA user_version = 1;
            """)
        attachment = self.runtime / "storage" / "user-1" / "receipt.bin"
        attachment.parent.mkdir()
        attachment.write_bytes(b"\x00receipt-with-binary-content\xff")
        self.archive = self.work / "backup.zip"

    def backup(self):
        return create_backup(self.runtime, self.archive, self.root, stopped=True)

    def rewrite(self, destination, change):
        with zipfile.ZipFile(self.archive) as source, zipfile.ZipFile(destination, "w", zipfile.ZIP_DEFLATED) as target:
            for item in source.infolist():
                target.writestr(item.filename, change(item.filename, source.read(item.filename)))

    def test_complete_restore_preserves_balances_basis_links_attachments_and_secret(self):
        manifest = self.backup()
        restored = self.work / "restored"
        restore_backup(self.archive, restored)
        with sqlite3.connect(self.db) as original, sqlite3.connect(restored / "data" / "cyledger.db") as copy:
            self.assertEqual(list(original.iterdump()), list(copy.iterdump()))
            self.assertEqual(copy.execute("SELECT balance FROM accounts WHERE id=1").fetchone()[0], 1098300)
            self.assertEqual(copy.execute("SELECT quantity,cost FROM positions").fetchone(), ("0.015000000000000001", "9759.750000000000000001000000000000000001"))
            self.assertEqual(copy.execute("SELECT total FROM valuation_snapshots").fetchone()[0], "22983.0000000000000008")
        self.assertEqual((self.runtime / "storage/user-1/receipt.bin").read_bytes(), (restored / "storage/user-1/receipt.bin").read_bytes())
        before, after = configparser.ConfigParser(interpolation=None), configparser.ConfigParser(interpolation=None)
        before.read(self.config, encoding="utf-8")
        after.read(restored / "cyledger.ini", encoding="utf-8")
        self.assertEqual(before["security"]["secret_key"], after["security"]["secret_key"])
        self.assertEqual(after["database"]["db_path"], str(restored / "data/cyledger.db"))
        self.assertFalse(after.getboolean("user", "enable_register"))
        self.assertEqual(manifest["database"]["tables"]["investment_transaction_links"], 1)
        self.assertEqual(verify_backup(self.archive)["format"], manifest["format"])
        (self.work / "verification.json").write_text(json.dumps({"balances": "equal", "positions": "equal", "costs": "equal", "netAssets": "equal", "links": "equal", "attachments": "equal", "secret": "preserved"}), encoding="utf-8")

    def test_byte_tampering_is_rejected_before_target_creation(self):
        self.backup()
        tampered = self.work / "tampered.zip"
        self.rewrite(tampered, lambda name, data: b"x" + data[1:] if name.endswith("receipt.bin") else data)
        with self.assertRaisesRegex(ValueError, "SHA256"):
            restore_backup(tampered, self.work / "must-not-exist")
        self.assertFalse((self.work / "must-not-exist").exists())

    def test_traversal_absolute_windows_and_duplicate_paths_are_rejected(self):
        self.backup()
        for number, path in enumerate(("../escape", "/escape", "C:/escape", "storage/../../escape", "storage\\escape", "storage/CON.txt", "storage/file. ")):
            with self.subTest(path=path):
                bad = self.work / f"bad-path-{number}.zip"
                self.rewrite(bad, lambda name, data: data)
                with zipfile.ZipFile(bad, "a") as target:
                    target.writestr(path, "bad")
                with self.assertRaises(ValueError):
                    verify_backup(bad)
        duplicate = self.work / "duplicate.zip"
        self.rewrite(duplicate, lambda name, data: data)
        with zipfile.ZipFile(duplicate, "a") as target:
            target.writestr("CYLEDGER.INI", "bad")
        with self.assertRaises(ValueError):
            verify_backup(duplicate)

    def test_existing_runtime_is_never_overwritten(self):
        self.backup()
        original = self.db.read_bytes()
        with self.assertRaisesRegex(ValueError, "empty"):
            restore_backup(self.archive, self.runtime)
        self.assertEqual(original, self.db.read_bytes())

    def test_existing_empty_target_supports_mounted_volume_layout(self):
        self.backup()
        target = self.work / "empty-volume"
        target.mkdir()
        restore_backup(self.archive, target)
        self.assertTrue((target / "data/cyledger.db").is_file())
        self.assertTrue((target / "storage/user-1/receipt.bin").is_file())
        self.assertFalse(any(p.name.startswith(".restoring-") for p in target.iterdir()))

    def test_repeated_config_keeps_signing_secret_and_backup_requires_stopped(self):
        first = self.config.read_text(encoding="utf-8")
        configure(self.root, self.runtime, 18081, "127.0.0.1", self.root / "public")
        self.assertEqual(first, self.config.read_text(encoding="utf-8"))
        with self.assertRaisesRegex(ValueError, "Stop"):
            create_backup(self.runtime, self.archive, self.root, stopped=False)


if __name__ == "__main__":
    unittest.main()
