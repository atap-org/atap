<?php

declare(strict_types=1);

namespace Atap\Sdk\Exception;

use Atap\Sdk\Model\ProblemDetail;

/**
 * Conflict error (409).
 */
class ATAPConflictException extends ATAPException
{
    public function __construct(
        string $message = 'Conflict',
        public readonly ?ProblemDetail $problem = null,
        ?\Throwable $previous = null,
    ) {
        parent::__construct($message, 409, $previous);
    }
}
