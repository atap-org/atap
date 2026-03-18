<?php

declare(strict_types=1);

namespace Atap\Sdk\Model;

/**
 * DIDComm inbox response.
 */
final class DIDCommInbox
{
    /**
     * @param DIDCommMessage[] $messages
     */
    public function __construct(
        public readonly array $messages = [],
        public readonly int $count = 0,
    ) {
    }
}
