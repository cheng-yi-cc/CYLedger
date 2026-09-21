#!/usr/bin/env python3
"""Create a local user through the upstream CLI without putting a password in shell history."""
import argparse
import getpass
from pathlib import Path
import re
import subprocess
import sys


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--binary", type=Path, required=True)
    parser.add_argument("--config", type=Path, required=True)
    parser.add_argument("--username", required=True)
    parser.add_argument("--email", required=True)
    parser.add_argument("--nickname", default="我的账本")
    args = parser.parse_args()
    password = getpass.getpass("设置登录密码（至少 8 个字符）：")
    if len(password) < 8 or password != getpass.getpass("再次输入密码："):
        raise ValueError("密码不足 8 个字符，或两次输入不一致")
    base = [str(args.binary.resolve()), "--conf-path", str(args.config.resolve()), "--no-boot-log"]
    subprocess.run(base + ["database", "update"], check=True)
    # Upstream prints os.Args on errors. Capture both streams and redact before
    # displaying, so a rejected username does not echo its password to a log.
    result = subprocess.run(base + ["userdata", "user-add", "--username", args.username,
                            "--email", args.email, "--nickname", args.nickname,
                            "--password", password, "--default-currency", "CNY"],
                            capture_output=True, text=True, encoding="utf-8", errors="replace")
    output = (result.stdout + result.stderr).replace(password, "[REDACTED]")
    if result.returncode:
        output = re.sub(r"(?im)^.*\[(?:Password|Salt)\].*$", "[REDACTED]", output)
        print(output, end="")
        raise RuntimeError("用户创建失败；密码已从输出中移除")
    print("账户已创建。公开注册保持关闭，请回到网页登录。")


if __name__ == "__main__":
    try:
        main()
    except (ValueError, RuntimeError, subprocess.CalledProcessError) as exc:
        print(str(exc), file=sys.stderr)
        sys.exit(1)
