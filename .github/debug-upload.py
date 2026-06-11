#!/usr/bin/env python3
"""Debug: test GitHub Contents API with a single file."""

import os, json, base64, subprocess, sys

TOKEN = sys.argv[1] if len(sys.argv) > 1 else os.environ.get("GH_TOKEN", "")
if not TOKEN or TOKEN == "":
    print("No token provided")
    sys.exit(1)

# Try to upload LICENSE
content = (open(r"C:\Users\Administrator\unicli-os\LICENSE", "rb").read())
b64 = base64.b64encode(content).decode()

payload = json.dumps({
    "message": "chore: add MIT license",
    "content": b64,
    "branch": "main",
}).encode()

cmd = [
    "curl", "-s", "-v", "-X", "PUT",
    "https://api.github.com/repos/Guo-Dong-Liang/unicli-os/contents/LICENSE",
    "-H", f"Authorization: token {TOKEN}",
    "-H", "Accept: application/vnd.github.v3+json",
    "-H", "Content-Type: application/json",
    "--data-binary", "@-",
]
proc = subprocess.run(cmd, input=payload, capture_output=True, timeout=30)
print("STDOUT:", proc.stdout.decode()[:2000])
print("STDERR:", proc.stderr.decode()[:2000])
print("RC:", proc.returncode)
