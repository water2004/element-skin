"""图像验证和处理工具"""

import hashlib
import struct
from PIL import Image
from io import BytesIO
from typing import Tuple


def validate_texture_dimensions(img: Image.Image, is_cape: bool = False) -> bool:
    """
    验证材质尺寸是否合法

    Args:
        img: PIL Image 对象
        is_cape: 是否为披风材质

    Returns:
        bool: 尺寸是否合法
    """
    w, h = img.size
    if is_cape:
        return (w % 64 == 0 and h % 32 == 0) or (w % 22 == 0 and h % 17 == 0)
    else:
        return (w % 64 == 0 and h == w) or (w % 64 == 0 and h * 2 == w)


def compute_texture_hash_from_image(img: Image.Image) -> str:
    """
    实现规范中定义的特殊材质 Hash 算法：基于像素数据的SHA-256
    规范要求计算缓冲区 (width, height, pixels) 的 SHA-256，而非 PNG 文件字节

    Args:
        img: PIL Image 对象（RGBA模式）

    Returns:
        str: 材质 hash（SHA-256 十六进制字符串）
    """
    width, height = img.size
    # 缓冲区大小: w * h * 4 + 8
    buf = bytearray(width * height * 4 + 8)

    # 写入宽和高 (Big-Endian)
    struct.pack_into(">I", buf, 0, width)
    struct.pack_into(">I", buf, 4, height)

    pos = 8
    pixels = img.load()

    for x in range(width):
        for y in range(height):
            r, g, b, a = pixels[x, y]
            # 规范：若 Alpha 为 0，则 RGB 皆处理为 0
            if a == 0:
                r = g = b = 0

            # 写入 ARGB
            buf[pos] = a
            buf[pos + 1] = r
            buf[pos + 2] = g
            buf[pos + 3] = b
            pos += 4

    return hashlib.sha256(buf).hexdigest()


def normalize_png(image_bytes: bytes) -> Tuple[bytes, Image.Image]:
    """
    规范化 PNG 图像，移除多余数据

    Args:
        image_bytes: 原始 PNG 字节

    Returns:
        Tuple[bytes, Image.Image]: (规范化后的字节, PIL Image对象)

    Raises:
        ValueError: 无效的图像数据
    """
    try:
        img = Image.open(BytesIO(image_bytes))
        if img.format != "PNG":
            raise ValueError("Image must be PNG format")

        # 转为 RGBA 并重新保存，去除多余信息
        img = img.convert("RGBA")
        output = BytesIO()
        img.save(output, format="PNG")
        return output.getvalue(), img
    except Exception as e:
        raise ValueError(f"Failed to normalize PNG: {str(e)}")
