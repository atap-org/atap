package dev.atap.sdk.api;

import com.fasterxml.jackson.databind.ObjectMapper;
import dev.atap.sdk.ATAPClient;
import dev.atap.sdk.model.DIDDocument;
import dev.atap.sdk.model.DiscoveryDocument;
import dev.atap.sdk.model.VerificationMethod;

import java.util.*;

/**
 * Server discovery and DID document resolution.
 */
public class DiscoveryApi {

    private final ATAPClient client;

    public DiscoveryApi(ATAPClient client) {
        this.client = client;
    }

    /**
     * Fetch the server discovery document from /.well-known/atap.json.
     */
    public DiscoveryDocument discover() {
        Map<String, Object> data = client.getHttpClient().request("GET", "/.well-known/atap.json", null, null, null);
        ObjectMapper mapper = new ObjectMapper();
        return mapper.convertValue(data, DiscoveryDocument.class);
    }

    /**
     * Resolve an entity's DID Document.
     *
     * @param entityType entity type (agent, machine, human, org)
     * @param entityId   entity ID
     * @return DIDDocument
     */
    public DIDDocument resolveDid(String entityType, String entityId) {
        Map<String, Object> data = client.getHttpClient().request("GET",
                "/" + entityType + "/" + entityId + "/did.json", null, null, null);
        return mapToDIDDocument(data);
    }

    /**
     * Fetch the server's DID Document.
     */
    public DIDDocument serverDid() {
        Map<String, Object> data = client.getHttpClient().request("GET",
                "/server/platform/did.json", null, null, null);
        return mapToDIDDocument(data);
    }

    /**
     * Check server health.
     */
    public Map<String, Object> health() {
        return client.getHttpClient().request("GET", "/v1/health", null, null, null);
    }

    @SuppressWarnings("unchecked")
    static DIDDocument mapToDIDDocument(Map<String, Object> data) {
        DIDDocument doc = new DIDDocument();
        doc.setId((String) data.getOrDefault("id", ""));
        doc.setContext((List<String>) data.getOrDefault("@context", Collections.emptyList()));

        List<VerificationMethod> vms = new ArrayList<>();
        List<Map<String, Object>> rawVMs = (List<Map<String, Object>>) data.getOrDefault("verificationMethod", Collections.emptyList());
        for (Map<String, Object> vm : rawVMs) {
            VerificationMethod v = new VerificationMethod();
            v.setId((String) vm.getOrDefault("id", ""));
            v.setType((String) vm.getOrDefault("type", ""));
            v.setController((String) vm.getOrDefault("controller", ""));
            v.setPublicKeyMultibase((String) vm.getOrDefault("publicKeyMultibase", ""));
            vms.add(v);
        }
        doc.setVerificationMethod(vms);
        doc.setAuthentication((List<String>) data.getOrDefault("authentication", Collections.emptyList()));
        doc.setAssertionMethod((List<String>) data.getOrDefault("assertionMethod", Collections.emptyList()));
        doc.setKeyAgreement((List<String>) data.getOrDefault("keyAgreement", Collections.emptyList()));
        doc.setService((List<Map<String, Object>>) data.getOrDefault("service", Collections.emptyList()));
        doc.setAtapType((String) data.getOrDefault("atap:type", ""));
        doc.setAtapPrincipal((String) data.getOrDefault("atap:principal", ""));
        return doc;
    }
}
