package dev.atap.sdk.model;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import com.fasterxml.jackson.annotation.JsonProperty;

import java.util.List;

/**
 * A list of active revocations for an entity.
 */
@JsonIgnoreProperties(ignoreUnknown = true)
public class RevocationList {

    @JsonProperty("entity")
    private String entity;

    @JsonProperty("revocations")
    private List<Revocation> revocations;

    @JsonProperty("checked_at")
    private String checkedAt;

    public RevocationList() {
    }

    public String getEntity() { return entity; }
    public void setEntity(String entity) { this.entity = entity; }

    public List<Revocation> getRevocations() { return revocations; }
    public void setRevocations(List<Revocation> revocations) { this.revocations = revocations; }

    public String getCheckedAt() { return checkedAt; }
    public void setCheckedAt(String checkedAt) { this.checkedAt = checkedAt; }
}
