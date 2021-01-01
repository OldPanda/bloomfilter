# bloomfilter
![Build](https://github.com/OldPanda/bloomfilter/workflows/Build/badge.svg?event=push)

## Overview
Yet another Bloomfilter implementation in Go, compatible with Java's Guava library. This library borrows how [Java's Guava libraray](https://guava.dev/) implements Bloomfilter hashing strategies to achieve the serialization compatibility.

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
    bf, _ := bloomfilter.NewBloomFilter(500, 0.01)
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
    bf, _ := bloomfilter.NewBloomFilter(500, 0.01)
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
    bf, _ := bloomfilter.FromBytes(bytes)
    // check whether number 100 is in bloomfilter
    fmt.Println(bf.MightContain(100))
}
```
