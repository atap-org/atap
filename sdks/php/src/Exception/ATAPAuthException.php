<?php

declare(strict_types=1);

namespace Atap\Sdk\Exception;

use Atap\Sdk\Model\ProblemDetail;

/**
 * Authentication or authorization error (401/403).
 */
class ATAPAuthException extends ATAPException
{
    public function __construct(
        string $message = 'Authentication failed',
        int $statusCode = 401,
        public readonly ?ProblemDetail $problem = null,
        ?\Throwable $previous = null,
    ) {
        parent::__construct($message, $statusCode, $previous);
    }
}
