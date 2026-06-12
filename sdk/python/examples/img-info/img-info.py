#!/usr/bin/env python3
"""示例工具：图片信息查询 - 用 UniCLI SDK 创建"""
import sys
sys.path.insert(0, r'C:\Users\Administrator\unicli-os\sdk\python')

from unicli import tool

@tool(
    name="img-info",
    description="查询图片基本信息（宽高、格式、大小）",
    inputs={
        "path": {"type": "STRING", "required": True, "desc": "图片路径", "flag": "--path"},
        "detail": {"type": "BOOLEAN", "default": False, "desc": "是否显示详细信息", "flag": "--detail"},
    }
)
def img_info(path: str = "", detail: bool = False):
    """查询图片信息"""
    import os
    if not os.path.exists(path):
        return f"❌ 文件不存在: {path}"

    size = os.path.getsize(path)
    name = os.path.basename(path)

    # Try to get image dimensions (if Pillow available)
    width = height = format_name = "?"
    try:
        from PIL import Image
        img = Image.open(path)
        width, height = img.size
        format_name = img.format or "?"
    except ImportError:
        pass

    result = f"📷 {name}\n"
    result += f"   大小: {_fmt_size(size)}\n"
    result += f"   尺寸: {width} × {height}\n"
    if format_name != "?":
        result += f"   格式: {format_name}\n"

    if detail:
        import time
        mtime = os.path.getmtime(path)
        result += f"   修改: {time.ctime(mtime)}\n"
        ext = os.path.splitext(name)[1].lower()
        ext_map = {'.jpg': 'JPEG', '.png': 'PNG', '.gif': 'GIF', '.webp': 'WebP', '.mp4': 'MP4'}
        result += f"   类型: {ext_map.get(ext, '未知')}\n"

    return result.strip()


def _fmt_size(bytes):
    for unit in ['B', 'KB', 'MB', 'GB']:
        if bytes < 1024:
            return f"{bytes:.1f} {unit}"
        bytes /= 1024
    return f"{bytes:.1f} TB"


if __name__ == "__main__":
    img_info.cli()
