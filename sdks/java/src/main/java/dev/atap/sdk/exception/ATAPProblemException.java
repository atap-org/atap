package dev.atap.sdk.exception;

import dev.atap.sdk.model.ProblemDetail;

/**
 * Exception wrapping an RFC 7807 Problem Details response.
 */
public class ATAPProblemException extends ATAPException {

    private final ProblemDetail problem;

    public ATAPProblemException(ProblemDetail problem) {
        super(problem.getDetail() != null ? problem.getDetail() : problem.getTitle(), problem.getStatus());
        this.problem = problem;
    }

    public ProblemDetail getProblem() {
        return problem;
    }

    @Override
    public String toString() {
        return "[" + problem.getStatus() + "] " + problem.getTitle() + ": "
                + (problem.getDetail() != null ? problem.getDetail() : "");
    }
}
