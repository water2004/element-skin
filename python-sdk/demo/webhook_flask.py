"""Flask Webhook receiver example.

Install the optional demo dependency with ``pip install -e .[webhook-flask]``.
``MemoryReplayGuard`` is process-local; replace it with a durable inbox claim in
multi-process or production deployments.
"""

from __future__ import annotations

import os

from flask import Flask, Response, request

from element_skin_sdk import (
    MemoryReplayGuard,
    WebhookError,
    WebhookReplayError,
    WebhookVerifier,
)


def create_app(signing_secret: str) -> Flask:
    app = Flask(__name__)
    verifier = WebhookVerifier(signing_secret)
    replay_guard = MemoryReplayGuard()

    @app.post("/webhooks/element-skin")
    def receive_element_skin_webhook() -> Response:
        try:
            event = verifier.verify_and_claim(
                request.get_data(cache=True), request.headers, replay_guard
            )
        except WebhookReplayError:
            return Response(status=204)
        except WebhookError as error:
            return Response(
                str(error), status=400, content_type="text/plain; charset=utf-8"
            )

        # Production receivers should enqueue the authenticated event durably
        # before returning 2xx, then fetch current resource state through /v2.
        print(event.type, event.id, event.data)
        return Response(status=204)

    return app


if __name__ == "__main__":
    secret = os.environ.get("ELEMENT_SKIN_WEBHOOK_SECRET")
    if not secret:
        raise SystemExit("set ELEMENT_SKIN_WEBHOOK_SECRET before starting the demo")
    create_app(secret).run(host="127.0.0.1", port=8080)
