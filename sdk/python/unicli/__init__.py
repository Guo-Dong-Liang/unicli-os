"""
UniCLI Python SDK - 用装饰器快速创建 CPL 兼容的 CLI 工具

用法:
    from unicli import tool, run

    @tool(name="hello", description="打招呼")
    def say_hello(name: str = "World", greeting: str = "你好"):
        print(f"{greeting}, {name}!")
        return f"{greeting}, {name}!"

    if __name__ == "__main__":
        say_hello()
"""
from .tool import tool, get_manifest, list_tools, run_tool

__all__ = ["tool", "get_manifest", "list_tools", "run_tool"]
