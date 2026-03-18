<?php

declare(strict_types=1);

namespace Atap\Sdk\Exception;

use Atap\Sdk\Model\ProblemDetail;

/**
 * Error wrapping an RFC 7807 Problem Details response.
 */
class ATAPProblemException extends ATAPException
{
    public function __construct(
        public readonly ProblemDetail $problem,
        ?\Throwable $previous = null,
    ) {
        parent::__construct(
            $problem->detail ?? $problem->title,
            $problem->status,
            $previous,
        );
    }

    public function __toString(): string
    {
        return sprintf('[%d] %s: %s', $this->problem->status, $this->problem->title, $this->problem->detail ?? '');
    }
}
