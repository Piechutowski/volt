package maphash // import "hash/maphash"

Package maphash provides hash functions on byte sequences and comparable values.
These hash functions are intended to be used to implement hash tables or other
data structures that need to map arbitrary strings or byte sequences to a
uniform distribution on unsigned 64-bit integers. Each different instance of a
hash table or data structure should use its own Seed.

The hash functions are not cryptographically secure. (See crypto/sha256 and
crypto/sha512 for cryptographic use.)

FUNCTIONS

func Bytes(seed Seed, b []byte) uint64
    Bytes returns the hash of b with the given seed.

    Bytes is equivalent to, but more convenient and efficient than:

        var h Hash
        h.SetSeed(seed)
        h.Write(b)
        return h.Sum64()

func Comparable[T comparable](seed Seed, v T) uint64
    Comparable returns the hash of comparable value v with the given seed
    such that Comparable(s, v1) == Comparable(s, v2) if v1 == v2. If v != v,
    then the resulting hash is randomly distributed.

func String(seed Seed, s string) uint64
    String returns the hash of s with the given seed.

    String is equivalent to, but more convenient and efficient than:

        var h Hash
        h.SetSeed(seed)
        h.WriteString(s)
        return h.Sum64()

func WriteComparable[T comparable](h *Hash, x T)
    WriteComparable adds x to the data hashed by h.


TYPES

type Hash struct {
	// Has unexported fields.
}
    A Hash computes a seeded hash of a byte sequence.

    The zero Hash is a valid Hash ready to use. A zero Hash chooses a random
    seed for itself during the first call to a Reset, Write, Seed, Clone,
    or Sum64 method. For control over the seed, use SetSeed.

    The computed hash values depend only on the initial seed and the sequence
    of bytes provided to the Hash object, not on the way in which the bytes are
    provided. For example, the three sequences

        h.Write([]byte{'f','o','o'})
        h.WriteByte('f'); h.WriteByte('o'); h.WriteByte('o')
        h.WriteString("foo")

    all have the same effect.

    Hashes are intended to be collision-resistant, even for situations where an
    adversary controls the byte sequences being hashed.

    A Hash is not safe for concurrent use by multiple goroutines, but a Seed is.
    If multiple goroutines must compute the same seeded hash, each can declare
    its own Hash and call SetSeed with a common Seed.

func (h *Hash) BlockSize() int
    BlockSize returns h's block size.

func (h *Hash) Clone() (hash.Cloner, error)
    Clone implements hash.Cloner.

func (h *Hash) Reset()
    Reset discards all bytes added to h. (The seed remains the same.)

func (h *Hash) Seed() Seed
    Seed returns h's seed value.

func (h *Hash) SetSeed(seed Seed)
    SetSeed sets h to use seed, which must have been returned by MakeSeed or
    by another Hash.Seed method. Two Hash objects with the same seed behave
    identically. Two Hash objects with different seeds will very likely behave
    differently. Any bytes added to h before this call will be discarded.

func (h *Hash) Size() int
    Size returns h's hash value size, 8 bytes.

func (h *Hash) Sum(b []byte) []byte
    Sum appends the hash's current 64-bit value to b. It exists for implementing
    hash.Hash. For direct calls, it is more efficient to use Hash.Sum64.

func (h *Hash) Sum64() uint64
    Sum64 returns h's current 64-bit value, which depends on h's seed and
    the sequence of bytes added to h since the last call to Hash.Reset or
    Hash.SetSeed.

    All bits of the Sum64 result are close to uniformly and independently
    distributed, so it can be safely reduced by using bit masking, shifting,
    or modular arithmetic.

func (h *Hash) Write(b []byte) (int, error)
    Write adds b to the sequence of bytes hashed by h. It always writes all of b
    and never fails; the count and error result are for implementing io.Writer.

func (h *Hash) WriteByte(b byte) error
    WriteByte adds b to the sequence of bytes hashed by h. It never fails;
    the error result is for implementing io.ByteWriter.

func (h *Hash) WriteString(s string) (int, error)
    WriteString adds the bytes of s to the sequence of bytes hashed by h.
    It always writes all of s and never fails; the count and error result are
    for implementing io.StringWriter.

type Seed struct {
	// Has unexported fields.
}
    A Seed is a random value that selects the specific hash function computed
    by a Hash. If two Hashes use the same Seeds, they will compute the same hash
    values for any given input. If two Hashes use different Seeds, they are very
    likely to compute distinct hash values for any given input.

    A Seed must be initialized by calling MakeSeed. The zero seed is
    uninitialized and not valid for use with Hash's SetSeed method.

    Each Seed value is local to a single process and cannot be serialized or
    otherwise recreated in a different process.

func MakeSeed() Seed
    MakeSeed returns a new random seed.

