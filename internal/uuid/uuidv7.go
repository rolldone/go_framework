package uuid

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// New generates a UUIDv7-compatible UUID string (time-ordered).
func New() (string, error) {
	ts := uint64(time.Now().UnixNano()/1e6) & 0xFFFFFFFFFFFF
	r := make([]byte, 10)
	if _, err := rand.Read(r); err != nil {
		return "", err
	}

	b := make([]byte, 16)
	b[0] = byte(ts >> 40)
	b[1] = byte(ts >> 32)
	b[2] = byte(ts >> 24)
	b[3] = byte(ts >> 16)
	b[4] = byte(ts >> 8)
	b[5] = byte(ts)

	// version 7 in high nibble of b[6]
	b[6] = (r[0] & 0x0F) | 0x70
	b[7] = r[1]

	// variant in b[8]
	b[8] = (r[2] & 0x3F) | 0x80
	copy(b[9:], r[3:10])

	hexs := make([]byte, 32)
	hex.Encode(hexs, b)
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		string(hexs[0:8]), string(hexs[8:12]), string(hexs[12:16]), string(hexs[16:20]), string(hexs[20:32])), nil
}
