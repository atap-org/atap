package dev.atap.sdk.api;

import com.fasterxml.jackson.databind.ObjectMapper;
import dev.atap.sdk.ATAPClient;
import dev.atap.sdk.model.Approval;
import dev.atap.sdk.model.ApprovalSubject;

import java.util.*;

/**
 * Create, respond to, list, and revoke approvals.
 */
public class ApprovalApi {

    private final ATAPClient client;

    public ApprovalApi(ATAPClient client) {
        this.client = client;
    }

    /**
     * Create an approval request. Requires atap:send scope.
     *
     * @param fromDid requester DID
     * @param toDid   approver DID (or org DID for fan-out)
     * @param subject the approval subject
     * @param via     optional mediating system DID
     * @return created Approval
     */
    public Approval create(String fromDid, String toDid, ApprovalSubject subject, String via) {
        Map<String, Object> body = new LinkedHashMap<>();
        body.put("from", fromDid);
        body.put("to", toDid);

        Map<String, Object> subjectMap = new LinkedHashMap<>();
        subjectMap.put("type", subject.getType());
        subjectMap.put("label", subject.getLabel());
        subjectMap.put("payload", subject.getPayload() != null ? subject.getPayload() : Collections.emptyMap());
        body.put("subject", subjectMap);

        if (via != null && !via.isEmpty()) {
            body.put("via", via);
        }

        Map<String, Object> data = client.authedRequest("POST", "/v1/approvals", body, null, null, null);
        return mapToApproval(data);
    }

    public Approval create(String fromDid, String toDid, ApprovalSubject subject) {
        return create(fromDid, toDid, subject, null);
    }

    /**
     * Respond to an approval (approve). Requires atap:send scope.
     */
    public Approval respond(String approvalId, String signature) {
        Map<String, Object> body = new LinkedHashMap<>();
        body.put("signature", signature);

        Map<String, Object> data = client.authedRequest("POST",
                "/v1/approvals/" + approvalId + "/respond", body, null, null, null);
        return mapToApproval(data);
    }

    /**
     * List approvals addressed to the authenticated entity. Requires atap:inbox scope.
     */
    @SuppressWarnings("unchecked")
    public List<Approval> list() {
        Map<String, Object> data = client.authedRequest("GET", "/v1/approvals", null, null, null, null);
        List<Map<String, Object>> items;
        if (data.containsKey("approvals")) {
            items = (List<Map<String, Object>>) data.get("approvals");
        } else if (data.containsKey("items")) {
            items = (List<Map<String, Object>>) data.get("items");
        } else {
            items = Collections.emptyList();
        }
        List<Approval> result = new ArrayList<>();
        for (Map<String, Object> item : items) {
            result.add(mapToApproval(item));
        }
        return result;
    }

    /**
     * Revoke an approval. Requires atap:revoke scope.
     */
    public Approval revoke(String approvalId) {
        Map<String, Object> data = client.authedRequest("DELETE",
                "/v1/approvals/" + approvalId, null, null, null, null);
        return mapToApproval(data);
    }

    @SuppressWarnings("unchecked")
    static Approval mapToApproval(Map<String, Object> data) {
        ObjectMapper mapper = new ObjectMapper();
        return mapper.convertValue(data, Approval.class);
    }
}
