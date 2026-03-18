import type { ProblemDetail } from "./models.js";

/** Base error for all ATAP SDK errors. */
export class ATAPError extends Error {
  public readonly statusCode: number;

  constructor(message: string, statusCode = 0) {
    super(message);
    this.name = "ATAPError";
    this.statusCode = statusCode;
  }
}

/** Error wrapping an RFC 7807 Problem Details response. */
export class ATAPProblemError extends ATAPError {
  public readonly problem: ProblemDetail;

  constructor(problem: ProblemDetail) {
    super(problem.detail || problem.title, problem.status);
    this.name = "ATAPProblemError";
    this.problem = problem;
  }

  override toString(): string {
    return `[${this.problem.status}] ${this.problem.title}: ${this.problem.detail || ""}`;
  }
}

/** Authentication or authorization error (401/403). */
export class ATAPAuthError extends ATAPError {
  public readonly problem?: ProblemDetail;

  constructor(message: string, statusCode = 401, problem?: ProblemDetail) {
    super(message, statusCode);
    this.name = "ATAPAuthError";
    this.problem = problem;
  }
}

/** Resource not found (404). */
export class ATAPNotFoundError extends ATAPError {
  public readonly problem?: ProblemDetail;

  constructor(message: string, problem?: ProblemDetail) {
    super(message, 404);
    this.name = "ATAPNotFoundError";
    this.problem = problem;
  }
}

/** Conflict error (409). */
export class ATAPConflictError extends ATAPError {
  public readonly problem?: ProblemDetail;

  constructor(message: string, problem?: ProblemDetail) {
    super(message, 409);
    this.name = "ATAPConflictError";
    this.problem = problem;
  }
}

/** Rate limit exceeded (429). */
export class ATAPRateLimitError extends ATAPError {
  public readonly problem?: ProblemDetail;

  constructor(message: string, problem?: ProblemDetail) {
    super(message, 429);
    this.name = "ATAPRateLimitError";
    this.problem = problem;
  }
}
