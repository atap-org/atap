package dev.atap.sdk.model;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import com.fasterxml.jackson.annotation.JsonProperty;

/**
 * A versioned public key for an entity.
 */
@JsonIgnoreProperties(ignoreUnknown = true)
public class KeyVersion {

    @JsonProperty("id")
    private String id;

    @JsonProperty("entity_id")
    private String entityId;

    @JsonProperty("key_index")
    private int keyIndex;

    @JsonProperty("valid_from")
    private String validFrom;

    @JsonProperty("valid_until")
    private String validUntil;

    @JsonProperty("created_at")
    private String createdAt;

    public KeyVersion() {
    }

    public String getId() { return id; }
    public void setId(String id) { this.id = id; }

    public String getEntityId() { return entityId; }
    public void setEntityId(String entityId) { this.entityId = entityId; }

    public int getKeyIndex() { return keyIndex; }
    public void setKeyIndex(int keyIndex) { this.keyIndex = keyIndex; }

    public String getValidFrom() { return validFrom; }
    public void setValidFrom(String validFrom) { this.validFrom = validFrom; }

    public String getValidUntil() { return validUntil; }
    public void setValidUntil(String validUntil) { this.validUntil = validUntil; }

    public String getCreatedAt() { return createdAt; }
    public void setCreatedAt(String createdAt) { this.createdAt = createdAt; }
}
