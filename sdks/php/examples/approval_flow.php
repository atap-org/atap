<?php

/**
 * Example: Create and respond to an approval.
 */

require_once __DIR__ . '/../vendor/autoload.php';

use Atap\Sdk\ATAPClient;
use Atap\Sdk\Model\ApprovalSubject;

// Create an authenticated client
$client = new ATAPClient([
    'base_url' => 'http://localhost:8080',
    'did' => 'did:web:localhost%3A8080:agent:requester',
    'private_key' => '<base64-encoded-ed25519-seed>',
    'client_secret' => 'atap_...',
]);

// 1. Create an approval request
$subject = new ApprovalSubject(
    type: 'data_access',
    label: 'Access user profile data',
    payload: ['scope' => 'read', 'resource' => '/users/123'],
);

$approval = $client->approvals()->create(
    fromDid: 'did:web:localhost%3A8080:agent:requester',
    toDid: 'did:web:localhost%3A8080:human:approver',
    subject: $subject,
);

echo "Created approval: {$approval->id}\n";
echo "  State: {$approval->state}\n";

// 2. List pending approvals (as the approver)
$approvals = $client->approvals()->list();
echo "\nPending approvals: " . count($approvals) . "\n";

// 3. Respond to the approval
$approved = $client->approvals()->respond($approval->id, 'jws-signature-from-approver');
echo "\nApproval responded: {$approved->state}\n";

// 4. Revoke the approval
$revoked = $client->approvals()->revoke($approval->id);
echo "Approval revoked: {$revoked->state}\n";
