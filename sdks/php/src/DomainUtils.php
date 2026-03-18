<?php

declare(strict_types=1);

namespace Atap\Sdk;

/**
 * Utility functions for DID domain extraction.
 */
final class DomainUtils
{
    /**
     * Extract platform domain from a DID.
     *
     * did:web:localhost%3A8080:agent:abc -> localhost:8080
     */
    public static function domainFromDid(string $did): string
    {
        $parts = explode(':', $did);
        if (count($parts) < 3) {
            throw new \InvalidArgumentException("Invalid DID format: {$did}");
        }

        $domain = $parts[2];
        $domain = str_replace('%3A', ':', $domain);

        return $domain;
    }
}
