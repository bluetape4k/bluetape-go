package jwt

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"time"
)

type cacheProfile struct {
	cacheable bool
	key       string
}

func buildCacheProfile(providerAlgorithm Algorithm, cfg cacheConfig, parse parseConfig, token string) cacheProfile {
	if parse.customClock {
		return cacheProfile{cacheable: false}
	}
	tokenDigest := sha256.Sum256([]byte(token))
	profileDigest := sha256.Sum256(cacheProfileBytes(parse))
	var buf []byte
	buf = append(buf, cfg.keyPrefix...)
	buf = append(buf, "|scope:"...)
	buf = append(buf, cfg.trustScope...)
	buf = append(buf, "|alg:"...)
	buf = append(buf, string(providerAlgorithm)...)
	buf = append(buf, "|token:"...)
	buf = hex.AppendEncode(buf, tokenDigest[:])
	buf = append(buf, "|profile:"...)
	buf = hex.AppendEncode(buf, profileDigest[:])
	return cacheProfile{cacheable: true, key: string(buf)}
}

func cacheProfileBytes(parse parseConfig) []byte {
	var buf []byte
	buf = appendField(buf, "leeway", strconv.FormatInt(int64(parse.leeway), 10))
	buf = appendField(buf, "issuer", parse.expectedIssuer)
	buf = appendField(buf, "subject", parse.expectedSubject)
	buf = appendField(buf, "exp", strconv.FormatBool(parse.expirationRequired))
	for _, audience := range parse.expectedAudience {
		buf = appendField(buf, "aud", audience)
	}
	return buf
}

func appendField(buf []byte, name string, value string) []byte {
	buf = append(buf, name...)
	buf = append(buf, '=')
	buf = strconv.AppendInt(buf, int64(len(value)), 10)
	buf = append(buf, ':')
	buf = append(buf, value...)
	buf = append(buf, ';')
	return buf
}

func cacheTTL(maxTTL time.Duration, now time.Time, reader *Reader, key *KeyChain) time.Duration {
	ttl := maxTTL
	if expiresAt, ok := reader.ExpiresAt(); ok {
		ttl = minPositiveDuration(ttl, expiresAt.Sub(now))
	}
	if keyExpiresAt := key.ExpiresAt(); !keyExpiresAt.IsZero() {
		ttl = minPositiveDuration(ttl, keyExpiresAt.Sub(now))
	}
	return ttl
}
