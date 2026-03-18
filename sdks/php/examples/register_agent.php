<?php

/**
 * Example: Register an agent entity and use the SDK.
 */

require_once __DIR__ . '/../vendor/autoload.php';

use Atap\Sdk\ATAPClient;

// 1. Register a new agent (no auth needed for registration)
$client = new ATAPClient([
    'base_url' => 'http://localhost:8080',
]);

$entity = $client->entities()->register('agent', 'My AI Agent');

echo "Registered agent:\n";
echo "  ID:            {$entity->id}\n";
echo "  DID:           {$entity->did}\n";
echo "  Client Secret: {$entity->clientSecret}\n";
echo "  Key ID:        {$entity->keyId}\n";

// 2. Create an authenticated client using the returned credentials
$authedClient = new ATAPClient([
    'base_url' => 'http://localhost:8080',
    'did' => $entity->did,
    'private_key' => $entity->privateKey, // Only if server generated the key
    'client_secret' => $entity->clientSecret,
]);

// 3. Check server health
$health = $authedClient->discovery()->health();
echo "\nServer health: {$health['status']}\n";

// 4. Look up the entity we just created
$fetched = $authedClient->entities()->get($entity->id);
echo "Fetched entity: {$fetched->name} (type: {$fetched->type})\n";
