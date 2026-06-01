"""密码哈希和验证工具"""

import bcrypt
import re

def hash_password(password: str) -> str:
    """
    使用 bcrypt 哈希密码

    Args:
        password: 明文密码

    Returns:
        str: 哈希后的密码字符串
    """
    return bcrypt.hashpw(password.encode("utf-8"), bcrypt.gensalt()).decode("utf-8")


def verify_password(password: str, hashed: str) -> bool:
    """
    验证密码

    Args:
        password: 明文密码
        hashed: 哈希后的密码

    Returns:
        bool: 密码是否正确
    """
    # bcrypt 密码验证
    if hashed.startswith("$2"):
        try:
            return bcrypt.checkpw(password.encode("utf-8"), hashed.encode("utf-8"))
        except Exception:
            return False
    # 兼容旧的明文密码
    return hashed == password


def needs_rehash(hashed: str) -> bool:
    """
    检查密码哈希是否需要升级

    Args:
        hashed: 哈希后的密码

    Returns:
        bool: 是否需要重新哈希（从明文升级到bcrypt）
    """
    return not hashed.startswith("$2")

def validate_strong_password(password: str) -> list[str]:
    """校验密码强度，返回错误信息列表（空列表表示通过）。

    - 长度至少 8 位；
    - 不超过 72 字节：bcrypt 只取前 72 字节，超出部分被静默忽略，
      明确拒绝可避免"用户以为设置了长密码、实际只生效前 72 字节"的误解；
    - 至少包含两类字符（大写/小写/数字/特殊），避免 `aaa111` 这类弱口令通过。
    """
    errors: list[str] = []
    if len(password) < 8:
        errors.append("密码长度至少8位")

    # bcrypt 以字节计长度，中文等多字节字符更易超限
    if len(password.encode("utf-8")) > 72:
        errors.append("密码过长（不超过72字节）")

    has_upper = bool(re.search(r"[A-Z]", password))
    has_lower = bool(re.search(r"[a-z]", password))
    has_digit = bool(re.search(r"\d", password))
    has_special = bool(re.search(r"[^\w\s]", password))
    classes = has_upper + has_lower + has_digit + has_special

    if classes < 2:
        errors.append("密码需包含大写、小写、数字、特殊字符中的至少两类")

    return errors