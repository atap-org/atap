package dev.atap.sdk.model;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import com.fasterxml.jackson.annotation.JsonProperty;

import java.util.List;
import java.util.Map;

/**
 * A W3C DID Document.
 */
@JsonIgnoreProperties(ignoreUnknown = true)
public class DIDDocument {

    @JsonProperty("id")
    private String id;

    @JsonProperty("@context")
    private List<String> context;

    @JsonProperty("verificationMethod")
    private List<VerificationMethod> verificationMethod;

    @JsonProperty("authentication")
    private List<String> authentication;

    @JsonProperty("assertionMethod")
    private List<String> assertionMethod;

    @JsonProperty("keyAgreement")
    private List<String> keyAgreement;

    @JsonProperty("service")
    private List<Map<String, Object>> service;

    @JsonProperty("atap:type")
    private String atapType;

    @JsonProperty("atap:principal")
    private String atapPrincipal;

    public DIDDocument() {
    }

    public String getId() { return id; }
    public void setId(String id) { this.id = id; }

    public List<String> getContext() { return context; }
    public void setContext(List<String> context) { this.context = context; }

    public List<VerificationMethod> getVerificationMethod() { return verificationMethod; }
    public void setVerificationMethod(List<VerificationMethod> verificationMethod) { this.verificationMethod = verificationMethod; }

    public List<String> getAuthentication() { return authentication; }
    public void setAuthentication(List<String> authentication) { this.authentication = authentication; }

    public List<String> getAssertionMethod() { return assertionMethod; }
    public void setAssertionMethod(List<String> assertionMethod) { this.assertionMethod = assertionMethod; }

    public List<String> getKeyAgreement() { return keyAgreement; }
    public void setKeyAgreement(List<String> keyAgreement) { this.keyAgreement = keyAgreement; }

    public List<Map<String, Object>> getService() { return service; }
    public void setService(List<Map<String, Object>> service) { this.service = service; }

    public String getAtapType() { return atapType; }
    public void setAtapType(String atapType) { this.atapType = atapType; }

    public String getAtapPrincipal() { return atapPrincipal; }
    public void setAtapPrincipal(String atapPrincipal) { this.atapPrincipal = atapPrincipal; }
}
