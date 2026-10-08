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
CI tests the patched Go 1.26 and 1.27 releases and runs a separate Go 1.16.15
compatibility job. The library retains C math calls for sizing compatibility.

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

Filters support concurrent `Put`, `MightContain`, and `ToBytes` calls.
Serialization takes a consistent snapshot and includes every capacity word,
including zeros. Never copy a filter value, even before its first operation:
copies share the bit array but have separate locks. Share the pointer returned
by a constructor or deserializer instead. Do not mutate a byte-slice key while
a filter operation is reading it.

Supported keys are `int`, `int32`, `uint32`, `int64`, `uint64`, `string`, and
`[]byte`. Unsupported keys and empty strings/byte slices return `false` without
logging their values. Integer encodings are little-endian; strings use their Go
byte representation. For Guava interoperability, use an identical funnel and
key encoding. A Bloom filter can produce false positives; verify positive
membership against authoritative storage when it affects a security decision.

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
```

Both strategies are checked against fixtures generated by Java Guava 33.4.8-jre.
To regenerate the additional fixtures with a JDK and that Guava jar:

```bash
javac -cp /path/to/guava-33.4.8-jre.jar -d /tmp/guava-fixtures guava_dump_files/FixtureGenerator.java
java -cp /path/to/guava-33.4.8-jre.jar:/tmp/guava-fixtures com.google.common.hash.FixtureGenerator guava_dump_files
```

GitHub Actions runs tests and read-only formatting/vulnerability checks on
pushes and pull requests. Action commits and the Codecov CLI version are pinned;
Dependabot proposes Go dependency and action updates weekly. Coverage uploads
use Codecov's GitHub OIDC authentication in a separate job only for pushes to
`master`, so test and lint jobs have no upload secret or repository write access.

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
