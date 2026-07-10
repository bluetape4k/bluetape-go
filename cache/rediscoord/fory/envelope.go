package rediscoordfory

import "encoding/binary"

var magic = [4]byte{'B', 'T', 'F', 'Y'}

func wrap(profile Profile, payload []byte) []byte {
	out := make([]byte, 10+len(payload))
	copy(out, magic[:])
	out[4] = 1
	if profile == ProfileNativeCompatible {
		out[5] = 2
	} else {
		out[5] = 1
	}
	binary.BigEndian.PutUint32(out[6:10], uint32(len(payload)))
	copy(out[10:], payload)
	return out
}
func unwrap(profile Profile, data []byte, max int) ([]byte, error) {
	if len(data) > 10+max {
		return nil, &CodecError{operation: "unmarshal", profile: profile, reason: ReasonPayloadTooLarge}
	}
	if len(data) < 10 || string(data[:4]) != string(magic[:]) {
		return nil, &CodecError{operation: "unmarshal", profile: profile, reason: ReasonInvalidMagic}
	}
	if data[4] != 1 {
		return nil, &CodecError{operation: "unmarshal", profile: profile, reason: ReasonUnsupportedVersion}
	}
	expected := byte(1)
	if profile == ProfileNativeCompatible {
		expected = 2
	}
	if data[5] != expected {
		return nil, &CodecError{operation: "unmarshal", profile: profile, reason: ReasonProfileMismatch}
	}
	n := binary.BigEndian.Uint32(data[6:10])
	if n > uint32(max) {
		return nil, &CodecError{operation: "unmarshal", profile: profile, reason: ReasonPayloadTooLarge}
	}
	if int(n) != len(data)-10 {
		return nil, &CodecError{operation: "unmarshal", profile: profile, reason: ReasonLengthMismatch}
	}
	return data[10:], nil
}
