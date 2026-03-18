package dev.atap.sdk.http;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.core.type.TypeReference;
import com.fasterxml.jackson.databind.ObjectMapper;
import dev.atap.sdk.crypto.DPoPProof;
import dev.atap.sdk.crypto.Ed25519KeyPair;
import dev.atap.sdk.exception.*;
import dev.atap.sdk.model.ProblemDetail;

import java.io.IOException;
import java.net.URI;
import java.net.URLEncoder;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.util.Collections;
import java.util.Map;
import java.util.StringJoiner;

/**
 * HTTP client wrapping java.net.http.HttpClient with DPoP authentication and RFC 7807 error parsing.
 */
public class ATAPHttpClient {

    private final String baseUrl;
    private final HttpClient httpClient;
    private final ObjectMapper objectMapper;
    private final Duration timeout;

    public ATAPHttpClient(String baseUrl, Duration timeout) {
        this.baseUrl = baseUrl.replaceAll("/+$", "");
        this.timeout = timeout;
        this.httpClient = HttpClient.newBuilder()
                .followRedirects(HttpClient.Redirect.NEVER)
                .connectTimeout(timeout)
                .build();
        this.objectMapper = new ObjectMapper();
    }

    public ATAPHttpClient(String baseUrl) {
        this(baseUrl, Duration.ofSeconds(30));
    }

    /**
     * Make an HTTP request and return parsed JSON response.
     */
    public Map<String, Object> request(String method, String path,
                                        Map<String, Object> jsonBody,
                                        Map<String, String> headers,
                                        Map<String, String> params) {
        String url = buildUrl(path, params);
        HttpRequest.Builder builder = HttpRequest.newBuilder()
                .uri(URI.create(url))
                .timeout(timeout);

        if (headers != null) {
            headers.forEach(builder::header);
        }

        if (jsonBody != null) {
            builder.header("Content-Type", "application/json");
            try {
                String body = objectMapper.writeValueAsString(jsonBody);
                builder.method(method, HttpRequest.BodyPublishers.ofString(body));
            } catch (JsonProcessingException e) {
                throw new ATAPException("Failed to serialize JSON body", 0, e);
            }
        } else {
            if ("GET".equalsIgnoreCase(method)) {
                builder.GET();
            } else if ("DELETE".equalsIgnoreCase(method)) {
                builder.DELETE();
            } else {
                builder.method(method, HttpRequest.BodyPublishers.noBody());
            }
        }

        return executeAndHandle(builder.build());
    }

    /**
     * Make a DPoP-authenticated HTTP request.
     */
    public Map<String, Object> authenticatedRequest(String method, String path,
                                                     Ed25519KeyPair keyPair,
                                                     String accessToken,
                                                     String platformDomain,
                                                     Map<String, Object> jsonBody,
                                                     byte[] rawBody,
                                                     String contentType,
                                                     Map<String, String> params) {
        String htuUrl = "https://" + platformDomain + path;
        String dpopProof = DPoPProof.create(keyPair, method, htuUrl, accessToken);

        String url = buildUrl(path, params);
        HttpRequest.Builder builder = HttpRequest.newBuilder()
                .uri(URI.create(url))
                .timeout(timeout)
                .header("Authorization", "DPoP " + accessToken)
                .header("DPoP", dpopProof);

        if (contentType != null) {
            builder.header("Content-Type", contentType);
        }

        if (rawBody != null) {
            if (contentType == null) {
                builder.header("Content-Type", "application/octet-stream");
            }
            builder.method(method, HttpRequest.BodyPublishers.ofByteArray(rawBody));
        } else if (jsonBody != null) {
            builder.header("Content-Type", "application/json");
            try {
                String body = objectMapper.writeValueAsString(jsonBody);
                builder.method(method, HttpRequest.BodyPublishers.ofString(body));
            } catch (JsonProcessingException e) {
                throw new ATAPException("Failed to serialize JSON body", 0, e);
            }
        } else {
            if ("GET".equalsIgnoreCase(method)) {
                builder.GET();
            } else if ("DELETE".equalsIgnoreCase(method)) {
                builder.DELETE();
            } else {
                builder.method(method, HttpRequest.BodyPublishers.noBody());
            }
        }

        return executeAndHandle(builder.build());
    }

    /**
     * POST form-encoded data (for OAuth token endpoint).
     */
    public Map<String, Object> postForm(String path, Map<String, String> formData, String dpopProof) {
        String url = buildUrl(path, null);
        StringJoiner joiner = new StringJoiner("&");
        formData.forEach((k, v) -> joiner.add(
                URLEncoder.encode(k, StandardCharsets.UTF_8) + "=" +
                URLEncoder.encode(v, StandardCharsets.UTF_8)));

        HttpRequest.Builder builder = HttpRequest.newBuilder()
                .uri(URI.create(url))
                .timeout(timeout)
                .header("Content-Type", "application/x-www-form-urlencoded")
                .POST(HttpRequest.BodyPublishers.ofString(joiner.toString()));

        if (dpopProof != null) {
            builder.header("DPoP", dpopProof);
        }

        return executeAndHandle(builder.build());
    }

    /**
     * GET request expecting a 302 redirect, returns the Location URL.
     */
    public String getRedirect(String path, Map<String, String> params, String dpopProof) {
        String url = buildUrl(path, params);
        HttpRequest.Builder builder = HttpRequest.newBuilder()
                .uri(URI.create(url))
                .timeout(timeout)
                .GET();

        if (dpopProof != null) {
            builder.header("DPoP", dpopProof);
        }

        try {
            HttpResponse<String> response = httpClient.send(builder.build(),
                    HttpResponse.BodyHandlers.ofString());

            if (response.statusCode() != 302) {
                handleResponseErrors(response);
                throw new ATAPException("Expected 302 redirect, got " + response.statusCode(),
                        response.statusCode());
            }

            String location = response.headers().firstValue("location").orElse("");
            if (location.isEmpty()) {
                throw new ATAPException("302 redirect with no Location header");
            }
            return location;
        } catch (ATAPException e) {
            throw e;
        } catch (IOException | InterruptedException e) {
            if (e instanceof InterruptedException) {
                Thread.currentThread().interrupt();
            }
            throw new ATAPException("HTTP request failed: " + e.getMessage(), 0, e);
        }
    }

    private Map<String, Object> executeAndHandle(HttpRequest request) {
        try {
            HttpResponse<String> response = httpClient.send(request,
                    HttpResponse.BodyHandlers.ofString());
            return handleResponse(response);
        } catch (ATAPException e) {
            throw e;
        } catch (IOException | InterruptedException e) {
            if (e instanceof InterruptedException) {
                Thread.currentThread().interrupt();
            }
            throw new ATAPException("HTTP request failed: " + e.getMessage(), 0, e);
        }
    }

    private Map<String, Object> handleResponse(HttpResponse<String> response) {
        int status = response.statusCode();

        if (status == 204) {
            return Collections.emptyMap();
        }

        Map<String, Object> data = null;
        String body = response.body();

        if (body != null && !body.isEmpty()) {
            try {
                data = objectMapper.readValue(body, new TypeReference<Map<String, Object>>() {});
            } catch (JsonProcessingException e) {
                if (status >= 200 && status < 300) {
                    return Collections.emptyMap();
                }
                throw new ATAPException("HTTP " + status + ": " + body, status);
            }
        }

        if (status >= 200 && status < 300) {
            return data != null ? data : Collections.emptyMap();
        }

        if (data == null) {
            throw new ATAPException("HTTP " + status + ": " + body, status);
        }

        handleErrorData(status, data);
        return data; // unreachable
    }

    private void handleResponseErrors(HttpResponse<String> response) {
        int status = response.statusCode();
        if (status >= 200 && status < 300) {
            return;
        }

        Map<String, Object> data = null;
        String body = response.body();
        if (body != null && !body.isEmpty()) {
            try {
                data = objectMapper.readValue(body, new TypeReference<Map<String, Object>>() {});
            } catch (JsonProcessingException e) {
                throw new ATAPException("HTTP " + status + ": " + body, status);
            }
        }
        if (data != null) {
            handleErrorData(status, data);
        }
        throw new ATAPException("HTTP " + status, status);
    }

    private void handleErrorData(int status, Map<String, Object> data) {
        ProblemDetail problem = null;
        if (data.containsKey("type") && data.containsKey("status")) {
            problem = new ProblemDetail(
                    stringVal(data, "type"),
                    stringVal(data, "title"),
                    status,
                    stringVal(data, "detail"),
                    stringVal(data, "instance")
            );
        }

        if (status == 401 || status == 403) {
            String msg = problem != null ? problem.getDetail() : stringVal(data, "detail");
            if (msg == null || msg.isEmpty()) msg = "Authentication failed";
            throw new ATAPAuthException(msg, status, problem);
        } else if (status == 404) {
            String msg = problem != null ? problem.getDetail() : "Not found";
            throw new ATAPNotFoundException(msg, problem);
        } else if (status == 409) {
            String msg = problem != null ? problem.getDetail() : "Conflict";
            throw new ATAPConflictException(msg, problem);
        } else if (status == 429) {
            String msg = problem != null ? problem.getDetail() : "Rate limit exceeded";
            throw new ATAPRateLimitException(msg, problem);
        } else if (problem != null) {
            throw new ATAPProblemException(problem);
        } else {
            String msg = stringVal(data, "detail");
            if (msg == null) msg = stringVal(data, "message");
            if (msg == null) msg = data.toString();
            throw new ATAPException("HTTP " + status + ": " + msg, status);
        }
    }

    private String stringVal(Map<String, Object> data, String key) {
        Object v = data.get(key);
        return v != null ? v.toString() : null;
    }

    private String buildUrl(String path, Map<String, String> params) {
        StringBuilder url = new StringBuilder(baseUrl);
        if (!path.startsWith("/")) {
            url.append("/");
        }
        url.append(path);

        if (params != null && !params.isEmpty()) {
            url.append("?");
            StringJoiner joiner = new StringJoiner("&");
            params.forEach((k, v) -> joiner.add(
                    URLEncoder.encode(k, StandardCharsets.UTF_8) + "=" +
                    URLEncoder.encode(v, StandardCharsets.UTF_8)));
            url.append(joiner.toString());
        }
        return url.toString();
    }

    public ObjectMapper getObjectMapper() {
        return objectMapper;
    }

    public String getBaseUrl() {
        return baseUrl;
    }
}
