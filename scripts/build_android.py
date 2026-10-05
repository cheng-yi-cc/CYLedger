#!/usr/bin/env python3
"""Build the offline Android APK using the installed Android SDK, NDK and Go."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import secrets
import shutil
import subprocess
import zipfile

ROOT = Path(__file__).resolve().parent.parent


def run(arguments, *, env=None):
    print("Running " + Path(str(arguments[0])).name, flush=True)
    subprocess.run([str(arg) for arg in arguments], cwd=ROOT, env=env, check=True)


def latest(directory, pattern):
    candidates = list(directory.glob(pattern))
    if not candidates:
        raise SystemExit(f"Required development dependency not found in {directory}")
    return max(candidates, key=lambda path: tuple(int(v) for v in path.name.replace("android-", "").split(".") if v.isdigit()))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--qa", action="store_true", help="Separate app/data, with WebView debugging")
    parser.add_argument("--skip-frontend", action="store_true", help="Package an already built dist directory")
    parser.add_argument("--skip-backend", action="store_true", help="Reuse already built native libraries")
    parser.add_argument("--sdk", type=Path, default=Path(os.environ.get("ANDROID_HOME", str(Path.home() / "AppData/Local/Android/Sdk"))))
    parser.add_argument("--ndk", type=Path, default=Path("D:/tools/cyledger/android-sdk/ndk/28.2.13676358"))
    args = parser.parse_args()
    tools = latest(args.sdk / "build-tools", "*")
    platform = latest(args.sdk / "platforms", "android-*") / "android.jar"
    go = shutil.which("go") or "D:/tools/cyledger/go/bin/go.exe"
    java = shutil.which("javac")
    keytool = shutil.which("keytool")
    if not java or not keytool or not platform.is_file():
        raise SystemExit("Java JDK and an Android platform SDK are required")
    compiler = args.ndk / "toolchains/llvm/prebuilt/windows-x86_64/bin/aarch64-linux-android26-clang.cmd"
    if not compiler.is_file():
        raise SystemExit("Install Android NDK 28.2.13676358, or provide --ndk")
    variant = "qa" if args.qa else "release"
    build = ROOT / ".runtime/android" / variant
    native = ROOT / ".runtime/android/native"
    assets = build / "assets"
    resources = build / "res"
    classes = build / "classes"
    dex = build / "dex"
    for folder in (native, assets, resources, classes, dex):
        folder.mkdir(parents=True, exist_ok=True)
    if not args.skip_frontend:
        run([shutil.which("npm.cmd") or "npm", "run", "build"])
    if not (ROOT / "dist/mobile.html").is_file():
        raise SystemExit("Build the frontend first (npm run build)")
    if not args.skip_backend:
        env = os.environ.copy()
        env.update(GOOS="android", GOARCH="arm64", CGO_ENABLED="1", CC=str(compiler),
                   CGO_LDFLAGS="-Wl,-z,max-page-size=16384")
        run([go, "build", "-trimpath", "-buildmode=c-shared", "-ldflags=-s -w",
             "-o", native / "libcyledger_backend.so", "./android/backend"], env=env)
        run([compiler, "-shared", "-fPIC", "-O2", "-Wl,-z,max-page-size=16384",
             "-Wl,-soname,libcyledger_jni.so", "-I", native,
             ROOT / "android/native/jni.c", "-L", native, "-lcyledger_backend",
             "-o", native / "libcyledger_jni.so"])
    digest = hashlib.sha256()
    with zipfile.ZipFile(assets / "frontend.zip", "w", zipfile.ZIP_DEFLATED) as archive:
        for path in sorted((ROOT / "dist").rglob("*")):
            if path.is_file() and path.relative_to(ROOT / "dist").parts[0] != "android":
                name = path.relative_to(ROOT / "dist").as_posix()
                data = path.read_bytes()
                digest.update(name.encode())
                digest.update(data)
                archive.writestr(name, data)
    default_config = ROOT / "conf/ezbookkeeping.ini"
    digest.update(default_config.read_bytes())
    (assets / "revision.txt").write_text(digest.hexdigest(), encoding="utf-8")
    shutil.copyfile(default_config, assets / "defaults.ini")
    # License notices travel with the app; no user runtime files enter the APK.
    notices = assets / "licenses"
    notices.mkdir(exist_ok=True)
    for path in [ROOT / "LICENSE", ROOT / "NOTICE", *sorted((ROOT / "licenses").glob("*"))]:
        if path.is_file():
            shutil.copyfile(path, notices / path.name)
    shutil.copytree(ROOT / "android/app/src/main/res", resources, dirs_exist_ok=True)
    (resources / "drawable").mkdir(exist_ok=True)
    shutil.copyfile(ROOT / "public/favicon.png", resources / "drawable/icon.png")
    manifest_text = (ROOT / "android/app/src/main/AndroidManifest.xml").read_text(encoding="utf-8")
    application_id = "com.cyledger.android.qa" if args.qa else "com.cyledger.android"
    manifest_text = manifest_text.replace("@APPLICATION_ID@", application_id)
    manifest_text = manifest_text.replace("@APP_LABEL@", "CYLedger 测试" if args.qa else "CYLedger")
    manifest_text = manifest_text.replace("@DEBUGGABLE@", "true" if args.qa else "false")
    manifest = build / "AndroidManifest.xml"
    manifest.write_text(manifest_text, encoding="utf-8")
    compiled_resources = build / "resources.zip"
    run([tools / "aapt2.exe", "compile", "--dir", resources, "-o", compiled_resources])
    unsigned = build / "unsigned.apk"
    run([tools / "aapt2.exe", "link", "-o", unsigned, "--manifest", manifest,
         "-I", platform, "-A", assets, "--version-code", "2", "--version-name", "0.1.1", compiled_resources])
    sources = sorted((ROOT / "android/app/src/main/java").rglob("*.java"))
    bootclasspath = os.pathsep.join([str(tools / "core-lambda-stubs.jar"), str(platform)])
    run([java, "-encoding", "UTF-8", "-source", "8", "-target", "8", "-bootclasspath", bootclasspath,
         "-d", classes, *sources])
    class_files = sorted(classes.rglob("*.class"))
    run([tools / "d8.bat", "--min-api", "26", "--lib", platform, "--output", dex, *class_files])
    with zipfile.ZipFile(unsigned, "a", zipfile.ZIP_DEFLATED) as archive:
        for path in sorted(dex.glob("*.dex")):
            archive.write(path, path.name)
        for name in ("libcyledger_backend.so", "libcyledger_jni.so"):
            archive.write(native / name, "lib/arm64-v8a/" + name, compress_type=zipfile.ZIP_STORED)
    aligned = build / "aligned.apk"
    run([tools / "zipalign.exe", "-f", "-P", "16", "4", unsigned, aligned])
    signing = ROOT / ".runtime/android-signing"
    signing.mkdir(exist_ok=True)
    password_file = signing / "password.txt"
    keystore = signing / "cyledger.p12"
    if not keystore.exists():
        password_file.write_text(secrets.token_hex(32), encoding="ascii")
        run([keytool, "-genkeypair", "-keystore", keystore, "-storetype", "PKCS12", "-alias", "cyledger",
             "-storepass:file", password_file, "-keypass:file", password_file,
             "-keyalg", "RSA", "-keysize", "3072", "-validity", "10000", "-dname", "CN=CYLedger Local Build"])
    if not password_file.is_file():
        raise SystemExit("Signing password is missing; preserve the existing key for APK updates")
    output = ROOT / "dist/android"
    output.mkdir(exist_ok=True)
    apk = output / ("CYLedger-Android-arm64-qa.apk" if args.qa else "CYLedger-Android-arm64.apk")
    run([tools / "apksigner.bat", "sign", "--ks", keystore, "--ks-pass", "file:" + str(password_file),
         "--ks-key-alias", "cyledger", "--out", apk, aligned])
    run([tools / "apksigner.bat", "verify", "--verbose", apk])
    run([tools / "zipalign.exe", "-c", "-P", "16", "4", apk])
    result = {"variant": variant, "applicationId": application_id, "apk": str(apk),
              "sha256": hashlib.sha256(apk.read_bytes()).hexdigest(), "bytes": apk.stat().st_size,
              "architecture": "arm64-v8a", "minimumAndroid": "8.0", "targetSdk": 36}
    (build / "build-result.json").write_text(json.dumps(result, ensure_ascii=False, indent=2), encoding="utf-8")
    print(json.dumps(result, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
