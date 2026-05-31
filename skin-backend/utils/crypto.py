import base64
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
