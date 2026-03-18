package dev.atap.sdk.model;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import com.fasterxml.jackson.annotation.JsonProperty;

import java.util.Map;

/**
 * The purpose and payload of an approval.
 */
@JsonIgnoreProperties(ignoreUnknown = true)
public class ApprovalSubject {

    @JsonProperty("type")
    private String type;

    @JsonProperty("label")
    private String label;

    @JsonProperty("reversible")
    private boolean reversible;

    @JsonProperty("payload")
    private Map<String, Object> payload;

    public ApprovalSubject() {
    }

    public ApprovalSubject(String type, String label, boolean reversible, Map<String, Object> payload) {
        this.type = type;
        this.label = label;
        this.reversible = reversible;
        this.payload = payload;
    }

    public String getType() { return type; }
    public void setType(String type) { this.type = type; }

    public String getLabel() { return label; }
    public void setLabel(String label) { this.label = label; }

    public boolean isReversible() { return reversible; }
    public void setReversible(boolean reversible) { this.reversible = reversible; }

    public Map<String, Object> getPayload() { return payload; }
    public void setPayload(Map<String, Object> payload) { this.payload = payload; }
}
