package dev.atap.sdk.model;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import com.fasterxml.jackson.annotation.JsonProperty;

/**
 * A W3C Verifiable Credential.
 */
@JsonIgnoreProperties(ignoreUnknown = true)
public class Credential {

    @JsonProperty("id")
    private String id;

    @JsonProperty("type")
    private String type;

    @JsonProperty("credential")
    private String credential;

    @JsonProperty("issued_at")
    private String issuedAt;

    @JsonProperty("revoked_at")
    private String revokedAt;

    public Credential() {
    }

    public String getId() { return id; }
    public void setId(String id) { this.id = id; }

    public String getType() { return type; }
    public void setType(String type) { this.type = type; }

    public String getCredential() { return credential; }
    public void setCredential(String credential) { this.credential = credential; }

    public String getIssuedAt() { return issuedAt; }
    public void setIssuedAt(String issuedAt) { this.issuedAt = issuedAt; }

    public String getRevokedAt() { return revokedAt; }
    public void setRevokedAt(String revokedAt) { this.revokedAt = revokedAt; }
}
