package dev.atap.sdk.api;

import com.fasterxml.jackson.databind.ObjectMapper;
import dev.atap.sdk.ATAPClient;
import dev.atap.sdk.model.Revocation;
import dev.atap.sdk.model.RevocationList;

import java.util.*;

/**
 * Submit and query revocations.
 */
public class RevocationApi {

    private final ATAPClient client;

    public RevocationApi(ATAPClient client) {
        this.client = client;
    }

    /**
     * Submit a revocation. Requires atap:revoke scope.
     *
     * @param approvalId the approval ID to revoke
     * @param signature  JWS signature
     * @param validUntil optional RFC3339 expiry
     * @return the Revocation
     */
    public Revocation submit(String approvalId, String signature, String validUntil) {
        Map<String, Object> body = new LinkedHashMap<>();
        body.put("approval_id", approvalId);
        body.put("signature", signature);
        if (validUntil != null && !validUntil.isEmpty()) {
            body.put("valid_until", validUntil);
        }

        Map<String, Object> data = client.authedRequest("POST", "/v1/revocations", body, null, null, null);
        return mapToRevocation(data);
    }

    public Revocation submit(String approvalId, String signature) {
        return submit(approvalId, signature, null);
    }

    /**
     * Query active revocations for an entity (public endpoint).
     *
     * @param entityDid the approver DID to query
     * @return RevocationList
     */
    @SuppressWarnings("unchecked")
    public RevocationList list(String entityDid) {
        Map<String, String> params = Collections.singletonMap("entity", entityDid);
        Map<String, Object> data = client.getHttpClient().request("GET", "/v1/revocations", null, null, params);

        RevocationList rl = new RevocationList();
        rl.setEntity(data.containsKey("entity") ? (String) data.get("entity") : entityDid);
        rl.setCheckedAt(data.containsKey("checked_at") ? (String) data.get("checked_at") : "");

        List<Revocation> revocations = new ArrayList<>();
        List<Map<String, Object>> items = (List<Map<String, Object>>) data.getOrDefault("revocations", Collections.emptyList());
        for (Map<String, Object> item : items) {
            revocations.add(mapToRevocation(item));
        }
        rl.setRevocations(revocations);
        return rl;
    }

    static Revocation mapToRevocation(Map<String, Object> data) {
        ObjectMapper mapper = new ObjectMapper();
        return mapper.convertValue(data, Revocation.class);
    }
}
