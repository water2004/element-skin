"""集中式日志配置。

各模块统一用 `logging.getLogger(__name__)` 取得 logger；日志级别由
`server.debug` 配置项驱动。在应用初始化早期调用一次 `setup_logging`。
"""

import logging

_configured = False


def setup_logging(debug: bool = False) -> None:
    """配置根 logger 的级别与格式。多次调用仅生效一次。"""
    global _configured
    if _configured:
        return
    logging.basicConfig(
        level=logging.DEBUG if debug else logging.INFO,
        format="%(asctime)s %(levelname)s %(name)s: %(message)s",
    )
    _configured = True
