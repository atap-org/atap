package dev.atap.sdk.crypto;

import org.bouncycastle.crypto.params.Ed25519PrivateKeyParameters;
import org.bouncycastle.crypto.params.Ed25519PublicKeyParameters;
import org.bouncycastle.crypto.signers.Ed25519Signer;

import java.security.SecureRandom;
import java.util.Base64;

/**
 * Ed25519 cryptographic operations using BouncyCastle.
 */
public final class Ed25519KeyPair {

    private final Ed25519PrivateKeyParameters privateKey;
    private final Ed25519PublicKeyParameters publicKey;

    private Ed25519KeyPair(Ed25519PrivateKeyParameters privateKey, Ed25519PublicKeyParameters publicKey) {
        this.privateKey = privateKey;
        this.publicKey = publicKey;
    }

    /**
     * Generate a new Ed25519 keypair.
     */
    public static Ed25519KeyPair generate() {
        SecureRandom random = new SecureRandom();
        Ed25519PrivateKeyParameters priv = new Ed25519PrivateKeyParameters(random);
        Ed25519PublicKeyParameters pub = priv.generatePublicKey();
        return new Ed25519KeyPair(priv, pub);
    }

    /**
     * Load a signing key from base64. Accepts 32-byte seed or 64-byte full key.
     */
    public static Ed25519KeyPair loadSigningKey(String base64Key) {
        byte[] raw = Base64.getDecoder().decode(base64Key);
        byte[] seed;
        if (raw.length == 64) {
            seed = new byte[32];
            System.arraycopy(raw, 0, seed, 0, 32);
        } else if (raw.length == 32) {
            seed = raw;
        } else {
            throw new IllegalArgumentException(
                    "Invalid private key length: " + raw.length + " bytes (expected 32 or 64)");
        }
        Ed25519PrivateKeyParameters priv = new Ed25519PrivateKeyParameters(seed, 0);
        Ed25519PublicKeyParameters pub = priv.generatePublicKey();
        return new Ed25519KeyPair(priv, pub);
    }

    /**
     * Get the public key bytes.
     */
    public byte[] getPublicKeyBytes() {
        return publicKey.getEncoded();
    }

    /**
     * Get base64-encoded public key.
     */
    public String getPublicKeyBase64() {
        return Base64.getEncoder().encodeToString(publicKey.getEncoded());
    }

    /**
     * Get base64url-encoded public key (no padding).
     */
    public String getPublicKeyBase64Url() {
        return Base64Url.encode(publicKey.getEncoded());
    }

    /**
     * Sign data with the private key.
     *
     * @param data the data to sign
     * @return the 64-byte Ed25519 signature
     */
    public byte[] sign(byte[] data) {
        Ed25519Signer signer = new Ed25519Signer();
        signer.init(true, privateKey);
        signer.update(data, 0, data.length);
        return signer.generateSignature();
    }

    /**
     * Verify a signature against data.
     *
     * @param data      the original data
     * @param signature the signature to verify
     * @return true if the signature is valid
     */
    public boolean verify(byte[] data, byte[] signature) {
        Ed25519Signer signer = new Ed25519Signer();
        signer.init(false, publicKey);
        signer.update(data, 0, data.length);
        return signer.verifySignature(signature);
    }

    public Ed25519PrivateKeyParameters getPrivateKey() {
        return privateKey;
    }

    public Ed25519PublicKeyParameters getPublicKey() {
        return publicKey;
    }
}
