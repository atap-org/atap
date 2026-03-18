package dev.atap.sdk.model;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import com.fasterxml.jackson.annotation.JsonProperty;

import java.util.Map;

/**
 * A multi-signature approval document.
 */
@JsonIgnoreProperties(ignoreUnknown = true)
public class Approval {

    @JsonProperty("id")
    private String id;

    @JsonProperty("state")
    private String state;

    @JsonProperty("created_at")
    private String createdAt;

    @JsonProperty("valid_until")
    private String validUntil;

    @JsonProperty("from")
    private String fromDid;

    @JsonProperty("to")
    private String toDid;

    @JsonProperty("via")
    private String via;

    @JsonProperty("parent")
    private String parent;

    @JsonProperty("subject")
    private ApprovalSubject subject;

    @JsonProperty("template_url")
    private String templateUrl;

    @JsonProperty("signatures")
    private Map<String, String> signatures;

    @JsonProperty("responded_at")
    private String respondedAt;

    @JsonProperty("fan_out")
    private Integer fanOut;

    public Approval() {
    }

    public String getId() { return id; }
    public void setId(String id) { this.id = id; }

    public String getState() { return state; }
    public void setState(String state) { this.state = state; }

    public String getCreatedAt() { return createdAt; }
    public void setCreatedAt(String createdAt) { this.createdAt = createdAt; }

    public String getValidUntil() { return validUntil; }
    public void setValidUntil(String validUntil) { this.validUntil = validUntil; }

    public String getFromDid() { return fromDid; }
    public void setFromDid(String fromDid) { this.fromDid = fromDid; }

    public String getToDid() { return toDid; }
    public void setToDid(String toDid) { this.toDid = toDid; }

    public String getVia() { return via; }
    public void setVia(String via) { this.via = via; }

    public String getParent() { return parent; }
    public void setParent(String parent) { this.parent = parent; }

    public ApprovalSubject getSubject() { return subject; }
    public void setSubject(ApprovalSubject subject) { this.subject = subject; }

    public String getTemplateUrl() { return templateUrl; }
    public void setTemplateUrl(String templateUrl) { this.templateUrl = templateUrl; }

    public Map<String, String> getSignatures() { return signatures; }
    public void setSignatures(Map<String, String> signatures) { this.signatures = signatures; }

    public String getRespondedAt() { return respondedAt; }
    public void setRespondedAt(String respondedAt) { this.respondedAt = respondedAt; }

    public Integer getFanOut() { return fanOut; }
    public void setFanOut(Integer fanOut) { this.fanOut = fanOut; }
}
