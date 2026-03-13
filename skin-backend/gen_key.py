import os
import sys
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric import rsa


def generate_keys(output_dir="."):
    print(f"正在生成 RSA 密钥对 (4096位) 到 {output_dir}... 这可能需要几秒钟。")
    os.makedirs(output_dir, exist_ok=True)

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

    private_path = os.path.join(output_dir, "private.pem")
    with open(private_path, "wb") as f:
        f.write(private_pem)
    print(f"已生成 {private_path} (服务端请妥善保管，不要泄露)")

    # 3. 生成公钥 (public.pem)
    public_key = private_key.public_key()
    public_pem = public_key.public_bytes(
        encoding=serialization.Encoding.PEM,
        format=serialization.PublicFormat.SubjectPublicKeyInfo,
    )

    public_path = os.path.join(output_dir, "public.pem")
    with open(public_path, "wb") as f:
        f.write(public_pem)
    print(f"已生成 {public_path} (用于 API 元数据响应)")


if __name__ == "__main__":
    out = sys.argv[1] if len(sys.argv) > 1 else "."
    generate_keys(out)
