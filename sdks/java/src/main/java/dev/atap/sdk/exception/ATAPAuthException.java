package dev.atap.sdk.exception;

import dev.atap.sdk.model.ProblemDetail;

/**
 * Authentication or authorization error (401/403).
 */
public class ATAPAuthException extends ATAPException {

    private final ProblemDetail problem;

    public ATAPAuthException(String message, int statusCode, ProblemDetail problem) {
        super(message, statusCode);
        this.problem = problem;
    }

    public ATAPAuthException(String message, int statusCode) {
        this(message, statusCode, null);
    }

    public ProblemDetail getProblem() {
        return problem;
    }
}
