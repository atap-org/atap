package dev.atap.sdk.exception;

import dev.atap.sdk.model.ProblemDetail;

/**
 * Rate limit exceeded (429).
 */
public class ATAPRateLimitException extends ATAPException {

    private final ProblemDetail problem;

    public ATAPRateLimitException(String message, ProblemDetail problem) {
        super(message, 429);
        this.problem = problem;
    }

    public ATAPRateLimitException(String message) {
        this(message, null);
    }

    public ProblemDetail getProblem() {
        return problem;
    }
}
