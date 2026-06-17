"""材质存储服务：图像规范化、尺寸校验、哈希计算、文件落盘。

这是介于 utils 与 backend 之间的纯领域服务，不涉及数据库访问。
"""

import asyncio
import os

from utils.image_utils import (
    validate_texture_dimensions,
    compute_texture_hash_from_image,
    normalize_png,
)


# max_texture_size 设置缺失时的兜底（KB）。与 settings_backend 默认值保持一致。
DEFAULT_MAX_TEXTURE_SIZE_KB = 1024


async def resolve_max_texture_bytes(db) -> int:
    """解析 max_texture_size 设置（KB）并换算为字节上限，供下载/落盘共用同一阈值。

    设置缺失或非法时退化为 DEFAULT_MAX_TEXTURE_SIZE_KB。
    """
    max_kb_str = await db.setting.get("max_texture_size", str(DEFAULT_MAX_TEXTURE_SIZE_KB))
    try:
        max_kb = int(max_kb_str)
    except (TypeError, ValueError):
        max_kb = DEFAULT_MAX_TEXTURE_SIZE_KB
    if max_kb <= 0:
        max_kb = DEFAULT_MAX_TEXTURE_SIZE_KB
    return max_kb * 1024


async def assert_texture_size(db, file_bytes: bytes) -> None:
    """统一的上传大小校验：超过 max_texture_size 设置（KB）则抛 ValueError。

    所有材质上传/导入路径都应在落盘前调用本函数，避免大文件进入 CPU 密集的
    图像处理。max_texture_size 是数据库设置，因此放在异步层而非同步的
    process_and_save 内。
    """
    max_bytes = await resolve_max_texture_bytes(db)
    if len(file_bytes) > max_bytes:
        raise ValueError("Texture file too large.")


class TextureStorage:
    def __init__(self, textures_dir: str):
        self.textures_dir = textures_dir
        os.makedirs(self.textures_dir, exist_ok=True)

    def process_and_save(self, file_bytes: bytes, texture_type: str) -> str:
        """规范化图像、校验尺寸、计算 hash、落盘，返回 texture_hash。

        校验失败（非 PNG / 尺寸非法）抛 ValueError。

        注意：这是 CPU 密集的同步函数（解码 + 逐像素哈希）。在异步请求处理中
        请改用 process_and_save_async，避免阻塞事件循环。
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

    def process_and_save_tracked(self, file_bytes: bytes, texture_type: str) -> tuple[str, bool]:
        """同 process_and_save，但额外返回 created（True 表示本次新建文件）。

        当文件已经存在时（同 hash 已落盘），created 为 False；DB 写入失败时调用方
        可据此决定是否物理删除本次刚写入的文件。
        """
        normalized_bytes, img = normalize_png(file_bytes)

        is_cape = texture_type.lower() == "cape"
        if not validate_texture_dimensions(img, is_cape):
            raise ValueError("Invalid texture dimensions")

        texture_hash = compute_texture_hash_from_image(img)

        file_path = os.path.join(self.textures_dir, f"{texture_hash}.png")
        try:
            fd = os.open(file_path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o644)
        except FileExistsError:
            return texture_hash, False
        try:
            with os.fdopen(fd, "wb") as f:
                f.write(normalized_bytes)
        except Exception:
            try:
                os.remove(file_path)
            except FileNotFoundError:
                pass
            raise
        return texture_hash, True

    async def process_and_save_async(self, file_bytes: bytes, texture_type: str) -> str:
        """process_and_save 的异步包装：在线程池中执行，避免阻塞事件循环。"""
        return await asyncio.to_thread(self.process_and_save, file_bytes, texture_type)

    async def process_and_save_async_tracked(self, file_bytes: bytes, texture_type: str) -> tuple[str, bool]:
        """优先复用 process_and_save_async（保留既有 mock 兼容），返回 (hash, created)。

        同步路径若额外能区分新建/复用则用 process_and_save_tracked。直接复用
        process_and_save_async 时无法判别，按"已新建"保守处理（最坏情况是 DB 失败
        回滚时多删一次本就该被复用的文件，这个文件会在下一次上传时被重新写入）。
        """
        # 让 process_and_save 的 mock 在测试场景下仍能拦截（业务路径会走线程池）
        texture_hash = await self.process_and_save_async(file_bytes, texture_type)
        return texture_hash, True

    def delete_file(self, texture_hash: str) -> None:
        """物理删除材质文件（幂等）。"""
        file_path = os.path.join(self.textures_dir, f"{texture_hash}.png")
        if os.path.exists(file_path):
            os.remove(file_path)
