package dev.atap.sdk.api;

import com.fasterxml.jackson.databind.ObjectMapper;
import dev.atap.sdk.ATAPClient;
import dev.atap.sdk.model.Entity;
import dev.atap.sdk.model.KeyVersion;

import java.util.LinkedHashMap;
import java.util.Map;

/**
 * Entity registration, retrieval, deletion, and key rotation.
 */
public class EntityApi {

    private final ATAPClient client;

    public EntityApi(ATAPClient client) {
        this.client = client;
    }

    /**
     * Register a new entity.
     *
     * @param entityType   one of "agent", "machine", "human", "org"
     * @param name         optional display name
     * @param publicKey    optional base64-encoded Ed25519 public key
     * @param principalDid optional DID for agent-to-principal binding
     * @return Entity with id, did, type, name, key_id, and optionally client_secret and private_key
     */
    public Entity register(String entityType, String name, String publicKey, String principalDid) {
        Map<String, Object> body = new LinkedHashMap<>();
        body.put("type", entityType);
        if (name != null && !name.isEmpty()) body.put("name", name);
        if (publicKey != null && !publicKey.isEmpty()) body.put("public_key", publicKey);
        if (principalDid != null && !principalDid.isEmpty()) body.put("principal_did", principalDid);

        Map<String, Object> data = client.getHttpClient().request("POST", "/v1/entities", body, null, null);
        return mapToEntity(data);
    }

    /**
     * Register a new entity with just a type.
     */
    public Entity register(String entityType) {
        return register(entityType, null, null, null);
    }

    /**
     * Get public entity info by ID.
     */
    public Entity get(String entityId) {
        Map<String, Object> data = client.getHttpClient().request("GET", "/v1/entities/" + entityId, null, null, null);
        return mapToEntity(data);
    }

    /**
     * Delete an entity (crypto-shred). Requires atap:manage scope.
     */
    public void delete(String entityId) {
        client.authedRequest("DELETE", "/v1/entities/" + entityId, null, null, null, null);
    }

    /**
     * Rotate an entity's Ed25519 public key. Requires atap:manage scope.
     *
     * @param entityId  the entity ID
     * @param publicKey base64-encoded new Ed25519 public key
     * @return new KeyVersion
     */
    public KeyVersion rotateKey(String entityId, String publicKey) {
        Map<String, Object> body = new LinkedHashMap<>();
        body.put("public_key", publicKey);

        Map<String, Object> data = client.authedRequest("POST",
                "/v1/entities/" + entityId + "/keys/rotate", body, null, null, null);
        return mapToKeyVersion(data);
    }

    @SuppressWarnings("unchecked")
    static Entity mapToEntity(Map<String, Object> data) {
        ObjectMapper mapper = new ObjectMapper();
        return mapper.convertValue(data, Entity.class);
    }

    static KeyVersion mapToKeyVersion(Map<String, Object> data) {
        ObjectMapper mapper = new ObjectMapper();
        return mapper.convertValue(data, KeyVersion.class);
    }
}
