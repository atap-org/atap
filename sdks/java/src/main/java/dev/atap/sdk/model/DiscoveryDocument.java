package dev.atap.sdk.model;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import com.fasterxml.jackson.annotation.JsonProperty;

import java.util.List;
import java.util.Map;

/**
 * Server discovery document from /.well-known/atap.json.
 */
@JsonIgnoreProperties(ignoreUnknown = true)
public class DiscoveryDocument {

    @JsonProperty("domain")
    private String domain;

    @JsonProperty("api_base")
    private String apiBase;

    @JsonProperty("didcomm_endpoint")
    private String didcommEndpoint;

    @JsonProperty("claim_types")
    private List<String> claimTypes;

    @JsonProperty("max_approval_ttl")
    private String maxApprovalTtl;

    @JsonProperty("trust_level")
    private int trustLevel;

    @JsonProperty("oauth")
    private Map<String, Object> oauth;

    public DiscoveryDocument() {
    }

    public String getDomain() { return domain; }
    public void setDomain(String domain) { this.domain = domain; }

    public String getApiBase() { return apiBase; }
    public void setApiBase(String apiBase) { this.apiBase = apiBase; }

    public String getDidcommEndpoint() { return didcommEndpoint; }
    public void setDidcommEndpoint(String didcommEndpoint) { this.didcommEndpoint = didcommEndpoint; }

    public List<String> getClaimTypes() { return claimTypes; }
    public void setClaimTypes(List<String> claimTypes) { this.claimTypes = claimTypes; }

    public String getMaxApprovalTtl() { return maxApprovalTtl; }
    public void setMaxApprovalTtl(String maxApprovalTtl) { this.maxApprovalTtl = maxApprovalTtl; }

    public int getTrustLevel() { return trustLevel; }
    public void setTrustLevel(int trustLevel) { this.trustLevel = trustLevel; }

    public Map<String, Object> getOauth() { return oauth; }
    public void setOauth(Map<String, Object> oauth) { this.oauth = oauth; }
}
