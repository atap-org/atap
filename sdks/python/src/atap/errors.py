"""Error types for the ATAP SDK."""

from __future__ import annotations

from typing import Optional

from atap.models import ProblemDetail


class ATAPError(Exception):
    """Base error for all ATAP SDK errors."""

    def __init__(self, message: str, status_code: int = 0) -> None:
        super().__init__(message)
        self.status_code = status_code


class ATAPProblemError(ATAPError):
    """Error wrapping an RFC 7807 Problem Details response."""

    def __init__(self, problem: ProblemDetail) -> None:
        super().__init__(problem.detail or problem.title, problem.status)
        self.problem = problem

    def __str__(self) -> str:
        return f"[{self.problem.status}] {self.problem.title}: {self.problem.detail or ''}"


class ATAPAuthError(ATAPError):
    """Authentication or authorization error (401/403)."""

    def __init__(self, message: str, status_code: int = 401, problem: Optional[ProblemDetail] = None) -> None:
        super().__init__(message, status_code)
        self.problem = problem


class ATAPNotFoundError(ATAPError):
    """Resource not found (404)."""

    def __init__(self, message: str, problem: Optional[ProblemDetail] = None) -> None:
        super().__init__(message, 404)
        self.problem = problem


class ATAPConflictError(ATAPError):
    """Conflict error (409)."""

    def __init__(self, message: str, problem: Optional[ProblemDetail] = None) -> None:
        super().__init__(message, 409)
        self.problem = problem


class ATAPRateLimitError(ATAPError):
    """Rate limit exceeded (429)."""

    def __init__(self, message: str, problem: Optional[ProblemDetail] = None) -> None:
        super().__init__(message, 429)
        self.problem = problem
