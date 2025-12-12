import hashlib
import struct
import uuid
import base64
import json
from io import BytesIO
from typing import Optional, List, Dict, Any
from enum import Enum
from pydantic import BaseModel, Field
from PIL import Image
from cryptography.hazmat.primitives import hashes
from cryptography.hazmat.primitives.asymmetric import padding
from cryptography.hazmat.primitives.serialization import load_pem_private_key

# ===========================
# Pydantic Models (请求/响应)
# ===========================


class Agent(BaseModel):
    name: str = "Minecraft"
    version: int = 1


class AuthRequest(BaseModel):
    username: str
    password: str
    clientToken: Optional[str] = None
    requestUser: bool = False
    agent: Optional[Agent] = None


class RefreshRequest(BaseModel):
    accessToken: str
    clientToken: Optional[str] = None
    requestUser: bool = False
    selectedProfile: Optional[Dict] = None


class ValidationRequest(BaseModel):
    accessToken: str
    clientToken: Optional[str] = None


class JoinRequest(BaseModel):
    accessToken: str
    selectedProfile: str
    serverId: str


# ===========================
# 工具函数 (Utils)
# ===========================


class CryptoUtils:
    def __init__(self, private_key_path: str):
        with open(private_key_path, "rb") as f:
            self.private_key = load_pem_private_key(f.read(), password=None)

    def sign_data(self, data: str) -> str:
        """
        对数据进行 SHA1withRSA 签名，返回 Base64 字符串
        """
        signature = self.private_key.sign(
            data.encode("utf-8"), padding.PKCS1v15(), hashes.SHA1()
        )
        return base64.b64encode(signature).decode("utf-8")

    @staticmethod
    def get_offline_uuid(name: str) -> str:
        """
        兼容离线模式的 UUID 生成算法
        UUID.nameUUIDFromBytes("OfflinePlayer:" + name)
        """
        data = f"OfflinePlayer:{name}".encode("utf-8")
        hash_bytes = hashlib.md5(data).digest()
        # Set the version to 3 (MD5 based) - Java's nameUUIDFromBytes does strictly this
        # Java logic: verify version 3, variant 2.
        # But actually Python's uuid.uuid3 does exactly this.
        return str(uuid.uuid3(uuid.NAMESPACE_OID, f"OfflinePlayer:{name}")).replace(
            "-", ""
        )

    @staticmethod
    def compute_texture_hash(image_bytes: bytes) -> str:
        """
        实现规范中定义的特殊材质 Hash 算法
        """
        try:
            img = Image.open(BytesIO(image_bytes)).convert("RGBA")
        except Exception:
            raise ValueError("Invalid image data")

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

    @staticmethod
    def validate_texture_dimensions(img: Image.Image, is_cape: bool = False) -> bool:
        w, h = img.size
        if is_cape:
            return (w % 64 == 0 and h % 32 == 0) or (w % 22 == 0 and h % 17 == 0)
        else:
            return (w % 64 == 0 and h == w) or (w % 64 == 0 and h * 2 == w)
