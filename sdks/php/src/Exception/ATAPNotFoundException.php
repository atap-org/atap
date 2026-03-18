<?php

declare(strict_types=1);

namespace Atap\Sdk\Exception;

use Atap\Sdk\Model\ProblemDetail;

/**
 * Resource not found (404).
 */
class ATAPNotFoundException extends ATAPException
{
    public function __construct(
        string $message = 'Not found',
        public readonly ?ProblemDetail $problem = null,
        ?\Throwable $previous = null,
    ) {
        parent::__construct($message, 404, $previous);
    }
}
