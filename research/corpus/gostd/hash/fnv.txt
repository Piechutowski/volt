package fnv // import "hash/fnv"

Package fnv implements FNV-1 and FNV-1a, non-cryptographic hash
functions created by Glenn Fowler, Landon Curt Noll, and Phong Vo. See
https://en.wikipedia.org/wiki/Fowler-Noll-Vo_hash_function.

All the hash.Hash implementations returned by this package also implement
encoding.BinaryMarshaler and encoding.BinaryUnmarshaler to marshal and unmarshal
the internal state of the hash.

FUNCTIONS

func New128() hash.Hash
    New128 returns a new 128-bit FNV-1 hash.Hash. Its Sum method will lay the
    value out in big-endian byte order.

func New128a() hash.Hash
    New128a returns a new 128-bit FNV-1a hash.Hash. Its Sum method will lay the
    value out in big-endian byte order.

func New32() hash.Hash32
    New32 returns a new 32-bit FNV-1 hash.Hash. Its Sum method will lay the
    value out in big-endian byte order.

func New32a() hash.Hash32
    New32a returns a new 32-bit FNV-1a hash.Hash. Its Sum method will lay the
    value out in big-endian byte order.

func New64() hash.Hash64
    New64 returns a new 64-bit FNV-1 hash.Hash. Its Sum method will lay the
    value out in big-endian byte order.

func New64a() hash.Hash64
    New64a returns a new 64-bit FNV-1a hash.Hash. Its Sum method will lay the
    value out in big-endian byte order.

