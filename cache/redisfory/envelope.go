package redisfory

import (
	"bytes"
	"encoding/binary"
)

const (
	envelopeHeaderSize = 14
	envelopeVersion    = 1
)

var envelopeMagic = [4]byte{'B', 'T', 'F', 'V'}

func wrap(profile Profile, generation uint32, payload []byte) []byte {
	encoded := make([]byte, envelopeHeaderSize+len(payload))
	copy(encoded[:4], envelopeMagic[:])
	encoded[4] = envelopeVersion
	encoded[5] = profileFormat(profile)
	binary.BigEndian.PutUint32(encoded[6:10], generation)
	binary.BigEndian.PutUint32(encoded[10:14], uint32(len(payload)))
	copy(encoded[envelopeHeaderSize:], payload)
	return encoded
}

func unwrap(profile Profile, generation uint32, encoded []byte, maxPayload int) ([]byte, error) {
	if len(encoded) > envelopeHeaderSize+maxPayload {
		return nil, newCacheError("decode", profile, ReasonPayloadTooLarge, nil)
	}
	if len(encoded) < envelopeHeaderSize || !bytes.Equal(encoded[:4], envelopeMagic[:]) {
		return nil, newCacheError("decode", profile, ReasonInvalidMagic, nil)
	}
	if encoded[4] != envelopeVersion {
		return nil, newCacheError("decode", profile, ReasonUnsupportedVersion, nil)
	}
	if encoded[5] != profileFormat(profile) {
		return nil, newCacheError("decode", profile, ReasonFormatMismatch, nil)
	}
	if binary.BigEndian.Uint32(encoded[6:10]) != generation {
		return nil, newCacheError("decode", profile, ReasonSchemaMismatch, nil)
	}
	declared := binary.BigEndian.Uint32(encoded[10:14])
	if uint64(declared) > uint64(maxPayload) {
		return nil, newCacheError("decode", profile, ReasonPayloadTooLarge, nil)
	}
	if uint64(len(encoded)) != uint64(envelopeHeaderSize)+uint64(declared) {
		return nil, newCacheError("decode", profile, ReasonLengthMismatch, nil)
	}
	return encoded[envelopeHeaderSize:], nil
}

func profileFormat(profile Profile) byte {
	if profile == ProfileNativeCompatible {
		return 2
	}
	return 1
}
