package dev.atap.sdk.exception;

/**
 * Base exception for all ATAP SDK errors.
 */
public class ATAPException extends RuntimeException {

    private final int statusCode;

    public ATAPException(String message) {
        this(message, 0);
    }

    public ATAPException(String message, int statusCode) {
        super(message);
        this.statusCode = statusCode;
    }

    public ATAPException(String message, int statusCode, Throwable cause) {
        super(message, cause);
        this.statusCode = statusCode;
    }

    public int getStatusCode() {
        return statusCode;
    }
}
