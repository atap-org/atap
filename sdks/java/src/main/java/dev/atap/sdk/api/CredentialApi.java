package dev.atap.sdk.api;

import com.fasterxml.jackson.databind.ObjectMapper;
import dev.atap.sdk.ATAPClient;
import dev.atap.sdk.model.Credential;

import java.util.*;

/**
 * Email/phone/personhood verification and credential management.
 */
public class CredentialApi {

    private final ATAPClient client;

    public CredentialApi(ATAPClient client) {
        this.client = client;
    }

    /**
     * Initiate email verification (OTP). Requires atap:manage scope.
     *
     * @param email email address to verify
     * @return status message
     */
    public String startEmail(String email) {
        Map<String, Object> body = Collections.singletonMap("email", email);
        Map<String, Object> data = client.authedRequest("POST", "/v1/credentials/email/start", body, null, null, null);
        return data.containsKey("message") ? (String) data.get("message") : "OTP sent";
    }

    /**
     * Verify email with OTP. Requires atap:manage scope.
     */
    public Credential verifyEmail(String email, String otp) {
        Map<String, Object> body = new LinkedHashMap<>();
        body.put("email", email);
        body.put("otp", otp);

        Map<String, Object> data = client.authedRequest("POST", "/v1/credentials/email/verify", body, null, null, null);
        return mapToCredential(data);
    }

    /**
     * Initiate phone verification (OTP). Requires atap:manage scope.
     *
     * @param phone phone number (E.164 format)
     * @return status message
     */
    public String startPhone(String phone) {
        Map<String, Object> body = Collections.singletonMap("phone", phone);
        Map<String, Object> data = client.authedRequest("POST", "/v1/credentials/phone/start", body, null, null, null);
        return data.containsKey("message") ? (String) data.get("message") : "OTP sent";
    }

    /**
     * Verify phone with OTP. Requires atap:manage scope.
     */
    public Credential verifyPhone(String phone, String otp) {
        Map<String, Object> body = new LinkedHashMap<>();
        body.put("phone", phone);
        body.put("otp", otp);

        Map<String, Object> data = client.authedRequest("POST", "/v1/credentials/phone/verify", body, null, null, null);
        return mapToCredential(data);
    }

    /**
     * Submit personhood attestation. Requires atap:manage scope.
     *
     * @param providerToken optional provider token
     * @return Credential with VC JWT
     */
    public Credential submitPersonhood(String providerToken) {
        Map<String, Object> body = new LinkedHashMap<>();
        if (providerToken != null && !providerToken.isEmpty()) {
            body.put("provider_token", providerToken);
        }

        Map<String, Object> data = client.authedRequest("POST", "/v1/credentials/personhood", body, null, null, null);
        return mapToCredential(data);
    }

    public Credential submitPersonhood() {
        return submitPersonhood(null);
    }

    /**
     * List credentials for the authenticated entity. Requires atap:manage scope.
     */
    @SuppressWarnings("unchecked")
    public List<Credential> list() {
        Map<String, Object> data = client.authedRequest("GET", "/v1/credentials", null, null, null, null);
        List<Map<String, Object>> items;
        if (data.containsKey("credentials")) {
            items = (List<Map<String, Object>>) data.get("credentials");
        } else {
            items = Collections.emptyList();
        }
        List<Credential> result = new ArrayList<>();
        for (Map<String, Object> item : items) {
            result.add(mapToCredential(item));
        }
        return result;
    }

    /**
     * Get W3C Bitstring Status List VC (public endpoint).
     *
     * @param listId status list ID (default "1")
     * @return raw status list data
     */
    public Map<String, Object> statusList(String listId) {
        return client.getHttpClient().request("GET", "/v1/credentials/status/" + listId, null, null, null);
    }

    public Map<String, Object> statusList() {
        return statusList("1");
    }

    static Credential mapToCredential(Map<String, Object> data) {
        ObjectMapper mapper = new ObjectMapper();
        return mapper.convertValue(data, Credential.class);
    }
}
