#!/usr/bin/env python3
"""Complete, checksummed CYLedger SQLite/local-storage backup and empty-target restore.

Use Python 3.11+. Stop the application before create, including scheduled writers,
so attachments and the SQLite snapshot represent the same application state.
"""
import argparse
import configparser
from contextlib import closing
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import shutil
import sqlite3
import stat
import subprocess
import sys
import tempfile
import uuid
import zipfile

FORMAT = "cyledger-complete-backup-v1"
MAX_TOTAL_BYTES = 16 * 1024 ** 3
MAX_FILES = 100000
BLOCK_SIZE = 1024 * 1024
RESERVED = {"CON", "PRN", "AUX", "NUL", *[f"COM{i}" for i in range(1, 10)], *[f"LPT{i}" for i in range(1, 10)]}


def safe_name(name: str) -> str:
    if not isinstance(name, str) or not name or "\\" in name or "\x00" in name:
        raise ValueError("Invalid archive path")
    p = PurePosixPath(name)
    if p.is_absolute() or str(p) != name or any(part in (".", "..") for part in p.parts):
        raise ValueError(f"Unsafe archive path: {name}")
    for part in p.parts:
        if ":" in part or part.endswith((".", " ")) or part.split(".", 1)[0].upper() in RESERVED:
            raise ValueError(f"Nonportable archive path: {name}")
    if name not in ("manifest.json", "cyledger.ini") and (len(p.parts) < 2 or p.parts[0] not in ("data", "storage")):
        raise ValueError(f"Unexpected archive path: {name}")
    return name


def reject_links(path: Path):
    current = path.absolute()
    for part in (current, *current.parents):
        if part.is_symlink() or getattr(part, "is_junction", lambda: False)():
            raise ValueError(f"Symlink/junction is not allowed: {part}")


def digest_file(path: Path) -> str:
    h = hashlib.sha256()
    with path.open("rb") as stream:
        for block in iter(lambda: stream.read(BLOCK_SIZE), b""):
            h.update(block)
    return h.hexdigest()


def db_summary(path: Path) -> dict:
    with closing(sqlite3.connect(path.resolve().as_uri() + "?mode=ro", uri=True)) as conn:
        check = conn.execute("PRAGMA integrity_check").fetchone()[0]
        if check != "ok":
            raise ValueError(f"SQLite integrity check failed: {check}")
        tables = {}
        for (table,) in conn.execute("SELECT name FROM sqlite_master WHERE type='table' ORDER BY name"):
            quoted = '"' + table.replace('"', '""') + '"'
            tables[table] = conn.execute(f"SELECT COUNT(*) FROM {quoted}").fetchone()[0]
        return {"tables": tables, "userVersion": conn.execute("PRAGMA user_version").fetchone()[0]}


def inventory(folder: Path, prefix: str) -> dict[str, Path]:
    if not folder.exists():
        return {}
    reject_links(folder)
    files = {}
    for path in folder.rglob("*"):
        reject_links(path)
        if path.is_file():
            name = safe_name(prefix + "/" + path.relative_to(folder).as_posix())
            files[name] = path
    return files


def runtime_sources(runtime: Path) -> tuple[Path, Path, Path]:
    config_path = runtime / "cyledger.ini"
    config = configparser.ConfigParser(interpolation=None, strict=False)
    if not config.read(config_path, encoding="utf-8-sig"):
        raise ValueError(f"Missing runtime configuration: {config_path}")
    if config.get("database", "type") != "sqlite3" or config.get("storage", "type") != "local_filesystem":
        raise ValueError("This tool supports SQLite with local filesystem storage only")
    db = Path(config.get("database", "db_path"))
    storage = Path(config.get("storage", "local_filesystem_path"))
    if not db.is_absolute() or not storage.is_absolute():
        raise ValueError("Run runtime_config.py first; backup requires absolute runtime paths")
    for path in (runtime, config_path, db, storage):
        reject_links(path)
    if not db.is_file():
        raise ValueError(f"Database does not exist: {db}")
    if not storage.is_dir():
        raise ValueError(f"Storage directory does not exist: {storage}")
    return config_path, db, storage


def create_backup(runtime: Path, output: Path, root: Path, stopped: bool) -> dict:
    if not stopped:
        raise ValueError("Stop the application and pass --stopped to capture attachments consistently")
    runtime, output, root = runtime.resolve(), output.absolute(), root.resolve()
    config_path, db, storage = runtime_sources(runtime)
    if output.exists():
        raise ValueError("Refusing to overwrite an existing backup")
    if output.is_relative_to(storage) or output.is_relative_to(runtime / "data"):
        raise ValueError("Place backups outside the data and attachment directories")
    reject_links(output)
    output.parent.mkdir(parents=True, exist_ok=True)
    manifest = {"format": FORMAT, "createdAt": datetime.now(timezone.utc).isoformat(),
                "application": "CYLedger", "includesSecrets": True, "consistency": "application-stopped",
                "files": {}}
    package = root / "package.json"
    if package.exists():
        manifest["upstreamVersion"] = json.loads(package.read_text(encoding="utf-8-sig"))["version"]
    try:
        commit = subprocess.run(["git", "rev-parse", "HEAD"], cwd=root, capture_output=True, text=True, check=True)
        manifest["sourceCommit"] = commit.stdout.strip()
    except (OSError, subprocess.CalledProcessError):
        manifest["sourceCommit"] = "unavailable"
    # The short-lived working snapshot is private and never substitutes for the
    # final archive. SQLite's backup API includes any committed WAL contents.
    with tempfile.TemporaryDirectory(prefix="cyledger-snapshot-") as work:
        snapshot = Path(work) / "cyledger.db"
        with closing(sqlite3.connect(db.as_uri() + "?mode=ro", uri=True)) as source:
            with closing(sqlite3.connect(snapshot)) as target:
                source.backup(target)
        manifest["database"] = db_summary(snapshot)
        sources = inventory(storage, "storage")
        sources["cyledger.ini"] = config_path
        sources["data/cyledger.db"] = snapshot
        # Preserve non-database data files, while excluding SQLite sidecars.
        for name, path in inventory(runtime / "data", "data").items():
            if path.resolve() == db.resolve() or path.name in {db.name + "-wal", db.name + "-shm", db.name + "-journal"}:
                continue
            if name in sources:
                raise ValueError(f"Conflicting data path: {name}")
            sources[name] = path
        if len(sources) > MAX_FILES or sum(p.stat().st_size for p in sources.values()) > MAX_TOTAL_BYTES:
            raise ValueError("Backup exceeds supported archive limits")
        with output.open("xb") as raw:
            if os.name != "nt":
                output.chmod(0o600)
            with zipfile.ZipFile(raw, "w", zipfile.ZIP_DEFLATED, compresslevel=6) as archive:
                for name, path in sorted(sources.items()):
                    before = path.stat()
                    checksum = digest_file(path)
                    archive.write(path, name)
                    after = path.stat()
                    if (before.st_size, before.st_mtime_ns) != (after.st_size, after.st_mtime_ns) or checksum != digest_file(path):
                        raise ValueError(f"File changed during backup; stop all application writers: {name}")
                    manifest["files"][name] = {"sha256": checksum, "size": before.st_size}
                archive.writestr("manifest.json", json.dumps(manifest, ensure_ascii=False, indent=2))
    verify_backup(output)
    return manifest


def verify_backup(path: Path) -> dict:
    with zipfile.ZipFile(path, "r") as archive:
        entries = archive.infolist()
        if len(entries) > MAX_FILES + 1 or sum(i.file_size for i in entries) > MAX_TOTAL_BYTES:
            raise ValueError("Archive exceeds supported size or entry count")
        names, folded = {}, set()
        for item in entries:
            name = safe_name(item.filename)
            if item.is_dir() or name.casefold() in folded or item.flag_bits & 1:
                raise ValueError(f"Duplicate, directory or encrypted archive entry: {name}")
            mode = item.external_attr >> 16
            if stat.S_ISLNK(mode) or stat.S_ISDIR(mode):
                raise ValueError(f"Archive links and directories are not allowed: {name}")
            folded.add(name.casefold())
            names[name] = item
        if "manifest.json" not in names or names["manifest.json"].file_size > 16 * 1024 ** 2:
            raise ValueError("Missing or oversized manifest")
        manifest = json.loads(archive.read("manifest.json"))
        if manifest.get("format") != FORMAT or not isinstance(manifest.get("files"), dict):
            raise ValueError("Unknown backup format")
        if set(manifest["files"]) != set(names) - {"manifest.json"}:
            raise ValueError("Manifest and archive file lists differ")
        if not {"data/cyledger.db", "cyledger.ini"}.issubset(manifest["files"]):
            raise ValueError("Backup is missing database or configuration")
        for name, expected in manifest["files"].items():
            if not isinstance(expected, dict) or names[name].file_size != expected.get("size"):
                raise ValueError(f"File size mismatch: {name}")
            digest = hashlib.sha256()
            with archive.open(name) as stream:
                for block in iter(lambda: stream.read(BLOCK_SIZE), b""):
                    digest.update(block)
            if digest.hexdigest() != expected.get("sha256"):
                raise ValueError(f"SHA256 mismatch: {name}")
        return manifest


def restore_backup(archive_path: Path, target: Path) -> dict:
    target = target.absolute()
    reject_links(target)
    if target.exists() and (not target.is_dir() or any(target.iterdir())):
        raise ValueError("Restore target must be absent or an empty directory")
    manifest = verify_backup(archive_path)
    target.parent.mkdir(parents=True, exist_ok=True)
    target_exists = target.exists()
    # A mounted Docker volume cannot itself be renamed or removed. Stage
    # inside an existing empty target so final moves stay on its filesystem.
    staging_parent = target if target_exists else target.parent
    staging = staging_parent / (".restoring-" + uuid.uuid4().hex)
    staging.mkdir(mode=0o700)
    # Never call ZipFile.extract/extractall. Every destination is checked and
    # no existing files can be overwritten. A failed restore remains in its
    # named staging directory for inspection, never in the active runtime.
    with zipfile.ZipFile(archive_path, "r") as archive:
        for name in manifest["files"]:
            safe_name(name)
            destination = staging.joinpath(*PurePosixPath(name).parts)
            if not destination.resolve().is_relative_to(staging.resolve()):
                raise ValueError("Archive destination escapes staging directory")
            destination.parent.mkdir(parents=True, exist_ok=True)
            with archive.open(name) as source, destination.open("xb") as output:
                shutil.copyfileobj(source, output, BLOCK_SIZE)
            if digest_file(destination) != manifest["files"][name]["sha256"]:
                raise ValueError(f"Restored file checksum mismatch: {name}")
    if db_summary(staging / "data" / "cyledger.db") != manifest["database"]:
        raise ValueError("Restored database summary differs from backup")
    for name in ("storage", "log"):
        (staging / name).mkdir(exist_ok=True)
    (staging / "restore-manifest.json").write_text(json.dumps(manifest, ensure_ascii=False, indent=2), encoding="utf-8")
    # Preserve the signing key and all business data, but relocate private
    # filesystem paths so a restored instance cannot open the original DB.
    cfg = configparser.ConfigParser(interpolation=None, strict=False)
    cfg.read(staging / "cyledger.ini", encoding="utf-8-sig")
    cfg.set("database", "db_path", str(target / "data" / "cyledger.db"))
    cfg.set("storage", "local_filesystem_path", str(target / "storage"))
    cfg.set("log", "log_path", str(target / "log" / "cyledger.log"))
    cfg.set("user", "enable_register", "false")
    with (staging / "cyledger.ini").open("w", encoding="utf-8", newline="\n") as stream:
        cfg.write(stream)
    if target_exists:
        if list(target.iterdir()) != [staging]:
            raise ValueError("Restore target changed while verification was running")
        for child in list(staging.iterdir()):
            child.rename(target / child.name)
        staging.rmdir()  # Only the now-empty, tool-created staging directory.
    else:
        staging.rename(target)
    return manifest


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest="command", required=True)
    create = sub.add_parser("create", help="Stop app first; back up all tables, local attachments and configuration")
    create.add_argument("--runtime", type=Path, required=True)
    create.add_argument("--output", type=Path, required=True)
    create.add_argument("--root", type=Path, default=Path(__file__).resolve().parents[1])
    create.add_argument("--stopped", action="store_true", help="Confirm all app processes/writers have stopped")
    verify = sub.add_parser("verify", help="Verify archive paths and SHA256 integrity without extracting")
    verify.add_argument("archive", type=Path)
    restore = sub.add_parser("restore", help="Restore only into an absent or empty runtime directory")
    restore.add_argument("archive", type=Path)
    restore.add_argument("--target", type=Path, required=True)
    args = parser.parse_args()
    if args.command == "create":
        manifest = create_backup(args.runtime, args.output, args.root, args.stopped)
    elif args.command == "verify":
        manifest = verify_backup(args.archive)
    else:
        manifest = restore_backup(args.archive, args.target)
    # Print counts and integrity status, never configuration or financial rows.
    print(json.dumps({"status": "ok", "operation": args.command, "createdAt": manifest["createdAt"],
                      "files": len(manifest["files"]), "tables": len(manifest["database"]["tables"])}, ensure_ascii=False))


if __name__ == "__main__":
    try:
        main()
    except (OSError, ValueError, KeyError, sqlite3.Error, zipfile.BadZipFile) as exc:
        print(f"Backup operation failed: {exc}", file=sys.stderr)
        sys.exit(1)
