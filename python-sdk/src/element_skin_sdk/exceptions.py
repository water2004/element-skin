"""SDK exception hierarchy."""


class ElementSkinError(Exception):
    """Base class for all SDK errors."""


class ValidationError(ElementSkinError):
    """Raised when local input validation fails."""


class InvalidScope(ValidationError):
    """Raised when requested permissions are invalid for a flow."""

    def __init__(self, message: str, invalid_scopes: list[str] | None = None):
        super().__init__(message)
        self.invalid_scopes = invalid_scopes or []


class APIError(ElementSkinError):
    """Raised for non-2xx HTTP API responses."""

    def __init__(
        self,
        status_code: int,
        detail: str,
        *,
        response_body: object | None = None,
        error: str | None = None,
    ):
        super().__init__(detail)
        self.status_code = status_code
        self.detail = detail
        self.response_body = response_body
        self.error = error


class AuthenticationError(APIError):
    """Raised when authentication fails."""


class PermissionDenied(APIError):
    """Raised when a request lacks required permissions."""


class NotFound(APIError):
    """Raised when a resource does not exist."""


class OAuthError(APIError):
    """Raised for OAuth protocol errors."""


class WebhookError(ValidationError):
    """Base class for Webhook verification errors."""


class WebhookHeaderError(WebhookError):
    """Raised when required Webhook headers are missing or malformed."""


class WebhookSignatureError(WebhookError):
    """Raised when the Webhook signature is malformed or invalid."""


class WebhookTimestampError(WebhookError):
    """Raised when the Webhook timestamp is malformed or outside the allowed window."""


class WebhookPayloadError(WebhookError):
    """Raised when the authenticated Webhook payload violates the event contract."""


class WebhookReplayError(WebhookError):
    """Raised when a replay guard has already claimed the delivery."""
