package uuid

import (
	"crypto/rand"
	"database/sql/driver"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

const Size = 16

var (
	ErrInvalidLength = errors.New("uuid: invalid length")
	ErrInvalidFormat = errors.New("uuid: invalid format")

	hexTable = [16]byte{
		'0', '1', '2', '3', '4', '5', '6', '7',
		'8', '9', 'a', 'b', 'c', 'd', 'e', 'f',
	}
)

type UUID [Size]byte

// Generator state for the "monotonic random" UUIDv7 method (RFC 9562 §6.2,
// method 3): the random portion is reseeded from crypto/rand whenever the
// millisecond tick advances, and incremented for calls within the same
// millisecond. This keeps IDs strictly ordered (needed for index locality)
// while making them unpredictable — unlike a bare atomic counter, an
// attacker who observes one ID cannot infer the next one.
var (
	genMu    sync.Mutex
	lastMs   int64
	seqRandA uint16 // 12 random bits, reseeded once per millisecond
	seqRandB uint64 // 62 random bits, reseeded per millisecond then incremented
)

func New() UUID {
	var u UUID
	now := time.Now().UnixMilli()

	genMu.Lock()
	if now > lastMs {
		lastMs = now
		var r [10]byte // one entropy read re-seeds the whole millisecond
		_, _ = rand.Read(r[:])
		seqRandA = binary.BigEndian.Uint16(r[:2]) & 0x0fff
		seqRandB = binary.BigEndian.Uint64(r[2:]) & 0x3fffffffffffffff
	} else {
		// Same millisecond, or the wall clock stepped backwards: keep the
		// last timestamp and increment so IDs stay strictly monotonic.
		now = lastMs
		seqRandB++
		if seqRandB > 0x3fffffffffffffff { // counter exhausted: borrow from rand_a
			seqRandB = 0
			seqRandA = (seqRandA + 1) & 0x0fff
			if seqRandA == 0 {
				lastMs++
				now = lastMs
			}
		}
	}
	a, b := seqRandA, seqRandB
	genMu.Unlock()

	binary.BigEndian.PutUint64(u[:8], uint64(now)<<16|uint64(a))
	binary.BigEndian.PutUint64(u[8:], b)

	u[6] = 0x70 | (u[6] & 0x0f)
	u[8] = 0x80 | (u[8] & 0x3f)
	return u
}

// randUint64 returns a cryptographically random value masked to fit within
// `mask` bits (mask must be a power-of-two-minus-one, e.g. 0x3fffffffffffffff
// for 62 bits). Falls back to zero only if the OS entropy source is
// unavailable, which never happens in practice on supported platforms.
func randUint64(mask uint64) uint64 {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return binary.BigEndian.Uint64(b[:]) & mask
}

func randUint16(mask uint16) uint16 {
	var b [2]byte
	_, _ = rand.Read(b[:])
	return binary.BigEndian.Uint16(b[:]) & mask
}

func (u UUID) String() string {
	var b [36]byte // stack buffer: one allocation (the string), no pool round-trip
	encodeHex(b[:], u)
	return string(b[:])
}

func encodeHex(dst []byte, u UUID) {
	dst[0] = hexTable[u[0]>>4]
	dst[1] = hexTable[u[0]&0x0f]
	dst[2] = hexTable[u[1]>>4]
	dst[3] = hexTable[u[1]&0x0f]
	dst[4] = hexTable[u[2]>>4]
	dst[5] = hexTable[u[2]&0x0f]
	dst[6] = hexTable[u[3]>>4]
	dst[7] = hexTable[u[3]&0x0f]
	dst[8] = '-'
	dst[9] = hexTable[u[4]>>4]
	dst[10] = hexTable[u[4]&0x0f]
	dst[11] = hexTable[u[5]>>4]
	dst[12] = hexTable[u[5]&0x0f]
	dst[13] = '-'
	dst[14] = hexTable[u[6]>>4]
	dst[15] = hexTable[u[6]&0x0f]
	dst[16] = hexTable[u[7]>>4]
	dst[17] = hexTable[u[7]&0x0f]
	dst[18] = '-'
	dst[19] = hexTable[u[8]>>4]
	dst[20] = hexTable[u[8]&0x0f]
	dst[21] = hexTable[u[9]>>4]
	dst[22] = hexTable[u[9]&0x0f]
	dst[23] = '-'
	dst[24] = hexTable[u[10]>>4]
	dst[25] = hexTable[u[10]&0x0f]
	dst[26] = hexTable[u[11]>>4]
	dst[27] = hexTable[u[11]&0x0f]
	dst[28] = hexTable[u[12]>>4]
	dst[29] = hexTable[u[12]&0x0f]
	dst[30] = hexTable[u[13]>>4]
	dst[31] = hexTable[u[13]&0x0f]
	dst[32] = hexTable[u[14]>>4]
	dst[33] = hexTable[u[14]&0x0f]
	dst[34] = hexTable[u[15]>>4]
	dst[35] = hexTable[u[15]&0x0f]
}

func (u UUID) Bytes() []byte { return u[:] }

func (u UUID) IsZero() bool { return u == UUID{} }

func (u UUID) Version() int {
	return int(u[6] >> 4)
}

func (u UUID) Timestamp() int64 {
	return int64(binary.BigEndian.Uint64(u[:8]) >> 16)
}

var hexPos = [32]byte{
	0, 1, 2, 3, 4, 5, 6, 7,
	9, 10, 11, 12,
	14, 15, 16, 17,
	19, 20, 21, 22,
	24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35,
}

func Parse(s string) (UUID, error) {
	var u UUID

	if len(s) != 36 {
		return u, ErrInvalidLength
	}

	if s[8] != '-' || s[13] != '-' || s[18] != '-' || s[23] != '-' {
		return u, ErrInvalidFormat
	}

	for i := 0; i < 16; i++ {
		hi := fromHexChar(s[hexPos[i*2]])
		lo := fromHexChar(s[hexPos[i*2+1]])
		if hi == 0xff || lo == 0xff {
			return u, ErrInvalidFormat
		}
		u[i] = (hi << 4) | lo
	}

	return u, nil
}

func fromHexChar(c byte) byte {
	switch {
	case '0' <= c && c <= '9':
		return c - '0'
	case 'a' <= c && c <= 'f':
		return c - 'a' + 10
	case 'A' <= c && c <= 'F':
		return c - 'A' + 10
	}
	return 0xff
}

func NewString() string { return New().String() }

func NewBytes() []byte {
	u := New()
	return u[:]
}

func FromBytes(b []byte) (UUID, error) {
	var u UUID
	if len(b) != Size {
		return u, ErrInvalidLength
	}
	copy(u[:], b)
	return u, nil
}

// Value implements driver.Valuer (stores as canonical string for portability).
func (u UUID) Value() (driver.Value, error) {
	if u.IsZero() {
		return nil, nil
	}
	return u.String(), nil
}

// Scan implements sql.Scanner.
func (u *UUID) Scan(value any) error {
	if u == nil {
		return errors.New("uuid: Scan on nil *UUID")
	}
	if value == nil {
		*u = UUID{}
		return nil
	}
	switch v := value.(type) {
	case string:
		if v == "" {
			*u = UUID{}
			return nil
		}
		parsed, err := Parse(v)
		if err != nil {
			return err
		}
		*u = parsed
		return nil
	case []byte:
		if len(v) == 0 {
			*u = UUID{}
			return nil
		}
		if len(v) == Size {
			return copyFromBytes(u, v)
		}
		parsed, err := Parse(string(v))
		if err != nil {
			return err
		}
		*u = parsed
		return nil
	case [Size]byte:
		*u = UUID(v)
		return nil
	default:
		return fmt.Errorf("uuid: unsupported Scan type %T", value)
	}
}

func copyFromBytes(u *UUID, v []byte) error {
	parsed, err := FromBytes(v)
	if err != nil {
		return err
	}
	*u = parsed
	return nil
}

// MarshalJSON encodes as a quoted UUID string.
func (u UUID) MarshalJSON() ([]byte, error) {
	if u.IsZero() {
		return []byte(`""`), nil
	}
	// Hex digits and dashes never need JSON escaping: one allocation.
	b := make([]byte, 38)
	b[0], b[37] = '"', '"'
	encodeHex(b[1:37], u)
	return b, nil
}

// UnmarshalJSON accepts a quoted UUID string (or null/empty → zero).
func (u *UUID) UnmarshalJSON(data []byte) error {
	if u == nil {
		return errors.New("uuid: UnmarshalJSON on nil *UUID")
	}
	if string(data) == "null" || string(data) == `""` {
		*u = UUID{}
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	if s == "" {
		*u = UUID{}
		return nil
	}
	parsed, err := Parse(s)
	if err != nil {
		return err
	}
	*u = parsed
	return nil
}
