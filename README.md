# bloomfilter

![Build](https://github.com/OldPanda/bloomfilter/actions/workflows/build.yml/badge.svg)
[![codecov](https://codecov.io/gh/OldPanda/bloomfilter/branch/master/graph/badge.svg?token=FCV788SCL7)](https://codecov.io/gh/OldPanda/bloomfilter)
[![Go Reference](https://pkg.go.dev/badge/github.com/OldPanda/bloomfilter.svg)](https://pkg.go.dev/github.com/OldPanda/bloomfilter)
[![Go Report Card](https://goreportcard.com/badge/github.com/OldPanda/bloomfilter)](https://goreportcard.com/report/github.com/OldPanda/bloomfilter)
[![Mentioned in Awesome Go](https://awesome.re/mentioned-badge-flat.svg)](https://github.com/avelino/awesome-go)

## Overview

Yet another Bloomfilter implementation in Go, compatible with Java's Guava library. This library borrows how [Java's Guava libraray](https://guava.dev/) implements Bloomfilter hashing strategies to achieve the serialization compatibility.

The library retains Go 1.16 source compatibility and requires a C compiler.
Use a supported, patched Go toolchain for production builds and security checks;
CI tests the latest stable and previous stable Go releases using `stable` and
`oldstable`, alongside pinned Go 1.26.9 and 1.27.2 builds and a separate Go 1.16.15
compatibility job. Rolling jobs check for the latest release on each run; lint,
vulnerability scans, and fuzzing use the latest stable toolchain. The library
retains C math calls for sizing compatibility.

Bit storage uses a private fixed-size `[]uint64` buffer protected by the filter's
mutex. MurmurHash3 x64_128 is implemented locally with seed zero and explicit
little-endian reads, using only the Go standard library. The library has no
third-party Go module dependencies. Storage and hashing retain the Guava bit
positions and serialization format.

## Installing

First pull the latest version of the library:

```
go get github.com/OldPanda/bloomfilter
```

Then import the this library in your code:

```
import "github.com/OldPanda/bloomfilter"
```

## Usage Examples

### Basic Usage

```Go
package main

import (
	"fmt"

	"github.com/OldPanda/bloomfilter"
)

func main() {
	// create bloomfilter with expected insertion=500, error rate=0.01
	bf, err := bloomfilter.NewBloomFilter(500, 0.01)
	if err != nil {
		panic(err)
	}
	// add number 0~199 into bloomfilter
	for i := 0; i < 200; i++ {
		bf.Put(i)
	}

	// check if number 100 and 200 are in bloomfilter
	fmt.Println(bf.MightContain(100))
	fmt.Println(bf.MightContain(200))
}
```

### Serialization

```Go
package main

import "github.com/OldPanda/bloomfilter"

func main() {
	// expected insertion=500, error rate=0.01
	bf, err := bloomfilter.NewBloomFilter(500, 0.01)
	if err != nil {
		panic(err)
	}
	// add 0~199 into bloomfilter
	for i := 0; i < 200; i++ {
		bf.Put(i)
	}

	// serialize bloomfilter to byte array
	bytes := bf.ToBytes()
	// handling the bytes ...
}
```

### Deserialization

```Go
package main

import (
	"fmt"

	"github.com/OldPanda/bloomfilter"
)

func main() {
	// create bloomfilter from byte array
	bf, err := bloomfilter.FromBytes(bytes)
	if err != nil {
		// reject invalid or oversized serialized data
		fmt.Println(err)
		return
	}
	// check whether number 100 is in bloomfilter
	fmt.Println(bf.MightContain(100))
}
```

`FromBytes` accepts one complete serialized filter of at most 64 MiB, including
its header. It rejects zero capacity, zero hash functions, truncated payloads,
and trailing bytes before allocating the bit array. Applications needing a
different size budget can call `FromBytesWithLimit(bytes, maxBytes)` with an
explicit limit in bytes (at least 14). Decoding allocates additional memory
approximately equal to the payload size, so choose the limit to fit your memory
budget and the number of simultaneous decodes.

Constructors use the same 64 MiB default serialized-size budget. They reject
non-finite error rates, nil strategies, configurations producing zero bits,
and configurations requiring more than 255 hashes. Use
`NewBloomFilterWithLimit(expectedInsertions, errorRate, maxBytes)` or
`NewBloomFilterWithStrategyAndLimit(expectedInsertions, errorRate, strategy, maxBytes)`
to select a different budget.

Filters support concurrent `Put`, `PutChecked`, `MightContain`,
`MightContainChecked`, and `ToBytes` calls.
Serialization takes a consistent snapshot and includes every capacity word,
including zeros. Never copy a filter value, even before its first operation:
copies share the bit array but have separate locks. Share the pointer returned
by a constructor or deserializer instead. Do not mutate a byte-slice key while
a filter operation is reading it.

### Key types and encodings

Empty strings and empty byte slices (including `[]byte(nil)`) are valid keys.
`Put` returns whether any bits changed, so reinserting a key can return `false`.
`MightContain` returns possible membership. Both return `false` for unsupported
types. Use `PutChecked`, `MightContainChecked`, or `GetBytesChecked` to distinguish
unsupported types from valid keys: they return `ErrUnsupportedKey` without
formatting or logging the key's value. A nil interface is unsupported; a typed
nil byte slice is the empty key. Named types need conversion to a supported type.

| Key type | Byte encoding | Matching Guava funnel |
| --- | --- | --- |
| `int32`, `uint32` | 4 bytes, little-endian; signed values use two's complement | `Funnels.integerFunnel()` with the same 32-bit pattern |
| `int64`, `uint64` | 8 bytes, little-endian; signed values use two's complement | `Funnels.longFunnel()` with the same 64-bit pattern |
| `int` | 4 bytes when the value fits `int32`, otherwise 8 | Integer or long funnel according to the value; use a fixed-width Go type for interchange |
| `string` | Raw Go string bytes, without Unicode normalization or UTF-8 validation | `Funnels.stringFunnel(StandardCharsets.UTF_8)` for valid UTF-8 strings; byte-array funnel for arbitrary bytes |
| `[]byte` | The supplied bytes, without copying | `Funnels.byteArrayFunnel()` |

```go
changed, err := bf.PutChecked(key)
if errors.Is(err, bloomfilter.ErrUnsupportedKey) {
    // Reject or explicitly encode the unsupported key type.
}
_ = changed // false with no error means the insertion did not change any bits
```

The example uses the standard-library `errors` package. Key encodings have no
type or domain prefix: `int32(65)`, `uint32(65)`, and `"A\x00\x00\x00"` all hash
the same bytes. Numeric values of different widths can hash different bytes.
Applications mixing identity domains should supply a canonical byte encoding
with a domain prefix and use the same encoding/funnel in Java. Existing nonempty
key encodings and the Guava wire format are unchanged; empty keys now work where
earlier versions silently rejected them.

A Bloom filter can produce false positives; verify positive membership against
authoritative storage when it affects a security decision. Bound key sizes and
insertion workloads at the application's trust boundary. Serialized filters have
no built-in authentication; authenticate an envelope or use protected storage
and transport when accepting snapshots from another party.

### Migrating existing 32-bit Go snapshots

`Murur128Mitz32` now uses Guava's hash loop (`1..k`). Earlier versions of this
Go library used `0..k` but serialized the same strategy ordinal. The byte
format cannot distinguish these two interpretations.

Use `FromBytes` for Guava snapshots and new Go snapshots. For strategy-zero
snapshots written by the old Go implementation, use `FromLegacyBytes` or
`FromLegacyBytesWithLimit`. The legacy reader preserves the old insertion and
lookup behavior, and its output remains legacy Go data. Rebuild those filters
from the original keys before switching to the corrected strategy or exchanging
them with Java; bits alone cannot reconstruct the original keys. The default
64-bit strategy is unaffected by this migration. Legacy readers apply the same
structural checks as `FromBytes`; previously truncated snapshots must be rebuilt.

### Verification

```bash
go test -race ./...
go vet ./...
# Go 1.18 or newer; fuzzed filters have a 4 KiB serialized-size budget
go test -run='^$' -fuzz='^FuzzFromBytesWithLimit$' -fuzztime=30s -parallel=2 -timeout=2m ./...
```

Both strategies are checked against fixtures generated by Java Guava 33.4.8-jre.
The Murmur3 corpus includes 128 independent Java reference hashes and filter bit
sets: every tail length through 64 bytes, larger block boundaries through 4097
bytes, high-bit and zero bytes, UTF-8, embedded NULs, and integer extremes. Raw
hash tests check all 16 slice offsets and input immutability. Filter tests check
both strategies' insertion, repeated insertion, lookup, serialization, and
rejection when any required bit is missing. Tests read the checked-in corpus
without requiring a Java runtime and verify the local MurmurHash3 implementation
through the private hash helper.
To regenerate the additional fixtures with a JDK and that Guava jar:

```bash
javac -cp /path/to/guava-33.4.8-jre.jar -d /tmp/guava-fixtures guava_dump_files/FixtureGenerator.java
java -cp /path/to/guava-33.4.8-jre.jar:/tmp/guava-fixtures com.google.common.hash.FixtureGenerator guava_dump_files
```

GitHub Actions runs tests and read-only formatting/vulnerability checks on
pushes and pull requests. Action commits and the Codecov CLI version are pinned;
Dependabot proposes Go dependency and action updates weekly. Coverage uploads
use Codecov's GitHub OIDC authentication in a separate job for pushes to `master`
and pull requests from this repository. Fork pull requests upload in a separate
read-only job using Codecov's public-repository tokenless support. Test and lint
jobs have no upload secret or repository write access.

CI fuzzes the standard and legacy deserializers for 30 seconds, with two workers,
a 4 KiB serialized-size budget per filter, and explicit test/job timeouts. Fuzz
tests use Go 1.18 build constraints, so Go 1.16 compatibility tests still run.
The lint job generates a `security-inventory` artifact containing the tested
commit, toolchain/platform metadata, the full selected module graph, and the
packages imported by code and tests. Failing fuzz inputs are saved as artifacts.

To regenerate the inventory locally with your production toolchain/platform:

```bash
mkdir -p target/security-inventory
git rev-parse HEAD > target/security-inventory/commit.txt
go env -json GOVERSION GOOS GOARCH CGO_ENABLED > target/security-inventory/environment.json
go list -m -json all > target/security-inventory/modules.json
go list -deps -test -json ./... > target/security-inventory/packages.json
```

Regenerate after dependency or toolchain changes; the ignored historical
`target/golist.json` is not an authoritative inventory. `govulncheck -test ./...`
scans imported code and tests. An advisory in an unused module in the full graph
needs separate triage; it is not evidence of a reachable library vulnerability.

## Benchmark

The benchmark testing runs on element insertion and query separately.

```Bash
» go test -bench . -benchmem ./...
# github.com/OldPanda/bloomfilter.test
goos: darwin
goarch: arm64
pkg: github.com/OldPanda/bloomfilter
BenchmarkBloomfilterInsertion-12                11091939                90.62 ns/op           17 B/op          1 allocs/op
BenchmarkBloomfilterQuery-12                    20389624                53.16 ns/op           15 B/op          1 allocs/op
BenchmarkBloomfilterDeserialization-12            293098              3767 ns/op           13200 B/op         52 allocs/op
PASS
ok      github.com/OldPanda/bloomfilter 3.719s
```
