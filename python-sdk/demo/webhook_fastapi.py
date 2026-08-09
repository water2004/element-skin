"""FastAPI Webhook receiver example.

Install optional demo dependencies with ``pip install -e .[webhook-fastapi]``.
``MemoryReplayGuard`` is process-local; replace it with a durable inbox claim in
multi-process or production deployments.
"""

from __future__ import annotations

import os

import uvicorn
from fastapi import FastAPI, HTTPException, Request, Response

from element_skin_sdk import (
    MemoryReplayGuard,
    WebhookError,
    WebhookReplayError,
    WebhookVerifier,
)


def create_app(signing_secret: str) -> FastAPI:
    app = FastAPI()
    verifier = WebhookVerifier(signing_secret)
    replay_guard = MemoryReplayGuard()

    @app.post("/webhooks/element-skin", status_code=204)
    async def receive_element_skin_webhook(request: Request) -> Response:
        raw_body = await request.body()
        try:
            event = verifier.verify_and_claim(raw_body, request.headers, replay_guard)
        except WebhookReplayError:
            return Response(status_code=204)
        except WebhookError as error:
            raise HTTPException(status_code=400, detail=str(error)) from error

        # Production receivers should enqueue the authenticated event durably
        # before returning 2xx, then fetch current resource state through /v2.
        print(event.type, event.id, event.data)
        return Response(status_code=204)

    return app


if __name__ == "__main__":
    secret = os.environ.get("ELEMENT_SKIN_WEBHOOK_SECRET")
    if not secret:
        raise SystemExit("set ELEMENT_SKIN_WEBHOOK_SECRET before starting the demo")
    uvicorn.run(create_app(secret), host="127.0.0.1", port=8080)
