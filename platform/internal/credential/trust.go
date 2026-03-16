package credential

// Trust levels per ATAP spec §6.
const (
	TrustLevelL0 = 0 // Anonymous — no credentials
	TrustLevelL1 = 1 // Email or phone verified
	TrustLevelL2 = 2 // Personhood verified (World ID or equivalent ZK proof)
	TrustLevelL3 = 3 // Full identity (eID / government-issued identity)
)

// DeriveTrustLevel returns the highest trust level indicated by the given credential types.
// The scan is highest-wins: finding ATAPIdentity immediately returns L3, etc.
func DeriveTrustLevel(credTypes []string) int {
	highest := TrustLevelL0
	for _, t := range credTypes {
		switch t {
		case "ATAPIdentity":
			return TrustLevelL3 // can't do better
		case "ATAPPersonhood":
			if highest < TrustLevelL2 {
				highest = TrustLevelL2
			}
		case "ATAPEmailVerification", "ATAPPhoneVerification":
			if highest < TrustLevelL1 {
				highest = TrustLevelL1
			}
		}
	}
	return highest
}

// EffectiveTrust returns the effective trust level, which is the minimum of the
// entity's own trust level and the server's configured trust ceiling (CRD-04).
func EffectiveTrust(entityTrust, serverTrust int) int {
	if entityTrust < serverTrust {
		return entityTrust
	}
	return serverTrust
}
