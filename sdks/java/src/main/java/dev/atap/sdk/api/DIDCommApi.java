package dev.atap.sdk.api;

import dev.atap.sdk.ATAPClient;
import dev.atap.sdk.model.DIDCommInbox;
import dev.atap.sdk.model.DIDCommMessage;

import java.util.*;

/**
 * Send and receive DIDComm messages.
 */
public class DIDCommApi {

    private final ATAPClient client;

    public DIDCommApi(ATAPClient client) {
        this.client = client;
    }

    /**
     * Send a DIDComm message (JWE envelope). Public endpoint.
     *
     * @param jweBytes raw JWE bytes (application/didcomm-encrypted+json)
     * @return response with id and status
     */
    public Map<String, Object> send(byte[] jweBytes) {
        return client.getHttpClient().request("POST", "/v1/didcomm",
                null,
                Collections.singletonMap("Content-Type", "application/didcomm-encrypted+json"),
                null);
    }

    /**
     * Retrieve pending DIDComm messages. Requires atap:inbox scope.
     *
     * @param limit max messages to return (default 50, max 100)
     * @return DIDCommInbox with messages
     */
    @SuppressWarnings("unchecked")
    public DIDCommInbox inbox(int limit) {
        int effectiveLimit = Math.min(limit, 100);
        Map<String, String> params = Collections.singletonMap("limit", String.valueOf(effectiveLimit));
        Map<String, Object> data = client.authedRequest("GET", "/v1/didcomm/inbox", null, null, null, params);

        DIDCommInbox inbox = new DIDCommInbox();
        List<DIDCommMessage> messages = new ArrayList<>();

        List<Map<String, Object>> rawMessages = (List<Map<String, Object>>) data.getOrDefault("messages", Collections.emptyList());
        for (Map<String, Object> m : rawMessages) {
            DIDCommMessage msg = new DIDCommMessage();
            msg.setId((String) m.getOrDefault("id", ""));
            msg.setSenderDid((String) m.getOrDefault("sender_did", ""));
            msg.setMessageType((String) m.getOrDefault("message_type", ""));
            msg.setPayload((String) m.getOrDefault("payload", ""));
            msg.setCreatedAt((String) m.getOrDefault("created_at", ""));
            messages.add(msg);
        }

        inbox.setMessages(messages);
        inbox.setCount(data.containsKey("count") ? ((Number) data.get("count")).intValue() : messages.size());
        return inbox;
    }

    public DIDCommInbox inbox() {
        return inbox(50);
    }
}
