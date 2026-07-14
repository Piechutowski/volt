package sha256 // import "crypto/sha256"

Package sha256 implements the SHA224 and SHA256 hash algorithms as defined in
FIPS 180-4.

CONSTANTS

const BlockSize = 64
    The blocksize of SHA256 and SHA224 in bytes.

const Size = 32
    The size of a SHA256 checksum in bytes.

const Size224 = 28
    The size of a SHA224 checksum in bytes.


FUNCTIONS

func New() hash.Hash
    New returns a new hash.Hash computing the SHA256 checksum. The Hash
    also implements encoding.BinaryMarshaler, encoding.BinaryAppender and
    encoding.BinaryUnmarshaler to marshal and unmarshal the internal state of
    the hash.

func New224() hash.Hash
    New224 returns a new hash.Hash computing the SHA224 checksum. The Hash
    also implements encoding.BinaryMarshaler, encoding.BinaryAppender and
    encoding.BinaryUnmarshaler to marshal and unmarshal the internal state of
    the hash.

func Sum224(data []byte) [Size224]byte
    Sum224 returns the SHA224 checksum of the data.

func Sum256(data []byte) [Size]byte
    Sum256 returns the SHA256 checksum of the data.

