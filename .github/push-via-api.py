#!/usr/bin/env python3
"""Push unicli-os to GitHub via Contents API (since git push is blocked)."""

import os, json, base64, subprocess, sys
from pathlib import Path

TOKEN = sys.argv[1] if len(sys.argv) > 1 else os.environ.get("GH_TOKEN", "")
REPO = "Guo-Dong-Liang/unicli-os"
BRANCH = "main"
ROOT = Path(r"C:\Users\Administrator\unicli-os")

# Files to skip
SKIP = {
    ".git", ".gitignore", ".github", "bin/unicli", "bin/unicli-windows-amd64.exe",
    "bin/unicli-linux-amd64", "bin/unicli-darwin-amd64", "bin/unicli-darwin-arm64",
    "__pycache__", ".gitattributes",
}
BINARY_EXTS = {".exe", ".png", ".jpg", ".jpeg", ".gif", ".ico", ".woff2", ".ttf"}

def collect_files():
    files = []
    for p in ROOT.rglob("*"):
        rel = p.relative_to(ROOT).as_posix()
        if any(rel.startswith(s) or rel == s for s in SKIP):
            continue
        if p.is_dir():
            continue
        if p.name in (".gitkeep", ".DS_Store", "Thumbs.db"):
            continue
        if "__pycache__" in rel:
            continue
        files.append((rel, p))
    files.sort(key=lambda x: x[0])
    return files

def encode_content(path):
    """Encode file content to base64."""
    raw = path.read_bytes()
    return base64.b64encode(raw).decode()

def create_file(api_url, rel_path, content_b64):
    """Create file via GitHub Contents API."""
    msg = f"feat: add {rel_path}" if rel_path != "LICENSE" else "chore: add MIT license"
    payload = {
        "message": msg,
        "content": content_b64,
        "branch": BRANCH,
    }
    body = json.dumps(payload).encode()
    cmd = [
        "curl", "-s", "-X", "PUT", api_url,
        "-H", f"Authorization: token {TOKEN}",
        "-H", "Accept: application/vnd.github.v3+json",
        "-H", "Content-Type: application/json",
        "--data-binary", "@-",
    ]
    proc = subprocess.run(cmd, input=body, capture_output=True, timeout=30)
    result = json.loads(proc.stdout) if proc.stdout else {}
    status = "OK" if proc.returncode == 0 and result.get("content") else "FAIL"
    return status, result.get("content", {}).get("sha", "")

def main():
    files = collect_files()
    total = len(files)
    print(f"📦 Uploading {total} files to {REPO} ({BRANCH})...\n")

    api_base = f"https://api.github.com/repos/{REPO}/contents"
    ok = 0
    fail = 0

    for i, (rel, path) in enumerate(files, 1):
        api_url = f"{api_base}/{rel}"
        b64 = encode_content(path)
        status, sha = create_file(api_url, rel, b64)
        icon = "✅" if status == "OK" else "❌"
        print(f"  [{i:3d}/{total}] {icon} {rel}")
        if status == "OK":
            ok += 1
        else:
            fail += 1
            print(f"          SHA: {sha}")

    print(f"\n{'='*50}")
    print(f"  ✅ {ok}/{total} files uploaded successfully")
    if fail:
        print(f"  ❌ {fail} files failed")

if __name__ == "__main__":
    main()
