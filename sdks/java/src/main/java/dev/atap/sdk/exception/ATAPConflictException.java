package dev.atap.sdk.exception;

import dev.atap.sdk.model.ProblemDetail;

/**
 * Conflict error (409).
 */
public class ATAPConflictException extends ATAPException {

    private final ProblemDetail problem;

    public ATAPConflictException(String message, ProblemDetail problem) {
        super(message, 409);
        this.problem = problem;
    }

    public ATAPConflictException(String message) {
        this(message, null);
    }

    public ProblemDetail getProblem() {
        return problem;
    }
}
