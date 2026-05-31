"""PlayerProfile 序列化与模型归一化的纯函数测试"""

from utils.typing import (
    PlayerProfile,
    normalize_texture_model,
    serialize_profile_summary,
)


def test_normalize_texture_model_slim_passthrough():
    assert normalize_texture_model("slim") == "slim"


def test_normalize_texture_model_everything_else_is_default():
    assert normalize_texture_model("default") == "default"
    assert normalize_texture_model("classic") == "default"
    assert normalize_texture_model("") == "default"
    assert normalize_texture_model("SLIM") == "default"  # 大小写敏感，仅精确 'slim'


def test_serialize_profile_summary_maps_fields():
    p = PlayerProfile("pid", "uid", "Steve", "slim", "skinhash", "capehash")
    assert serialize_profile_summary(p) == {
        "id": "pid",
        "name": "Steve",
        "model": "slim",
        "skin_hash": "skinhash",
        "cape_hash": "capehash",
    }


def test_serialize_profile_summary_handles_missing_textures():
    p = PlayerProfile("pid", "uid", "Alex")
    summary = serialize_profile_summary(p)
    assert summary["model"] == "default"
    assert summary["skin_hash"] is None
    assert summary["cape_hash"] is None
    # user_id 不应泄漏到列表项
    assert "user_id" not in summary
