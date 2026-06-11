"""
UniCLI tool decorator and runtime.
Auto-generates CPL manifest and CLI interface.
"""
import argparse
import inspect
import json
import os
import sys
from functools import wraps
from pathlib import Path

# Registry to store all decorated tools
_REGISTRY = {}


def tool(name=None, description="", inputs=None, outputs=None, version="1.0.0"):
    """Decorator that converts a Python function into a UniCLI-compatible tool.

    Args:
        name: Tool name (default: function name)
        description: Tool description
        inputs: Dict defining input parameters (auto-detected from function signature)
        outputs: Dict defining output (auto-detected from return)
        version: Tool version

    Usage:
        @tool(name="hello", description="打招呼")
        def say_hello(name: str = "World"):
            print(f"Hello, {name}!")
    """
    def decorator(func):
        tool_name = name or func.__name__

        # Auto-detect inputs from function signature
        sig = inspect.signature(func)
        detected_inputs = {}
        for param_name, param in sig.parameters.items():
            detected_inputs[param_name] = {
                "type": _py_type_to_cpl(param.annotation),
                "required": param.default is inspect.Parameter.empty,
                "default": "" if param.default is inspect.Parameter.empty else param.default,
                "desc": "",
                "flag": f"--{param_name.replace('_', '-')}",
            }

        # Merge with user-provided inputs (overrides auto-detected)
        merged_inputs = detected_inputs
        if inputs:
            for k, v in inputs.items():
                if k in merged_inputs:
                    merged_inputs[k].update(v)
                else:
                    merged_inputs[k] = v

        # Build manifest
        manifest = {
            "cpl_version": "1.0.0",
            "name": tool_name,
            "version": version,
            "description": description or func.__doc__ or tool_name,
            "author": "UniCLI SDK",
            "inputs": [],
            "outputs": [{
                "name": "output",
                "type": outputs.get("type", "TEXT") if outputs else "TEXT",
                "description": outputs.get("description", "工具输出") if outputs else "工具输出",
                "capture_stdout": True
            }],
            "resources": {"cpu": 1, "memory": 256, "network": False, "gpu": False, "timeout": 60, "disk": 128},
            "image": {
                "ref": f"{os.environ.get('IMAGE_REGISTRY', 'ghcr.io/unixcli')}/{tool_name}:{version}",
                "entrypoint": f"{tool_name}.py",
                "workdir": "/workspace",
                "user": "nobody:nogroup"
            }
        }

        for pname, pinfo in merged_inputs.items():
            inp = {
                "name": pname,
                "type": pinfo.get("type", "STRING"),
                "required": pinfo.get("required", False),
                "description": pinfo.get("desc", f"{pname} 参数"),
                "flag": pinfo.get("flag", f"--{pname}"),
            }
            default = pinfo.get("default")
            if default is not None and default != "":
                inp["default"] = default
            manifest["inputs"].append(inp)

        # Store manifest on function
        func._unicli_manifest = manifest
        func._unicli_name = tool_name

        _REGISTRY[tool_name] = func

        @wraps(func)
        def wrapper(*args, **kwargs):
            return func(*args, **kwargs)

        wrapper._unicli_manifest = manifest
        wrapper._unicli_name = tool_name

        # Add CLI support
        @wraps(func)
        def cli_main():
            """CLI entry point for the tool"""
            parser = argparse.ArgumentParser(description=description or func.__doc__)
            for pname, pinfo in merged_inputs.items():
                flag = pinfo.get("flag", f"--{pname}")
                default = pinfo.get("default")
                required = pinfo.get("required", False)
                help_text = pinfo.get("desc", "")
                typ = _argparse_type(pinfo.get("type", "STRING"))

                if required:
                    parser.add_argument(flag, type=typ, required=True, help=help_text)
                elif default is not None and default != "":
                    parser.add_argument(flag, type=typ, default=default, help=f"{help_text} (默认: {default})")
                else:
                    parser.add_argument(flag, type=typ, default=None, help=help_text)

            parser.add_argument("--generate-manifest", action="store_true", help="生成 CPL manifest 文件")
            parser.add_argument("--install", action="store_true", help="安装到 UniCLI 注册表")
            args = parser.parse_args()

            if args.generate_manifest:
                # Output manifest to stdout
                print(json.dumps(manifest, indent=2, ensure_ascii=False))
                return

            # Map CLI args -> function kwargs
            kwargs = {}
            for pname in merged_inputs:
                flag_name = pname.replace("-", "_")
                if hasattr(args, flag_name):
                    val = getattr(args, flag_name)
                    if val is not None:
                        kwargs[pname] = val

            # Run the tool
            result = func(**kwargs)
            if result is not None:
                print(result)

        wrapper.cli = cli_main

        # Replace the original function with the wrapper
        return wrapper

    return decorator


def _py_type_to_cpl(annotation):
    """Convert Python type annotation to CPL type"""
    mapping = {
        int: "INT",
        float: "FLOAT",
        bool: "BOOLEAN",
        str: "STRING",
        list: "STREAM",
        Path: "FILE",
        bytes: "FILE",
        "INT": "INT",
        "FLOAT": "FLOAT",
        "BOOLEAN": "BOOLEAN",
        "STRING": "STRING",
    }
    if annotation in mapping:
        return mapping[annotation]
    if annotation is inspect.Parameter.empty:
        return "STRING"
    return "STRING"


def _argparse_type(cpl_type):
    """Convert CPL type to argparse type"""
    mapping = {"INT": int, "FLOAT": float, "BOOLEAN": bool}
    return mapping.get(cpl_type, str)


def get_manifest(func_or_name):
    """Get the CPL manifest for a tool function or name"""
    if callable(func_or_name):
        return getattr(func_or_name, "_unicli_manifest", None)
    func = _REGISTRY.get(func_or_name)
    return getattr(func, "_unicli_manifest", None) if func else None


def list_tools():
    """List all registered tools"""
    return list(_REGISTRY.keys())


def run_tool(name, **kwargs):
    """Run a tool programmatically"""
    func = _REGISTRY.get(name)
    if not func:
        raise ValueError(f"Tool '{name}' not found. Available: {list(_REGISTRY.keys())}")
    return func(**kwargs)
