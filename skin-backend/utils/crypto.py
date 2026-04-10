import hashlib
import struct
import base64
from io import BytesIO
from PIL import Image
from cryptography.hazmat.primitives import hashes
from cryptography.hazmat.primitives.asymmetric import padding
from cryptography.hazmat.primitives.serialization import load_pem_private_key

class CryptoUtils:
    def __init__(self, private_key_path: str):
        with open(private_key_path, "rb") as f:
            self.private_key = load_pem_private_key(f.read(), password=None)

    def sign_data(self, data: str) -> str:
        signature = self.private_key.sign(
            data.encode("utf-8"), padding.PKCS1v15(), hashes.SHA1()
        )
        return base64.b64encode(signature).decode("utf-8")

    def get_public_key_pem(self) -> str:
        from cryptography.hazmat.primitives import serialization

        public_key = self.private_key.public_key()
        pem = public_key.public_bytes(
            encoding=serialization.Encoding.PEM,
            format=serialization.PublicFormat.SubjectPublicKeyInfo,
        )
        return pem.decode("utf-8")

    @staticmethod
    def compute_texture_hash_from_image(img: Image.Image) -> str:
        width, height = img.size
        buf = bytearray(width * height * 4 + 8)
        struct.pack_into(">I", buf, 0, width)
        struct.pack_into(">I", buf, 4, height)
        pos = 8
        pixels = img.load()

        for x in range(width):
            for y in range(height):
                r, g, b, a = pixels[x, y]
                if a == 0:
                    r = g = b = 0
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
