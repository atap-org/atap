<?php

declare(strict_types=1);

namespace Atap\Sdk\Model;

/**
 * A W3C DID Document.
 */
final class DIDDocument
{
    /**
     * @param string[] $context
     * @param VerificationMethod[] $verificationMethod
     * @param string[] $authentication
     * @param string[] $assertionMethod
     * @param string[] $keyAgreement
     * @param array<int, array<string, mixed>> $service
     */
    public function __construct(
        public readonly string $id,
        public readonly array $context = [],
        public readonly array $verificationMethod = [],
        public readonly array $authentication = [],
        public readonly array $assertionMethod = [],
        public readonly array $keyAgreement = [],
        public readonly array $service = [],
        public readonly string $atapType = '',
        public readonly string $atapPrincipal = '',
    ) {
    }

    /**
     * @param array<string, mixed> $data
     */
    public static function fromArray(array $data): self
    {
        $vms = [];
        foreach ($data['verificationMethod'] ?? [] as $vm) {
            $vms[] = VerificationMethod::fromArray($vm);
        }

        return new self(
            id: (string) ($data['id'] ?? ''),
            context: $data['@context'] ?? [],
            verificationMethod: $vms,
            authentication: $data['authentication'] ?? [],
            assertionMethod: $data['assertionMethod'] ?? [],
            keyAgreement: $data['keyAgreement'] ?? [],
            service: $data['service'] ?? [],
            atapType: (string) ($data['atap:type'] ?? ''),
            atapPrincipal: (string) ($data['atap:principal'] ?? ''),
        );
    }
}
