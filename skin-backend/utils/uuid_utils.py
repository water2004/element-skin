"""UUID 生成工具"""

import hashlib
import uuid


def get_offline_uuid(name: str) -> str:
    """
    兼容 Java UUID.nameUUIDFromBytes 的实现
    与标准 Minecraft 离线模式及 authlib-injector 服务端兼容
    """
    data = f"OfflinePlayer:{name}".encode("utf-8")

    # 1. 计算纯 MD5（不使用命名空间，与 Java 实现一致）
    md = hashlib.md5(data).digest()
    md = bytearray(md)

    # 2. 按照 RFC 4122 设置版本号 (Version 3) 和变体 (Variant 1/IETF)
    # Java 的 nameUUIDFromBytes 内部就是这样做的
    md[6] = (md[6] & 0x0F) | 0x30  # Version 3
    md[8] = (md[8] & 0x3F) | 0x80  # Variant 1

    # 3. 转为 UUID 对象并获取字符串
    return str(uuid.UUID(bytes=bytes(md))).replace("-", "")


def generate_random_uuid() -> str:
    """生成随机 UUID（Version 4）"""
    return uuid.uuid4().hex


if __name__ == "__main__":
    print(get_offline_uuid("MinecraftWiki"))
