package atap

import "fmt"

// ATAPError is the base error for all ATAP SDK errors.
type ATAPError struct {
	Message    string
	StatusCode int
}

func (e *ATAPError) Error() string {
	if e.StatusCode > 0 {
		return fmt.Sprintf("[%d] %s", e.StatusCode, e.Message)
	}
	return e.Message
}

// ATAPProblemError wraps an RFC 7807 Problem Details response.
type ATAPProblemError struct {
	ATAPError
	Problem ProblemDetail
}

func (e *ATAPProblemError) Error() string {
	detail := e.Problem.Detail
	if detail == "" {
		detail = e.Problem.Title
	}
	return fmt.Sprintf("[%d] %s: %s", e.Problem.Status, e.Problem.Title, detail)
}

// ATAPAuthError represents an authentication or authorization error (401/403).
type ATAPAuthError struct {
	ATAPError
	Problem *ProblemDetail
}

// ATAPNotFoundError represents a resource not found error (404).
type ATAPNotFoundError struct {
	ATAPError
	Problem *ProblemDetail
}

// ATAPConflictError represents a conflict error (409).
type ATAPConflictError struct {
	ATAPError
	Problem *ProblemDetail
}

// ATAPRateLimitError represents a rate limit exceeded error (429).
type ATAPRateLimitError struct {
	ATAPError
	Problem *ProblemDetail
}

// NewATAPError creates a new ATAPError.
func NewATAPError(message string, statusCode int) *ATAPError {
	return &ATAPError{Message: message, StatusCode: statusCode}
}

// NewATAPProblemError creates a new ATAPProblemError from a ProblemDetail.
func NewATAPProblemError(problem ProblemDetail) *ATAPProblemError {
	detail := problem.Detail
	if detail == "" {
		detail = problem.Title
	}
	return &ATAPProblemError{
		ATAPError: ATAPError{Message: detail, StatusCode: problem.Status},
		Problem:   problem,
	}
}

// NewATAPAuthError creates a new ATAPAuthError.
func NewATAPAuthError(message string, statusCode int, problem *ProblemDetail) *ATAPAuthError {
	return &ATAPAuthError{
		ATAPError: ATAPError{Message: message, StatusCode: statusCode},
		Problem:   problem,
	}
}

// NewATAPNotFoundError creates a new ATAPNotFoundError.
func NewATAPNotFoundError(message string, problem *ProblemDetail) *ATAPNotFoundError {
	return &ATAPNotFoundError{
		ATAPError: ATAPError{Message: message, StatusCode: 404},
		Problem:   problem,
	}
}

// NewATAPConflictError creates a new ATAPConflictError.
func NewATAPConflictError(message string, problem *ProblemDetail) *ATAPConflictError {
	return &ATAPConflictError{
		ATAPError: ATAPError{Message: message, StatusCode: 409},
		Problem:   problem,
	}
}

// NewATAPRateLimitError creates a new ATAPRateLimitError.
func NewATAPRateLimitError(message string, problem *ProblemDetail) *ATAPRateLimitError {
	return &ATAPRateLimitError{
		ATAPError: ATAPError{Message: message, StatusCode: 429},
		Problem:   problem,
	}
}
