package md5 // import "crypto/md5"

Package md5 implements the MD5 hash algorithm as defined in RFC 1321.

MD5 is cryptographically broken and should not be used for secure applications.

CONSTANTS

const BlockSize = 64
    The blocksize of MD5 in bytes.

const Size = 16
    The size of an MD5 checksum in bytes.


FUNCTIONS

func New() hash.Hash
    New returns a new hash.Hash computing the MD5 checksum. The Hash
    also implements encoding.BinaryMarshaler, encoding.BinaryAppender and
    encoding.BinaryUnmarshaler to marshal and unmarshal the internal state of
    the hash.

func Sum(data []byte) [Size]byte
    Sum returns the MD5 checksum of the data.

