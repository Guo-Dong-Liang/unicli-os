"""
UniCLI SDK: CLI entry point

Usage:
    python -m unicli --help
"""
import argparse
import json
import os
import sys

from .tool import _REGISTRY, get_manifest, list_tools, run_tool


def main():
    parser = argparse.ArgumentParser(description="UniCLI Python SDK")
    parser.add_argument("action", nargs="?", choices=["list", "manifest", "run", "install"], default="list",
                        help="动作: list=列出工具, manifest=生成manifest, run=运行工具, install=安装")
    parser.add_argument("tool_name", nargs="?", help="工具名称")
    parser.add_argument("args", nargs=argparse.REMAINDER, help="工具参数")

    args = parser.parse_args()

    if args.action == "list":
        tools = list_tools()
        if tools:
            print("已注册的工具:")
            for t in tools:
                m = get_manifest(t)
                desc = m.get("description", "") if m else ""
                print(f"  - {t}: {desc}")
        else:
            print("没有已注册的工具。使用 @tool 装饰器注册工具。")

    elif args.action == "manifest":
        if not args.tool_name:
            print("请指定工具名称")
            return
        m = get_manifest(args.tool_name)
        if m:
            print(json.dumps(m, indent=2, ensure_ascii=False))
        else:
            print(f"工具 '{args.tool_name}' 未找到")

    elif args.action == "run":
        if not args.tool_name:
            print("请指定工具名称")
            return
        kwargs = {}
        i = 0
        while i < len(args.args):
            if args.args[i].startswith("--"):
                key = args.args[i].lstrip("-").replace("-", "_")
                if i + 1 < len(args.args) and not args.args[i + 1].startswith("--"):
                    kwargs[key] = args.args[i + 1]
                    i += 2
                else:
                    kwargs[key] = True
                    i += 1
            else:
                i += 1
        try:
            result = run_tool(args.tool_name, **kwargs)
            if result is not None:
                print(result)
        except ValueError as e:
            print(e)

    elif args.action == "install":
        if not args.tool_name:
            print("请指定工具名称")
            return
        m = get_manifest(args.tool_name)
        if not m:
            print(f"工具 '{args.tool_name}' 未找到")
            return

        # Create tool directory
        tool_dir = m["name"]
        os.makedirs(tool_dir, exist_ok=True)

        # Write manifest
        with open(os.path.join(tool_dir, f"{m['name']}.cpl.json"), "w") as f:
            json.dump(m, f, indent=2, ensure_ascii=False)

        # Write entrypoint
        entrypoint = m["image"]["entrypoint"]
        with open(os.path.join(tool_dir, entrypoint), "w") as f:
            f.write(f