package dev.atap.sdk.model;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import com.fasterxml.jackson.annotation.JsonProperty;

/**
 * A revocation entry for a previously-granted approval.
 */
@JsonIgnoreProperties(ignoreUnknown = true)
public class Revocation {

    @JsonProperty("id")
    private String id;

    @JsonProperty("approval_id")
    private String approvalId;

    @JsonProperty("approver_did")
    private String approverDid;

    @JsonProperty("revoked_at")
    private String revokedAt;

    @JsonProperty("expires_at")
    private String expiresAt;

    public Revocation() {
    }

    public String getId() { return id; }
    public void setId(String id) { this.id = id; }

    public String getApprovalId() { return approvalId; }
    public void setApprovalId(String approvalId) { this.approvalId = approvalId; }

    public String getApproverDid() { return approverDid; }
    public void setApproverDid(String approverDid) { this.approverDid = approverDid; }

    public String getRevokedAt() { return revokedAt; }
    public void setRevokedAt(String revokedAt) { this.revokedAt = revokedAt; }

    public String getExpiresAt() { return expiresAt; }
    public void setExpiresAt(String expiresAt) { this.expiresAt = expiresAt; }
}
