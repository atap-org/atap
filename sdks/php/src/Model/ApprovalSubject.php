<?php

declare(strict_types=1);

namespace Atap\Sdk\Model;

/**
 * The purpose and payload of an approval.
 */
final class ApprovalSubject
{
    /**
     * @param array<string, mixed>|null $payload
     */
    public function __construct(
        public readonly string $type,
        public readonly string $label,
        public readonly bool $reversible = false,
        public readonly ?array $payload = null,
    ) {
    }

    /**
     * @param array<string, mixed> $data
     */
    public static function fromArray(array $data): self
    {
        return new self(
            type: (string) ($data['type'] ?? ''),
            label: (string) ($data['label'] ?? ''),
            reversible: (bool) ($data['reversible'] ?? false),
            payload: $data['payload'] ?? null,
        );
    }

    /**
     * @return array<string, mixed>
     */
    public function toArray(): array
    {
        return [
            'type' => $this->type,
            'label' => $this->label,
            'payload' => $this->payload ?? (object) [],
        ];
    }
}
