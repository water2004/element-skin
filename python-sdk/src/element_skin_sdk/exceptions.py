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
    """Raised for a structured non-2xx Element Skin API response."""

    def __init__(
        self,
        status_code: int,
        object: str,
        operation: str,
        reason: str,
        *,
        params: dict[str, object] | None = None,
        response_body: object | None = None,
    ):
        classification = f"{object}.{operation}.{reason}"
        super().__init__(classification)
        self.status_code = status_code
        self.object = object
        self.operation = operation
        self.reason = reason
        self.params = params or {}
        self.response_body = response_body


class AuthenticationError(APIError):
    """Raised when authentication fails."""


class PermissionDenied(APIError):
    """Raised when a request lacks required permissions."""


class NotFound(APIError):
    """Raised when a resource does not exist."""


class OAuthError(ElementSkinError):
    """Raised for OAuth protocol errors."""

    def __init__(self, status_code: int, error: str, *, response_body: object | None = None):
        super().__init__(error)
        self.status_code = status_code
        self.error = error
        self.response_body = response_body


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
