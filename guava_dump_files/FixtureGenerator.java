// Compile against Guava 33.4.8-jre. The package grants access to the strategy
// overload so both compact-format strategy ordinals can be tested.
package com.google.common.hash;

import java.nio.file.Files;
import java.nio.file.Path;
import java.io.OutputStream;

public final class FixtureGenerator {
  public static void main(String[] args) throws Exception {
    Path directory = Path.of(args[0]);
    Files.createDirectories(directory);
    for (BloomFilterStrategies strategy : BloomFilterStrategies.values()) {
      String name = strategy == BloomFilterStrategies.MURMUR128_MITZ_32 ? "mitz32" : "mitz64";
      BloomFilter<Integer> filter = BloomFilter.create(Funnels.integerFunnel(), 500L, .01, strategy);
      for (int i = 0; i < 100; i++) filter.put(i);
      try (OutputStream out = Files.newOutputStream(directory.resolve("500_0_01_0_to_99_" + name + ".dump"))) {
        filter.writeTo(out);
      }
      BloomFilter<byte[]> emptyKeyFilter = BloomFilter.create(Funnels.byteArrayFunnel(), 500L, .01, strategy);
      emptyKeyFilter.put(new byte[0]);
      try (OutputStream out = Files.newOutputStream(directory.resolve("500_0_01_empty_" + name + ".dump"))) {
        emptyKeyFilter.writeTo(out);
      }
    }
  }
}
