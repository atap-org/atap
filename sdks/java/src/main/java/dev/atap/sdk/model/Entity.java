package dev.atap.sdk.model;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import com.fasterxml.jackson.annotation.JsonProperty;

/**
 * An ATAP entity (agent, machine, human, or org).
 */
@JsonIgnoreProperties(ignoreUnknown = true)
public class Entity {

    @JsonProperty("id")
    private String id;

    @JsonProperty("type")
    private String type;

    @JsonProperty("did")
    private String did;

    @JsonProperty("principal_did")
    private String principalDid;

    @JsonProperty("name")
    private String name;

    @JsonProperty("key_id")
    private String keyId;

    @JsonProperty("public_key")
    private String publicKey;

    @JsonProperty("trust_level")
    private int trustLevel;

    @JsonProperty("registry")
    private String registry;

    @JsonProperty("created_at")
    private String createdAt;

    @JsonProperty("updated_at")
    private String updatedAt;

    @JsonProperty("client_secret")
    private String clientSecret;

    @JsonProperty("private_key")
    private String privateKey;

    public Entity() {
    }

    public String getId() { return id; }
    public void setId(String id) { this.id = id; }

    public String getType() { return type; }
    public void setType(String type) { this.type = type; }

    public String getDid() { return did; }
    public void setDid(String did) { this.did = did; }

    public String getPrincipalDid() { return principalDid; }
    public void setPrincipalDid(String principalDid) { this.principalDid = principalDid; }

    public String getName() { return name; }
    public void setName(String name) { this.name = name; }

    public String getKeyId() { return keyId; }
    public void setKeyId(String keyId) { this.keyId = keyId; }

    public String getPublicKey() { return publicKey; }
    public void setPublicKey(String publicKey) { this.publicKey = publicKey; }

    public int getTrustLevel() { return trustLevel; }
    public void setTrustLevel(int trustLevel) { this.trustLevel = trustLevel; }

    public String getRegistry() { return registry; }
    public void setRegistry(String registry) { this.registry = registry; }

    public String getCreatedAt() { return createdAt; }
    public void setCreatedAt(String createdAt) { this.createdAt = createdAt; }

    public String getUpdatedAt() { return updatedAt; }
    public void setUpdatedAt(String updatedAt) { this.updatedAt = updatedAt; }

    public String getClientSecret() { return clientSecret; }
    public void setClientSecret(String clientSecret) { this.clientSecret = clientSecret; }

    public String getPrivateKey() { return privateKey; }
    public void setPrivateKey(String privateKey) { this.privateKey = privateKey; }
}
