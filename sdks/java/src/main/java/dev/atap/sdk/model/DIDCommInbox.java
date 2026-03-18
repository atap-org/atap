package dev.atap.sdk.model;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import com.fasterxml.jackson.annotation.JsonProperty;

import java.util.List;

/**
 * DIDComm inbox response.
 */
@JsonIgnoreProperties(ignoreUnknown = true)
public class DIDCommInbox {

    @JsonProperty("messages")
    private List<DIDCommMessage> messages;

    @JsonProperty("count")
    private int count;

    public DIDCommInbox() {
    }

    public List<DIDCommMessage> getMessages() { return messages; }
    public void setMessages(List<DIDCommMessage> messages) { this.messages = messages; }

    public int getCount() { return count; }
    public void setCount(int count) { this.count = count; }
}
