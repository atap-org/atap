package dev.atap.sdk.model;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import com.fasterxml.jackson.annotation.JsonProperty;

/**
 * A verification method in a DID Document.
 */
@JsonIgnoreProperties(ignoreUnknown = true)
public class VerificationMethod {

    @JsonProperty("id")
    private String id;

    @JsonProperty("type")
    private String type;

    @JsonProperty("controller")
    private String controller;

    @JsonProperty("publicKeyMultibase")
    private String publicKeyMultibase;

    public VerificationMethod() {
    }

    public String getId() { return id; }
    public void setId(String id) { this.id = id; }

    public String getType() { return type; }
    public void setType(String type) { this.type = type; }

    public String getController() { return controller; }
    public void setController(String controller) { this.controller = controller; }

    public String getPublicKeyMultibase() { return publicKeyMultibase; }
    public void setPublicKeyMultibase(String publicKeyMultibase) { this.publicKeyMultibase = publicKeyMultibase; }
}
