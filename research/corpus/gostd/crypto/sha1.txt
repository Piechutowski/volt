package sha1 // import "crypto/sha1"

Package sha1 implements the SHA-1 hash algorithm as defined in RFC 3174.

SHA-1 is cryptographically broken and should not be used for secure
applications.

CONSTANTS

const BlockSize = 64
    The blocksize of SHA-1 in bytes.

const Size = 20
    The size of a SHA-1 checksum in bytes.


FUNCTIONS

func New() hash.Hash
    New returns a new hash.Hash computing the SHA1 checksum. The Hash
    also implements encoding.BinaryMarshaler, encoding.BinaryAppender and
    encoding.BinaryUnmarshaler to marshal and unmarshal the internal state of
    the hash.

func Sum(data []byte) [Size]byte
    Sum returns the SHA-1 checksum of the data.

