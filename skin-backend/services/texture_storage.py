"""材质存储服务：图像规范化、尺寸校验、哈希计算、文件落盘。

这是介于 utils 与 backend 之间的纯领域服务，不涉及数据库访问。
"""

import os

from utils.image_utils import (
    validate_texture_dimensions,
    compute_texture_hash_from_image,
    normalize_png,
)


class TextureStorage:
    def __init__(self, textures_dir: str):
        self.textures_dir = textures_dir
        os.makedirs(self.textures_dir, exist_ok=True)

    def process_and_save(self, file_bytes: bytes, texture_type: str) -> str:
        """规范化图像、校验尺寸、计算 hash、落盘，返回 texture_hash。

        校验失败（非 PNG / 尺寸非法）抛 ValueError。
        """
        normalized_bytes, img = normalize_png(file_bytes)

        is_cape = texture_type.lower() == "cape"
        if not validate_texture_dimensions(img, is_cape):
            raise ValueError("Invalid texture dimensions")

        texture_hash = compute_texture_hash_from_image(img)

        file_path = os.path.join(self.textures_dir, f"{texture_hash}.png")
        with open(file_path, "wb") as f:
            f.write(normalized_bytes)

        return texture_hash

    def delete_file(self, texture_hash: str) -> None:
        """物理删除材质文件（幂等）。"""
        file_path = os.path.join(self.textures_dir, f"{texture_hash}.png")
        if os.path.exists(file_path):
            os.remove(file_path)
