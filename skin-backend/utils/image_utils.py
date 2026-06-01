"""图像验证和处理工具"""

import hashlib
import struct
from PIL import Image
from io import BytesIO
from typing import Tuple


# 合法材质的边长上限。Minecraft 皮肤最大为 1024x1024（HD 皮肤），
# 披风更小。设置上限可挡住 6400x6400 这类"整除合法但巨大"的图，
# 避免哈希计算耗尽 CPU。
MAX_TEXTURE_DIMENSION = 1024

# 解压炸弹防护：声明像素数超过此值的图在解码时即抛 DecompressionBombError，
# 由 normalize_png 的 except 统一转成 ValueError。合法材质最多 1024*1024≈1M 像素，
# 这里留出充裕余量（4096*4096≈16M），既不会误伤正常材质，也能拦住超大炸弹图。
Image.MAX_IMAGE_PIXELS = 4096 * 4096


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
    # 上限与正数校验：尺寸非正或超过上限一律拒绝
    if w <= 0 or h <= 0 or w > MAX_TEXTURE_DIMENSION or h > MAX_TEXTURE_DIMENSION:
        return False
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

        # 早期尺寸闸门：Image.open 是惰性的，.size 直接来自 PNG 头(IHDR)，
        # 无需解码像素。任何合法材质（皮肤/披风）边长都不超过 MAX_TEXTURE_DIMENSION，
        # 因此在昂贵的 convert() 解码之前就拒绝超大图，关闭"解码再拒绝"的 DoS 窗口。
        w, h = img.size
        if w <= 0 or h <= 0 or w > MAX_TEXTURE_DIMENSION or h > MAX_TEXTURE_DIMENSION:
            raise ValueError(
                f"Image dimensions {w}x{h} exceed allowed maximum {MAX_TEXTURE_DIMENSION}"
            )

        # 转为 RGBA 并重新保存，去除多余信息
        img = img.convert("RGBA")
        output = BytesIO()
        img.save(output, format="PNG")
        return output.getvalue(), img
    except ValueError:
        # 已是规范化的拒绝原因，原样上抛（避免被下面的兜底包成嵌套消息）
        raise
    except Exception as e:
        # 包含 PIL.Image.DecompressionBombError 等：统一归一化为 ValueError
        raise ValueError(f"Failed to normalize PNG: {str(e)}")
