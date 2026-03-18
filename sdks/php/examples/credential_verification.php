<?php

/**
 * Example: Email/phone/personhood credential verification flow.
 */

require_once __DIR__ . '/../vendor/autoload.php';

use Atap\Sdk\ATAPClient;

$client = new ATAPClient([
    'base_url' => 'http://localhost:8080',
    'did' => 'did:web:localhost%3A8080:human:user123',
    'private_key' => '<base64-encoded-ed25519-seed>',
    'client_secret' => 'atap_...',
]);

// 1. Start email verification
echo "Starting email verification...\n";
$message = $client->credentials()->startEmail('user@example.com');
echo "  {$message}\n";

// 2. Verify email with OTP
$emailCred = $client->credentials()->verifyEmail('user@example.com', '123456');
echo "Email credential issued: {$emailCred->id}\n";
echo "  Type: {$emailCred->type}\n";

// 3. Start phone verification
echo "\nStarting phone verification...\n";
$message = $client->credentials()->startPhone('+1234567890');
echo "  {$message}\n";

// 4. Verify phone with OTP
$phoneCred = $client->credentials()->verifyPhone('+1234567890', '654321');
echo "Phone credential issued: {$phoneCred->id}\n";

// 5. Submit personhood attestation
echo "\nSubmitting personhood attestation...\n";
$personhood = $client->credentials()->submitPersonhood();
echo "Personhood credential issued: {$personhood->id}\n";

// 6. List all credentials
$credentials = $client->credentials()->list();
echo "\nAll credentials ({$credentials} total):\n";
foreach ($credentials as $cred) {
    echo "  - {$cred->type} (issued: {$cred->issuedAt})\n";
}

// 7. Check public status list
$statusList = $client->credentials()->statusList('1');
echo "\nStatus list type: " . implode(', ', $statusList['type'] ?? []) . "\n";
