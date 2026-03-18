<?php

declare(strict_types=1);

namespace Atap\Sdk\Exception;

use Atap\Sdk\Model\ProblemDetail;

/**
 * Rate limit exceeded (429).
 */
class ATAPRateLimitException extends ATAPException
{
    public function __construct(
        string $message = 'Rate limit exceeded',
        public readonly ?ProblemDetail $problem = null,
        ?\Throwable $previous = null,
    ) {
        parent::__construct($message, 429, $previous);
    }
}
