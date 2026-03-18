import { describe, expect, it } from "vitest";
import {
  ATAPError,
  ATAPProblemError,
  ATAPAuthError,
  ATAPNotFoundError,
  ATAPConflictError,
  ATAPRateLimitError,
} from "../src/errors.js";

describe("ATAPError", () => {
  it("sets message and statusCode", () => {
    const err = new ATAPError("something broke", 500);
    expect(err.message).toBe("something broke");
    expect(err.statusCode).toBe(500);
    expect(err.name).toBe("ATAPError");
  });

  it("defaults statusCode to 0", () => {
    const err = new ATAPError("oops");
    expect(err.statusCode).toBe(0);
  });
});

describe("ATAPProblemError", () => {
  it("uses detail as message", () => {
    const err = new ATAPProblemError({
      type: "about:blank",
      title: "Bad Request",
      status: 400,
      detail: "Missing field",
    });
    expect(err.message).toBe("Missing field");
    expect(err.statusCode).toBe(400);
    expect(err.problem.title).toBe("Bad Request");
  });

  it("uses title when no detail", () => {
    const err = new ATAPProblemError({
      type: "about:blank",
      title: "Internal Error",
      status: 500,
    });
    expect(err.message).toBe("Internal Error");
  });

  it("toString formats correctly", () => {
    const err = new ATAPProblemError({
      type: "about:blank",
      title: "Bad Request",
      status: 400,
      detail: "Missing field",
    });
    expect(err.toString()).toBe("[400] Bad Request: Missing field");
  });

  it("toString with no detail", () => {
    const err = new ATAPProblemError({
      type: "about:blank",
      title: "Error",
      status: 500,
    });
    expect(err.toString()).toBe("[500] Error: ");
  });
});

describe("ATAPAuthError", () => {
  it("sets problem", () => {
    const problem = { type: "about:blank", title: "Unauthorized", status: 401 };
    const err = new ATAPAuthError("bad token", 401, problem);
    expect(err.problem).toEqual(problem);
    expect(err.statusCode).toBe(401);
    expect(err.name).toBe("ATAPAuthError");
  });

  it("defaults statusCode to 401", () => {
    const err = new ATAPAuthError("no token");
    expect(err.statusCode).toBe(401);
  });
});

describe("ATAPNotFoundError", () => {
  it("always has status 404", () => {
    const err = new ATAPNotFoundError("gone");
    expect(err.statusCode).toBe(404);
    expect(err.name).toBe("ATAPNotFoundError");
    expect(err.problem).toBeUndefined();
  });

  it("accepts problem", () => {
    const problem = { type: "about:blank", title: "Not Found", status: 404 };
    const err = new ATAPNotFoundError("gone", problem);
    expect(err.problem).toEqual(problem);
  });
});

describe("ATAPConflictError", () => {
  it("always has status 409", () => {
    const err = new ATAPConflictError("duplicate");
    expect(err.statusCode).toBe(409);
    expect(err.name).toBe("ATAPConflictError");
  });
});

describe("ATAPRateLimitError", () => {
  it("always has status 429", () => {
    const err = new ATAPRateLimitError("slow down");
    expect(err.statusCode).toBe(429);
    expect(err.name).toBe("ATAPRateLimitError");
  });
});
