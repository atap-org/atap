package dev.atap.sdk.model;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import com.fasterxml.jackson.annotation.JsonProperty;

/**
 * A DIDComm message from the inbox.
 */
@JsonIgnoreProperties(ignoreUnknown = true)
public class DIDCommMessage {

    @JsonProperty("id")
    private String id;

    @JsonProperty("sender_did")
    private String senderDid;

    @JsonProperty("message_type")
    private String messageType;

    @JsonProperty("payload")
    private String payload;

    @JsonProperty("created_at")
    private String createdAt;

    public DIDCommMessage() {
    }

    public String getId() { return id; }
    public void setId(String id) { this.id = id; }

    public String getSenderDid() { return senderDid; }
    public void setSenderDid(String senderDid) { this.senderDid = senderDid; }

    public String getMessageType() { return messageType; }
    public void setMessageType(String messageType) { this.messageType = messageType; }

    public String getPayload() { return payload; }
    public void setPayload(String payload) { this.payload = payload; }

    public String getCreatedAt() { return createdAt; }
    public void setCreatedAt(String createdAt) { this.createdAt = createdAt; }
}
