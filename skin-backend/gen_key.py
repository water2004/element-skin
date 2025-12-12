from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric import rsa


def generate_keys():
    print("正在生成 RSA 密钥对 (4096位)... 这可能需要几秒钟。")

    # 1. 生成私钥
    private_key = rsa.generate_private_key(
        public_exponent=65537,
        key_size=4096,
    )

    # 2. 序列化私钥 (private.pem)
    private_pem = private_key.private_bytes(
        encoding=serialization.Encoding.PEM,
        format=serialization.PrivateFormat.PKCS8,
        encryption_algorithm=serialization.NoEncryption(),
    )

    with open("private.pem", "wb") as f:
        f.write(private_pem)
    print("已生成 private.pem (服务端请妥善保管，不要泄露)")

    # 3. 生成公钥 (public.pem)
    public_key = private_key.public_key()
    public_pem = public_key.public_bytes(
        encoding=serialization.Encoding.PEM,
        format=serialization.PublicFormat.SubjectPublicKeyInfo,
    )

    with open("public.pem", "wb") as f:
        f.write(public_pem)
    print("已生成 public.pem (用于 API 元数据响应)")


if __name__ == "__main__":
    generate_keys()
