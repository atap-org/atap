package dev.atap.sdk.exception;

import dev.atap.sdk.model.ProblemDetail;

/**
 * Resource not found (404).
 */
public class ATAPNotFoundException extends ATAPException {

    private final ProblemDetail problem;

    public ATAPNotFoundException(String message, ProblemDetail problem) {
        super(message, 404);
        this.problem = problem;
    }

    public ATAPNotFoundException(String message) {
        this(message, null);
    }

    public ProblemDetail getProblem() {
        return problem;
    }
}
