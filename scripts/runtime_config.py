#!/usr/bin/env python3
"""Create or relocate a private CYLedger runtime configuration."""
import argparse
import configparser
import os
from pathlib import Path
import secrets


def configure(root: Path, runtime: Path, port: int, bind: str, public_root: Path, root_url: str | None = None) -> Path:
    root, runtime, public_root = root.resolve(), runtime.resolve(), public_root.resolve()
    runtime.mkdir(parents=True, exist_ok=True)
    for name in ("data", "storage", "log"):
        (runtime / name).mkdir(exist_ok=True)
    path = runtime / "cyledger.ini"
    config = configparser.ConfigParser(interpolation=None, strict=False)
    config.read(root / "conf" / "ezbookkeeping.ini", encoding="utf-8-sig")
    if path.exists():
        config.read(path, encoding="utf-8-sig")
    values = {
        "global": {"mode": "production"},
        "server": {"protocol": "http", "http_addr": bind, "http_port": str(port),
                   "domain": "localhost", "root_url": root_url or f"http://localhost:{port}/",
                   "static_root_path": str(public_root), "log_request": "false"},
        "database": {"type": "sqlite3", "db_path": str(runtime / "data" / "cyledger.db"),
                     "max_open_conn": "1", "log_query": "false", "auto_update_database": "true"},
        "storage": {"type": "local_filesystem", "local_filesystem_path": str(runtime / "storage")},
        "log": {"mode": "console file", "level": "info", "log_path": str(runtime / "log" / "cyledger.log"),
                "log_file_rotate": "true"},
        "user": {"enable_register": "false"},
        "auth": {"enable_forget_password": "false", "oauth2_auto_register": "false"},
        "llm": {"transaction_from_ai_text_recognition": "false", "transaction_from_ai_image_recognition": "false"},
    }
    for section, options in values.items():
        if not config.has_section(section):
            config.add_section(section)
        for key, value in options.items():
            config.set(section, key, value)
    if not config.has_section("security"):
        config.add_section("security")
    if not config.get("security", "secret_key", fallback="").strip():
        config.set("security", "secret_key", secrets.token_hex(32))
    # Write atomically so a interrupted setup does not destroy the signing key.
    temporary = path.with_suffix(".ini.new")
    with temporary.open("w", encoding="utf-8", newline="\n") as output:
        config.write(output)
    if os.name != "nt":
        temporary.chmod(0o600)
    temporary.replace(path)
    return path


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, default=Path(__file__).resolve().parents[1])
    parser.add_argument("--runtime", type=Path, required=True)
    parser.add_argument("--port", type=int, default=8080)
    parser.add_argument("--bind", default="127.0.0.1")
    parser.add_argument("--public-root", type=Path)
    parser.add_argument("--root-url")
    args = parser.parse_args()
    if not 1 <= args.port <= 65535:
        parser.error("port must be between 1 and 65535")
    print(configure(args.root, args.runtime, args.port, args.bind, args.public_root or args.root / "dist", args.root_url))


if __name__ == "__main__":
    main()
