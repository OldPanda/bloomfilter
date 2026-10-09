// Compile against Guava 33.4.8-jre. The package grants access to the strategy
// overload so both compact-format strategy ordinals can be tested.
package com.google.common.hash;

import java.nio.file.Files;
import java.nio.file.Path;
import java.io.OutputStream;
import java.io.ByteArrayOutputStream;
import java.io.BufferedWriter;
import java.nio.ByteBuffer;
import java.nio.ByteOrder;
import java.nio.charset.StandardCharsets;
import java.util.Arrays;
import java.util.LinkedHashMap;
import java.util.Map;

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
    writeMurmurVectors(directory);
  }

  private static void writeMurmurVectors(Path directory) throws Exception {
    Map<String, byte[]> inputs = new LinkedHashMap<>();
    byte[] pattern = new byte[4097];
    int state = 1;
    for (int i = 0; i < pattern.length; i++) {
      state = state * 1664525 + 1013904223;
      pattern[i] = (byte) (state >>> 24);
    }
    // Every tail length after zero, one, two, and three complete 16-byte blocks.
    for (int length = 0; length <= 64; length++) {
      inputs.put("pattern_" + length, Arrays.copyOf(pattern, length));
    }
    for (int length : new int[] {127, 128, 129, 255, 256, 257, 1023, 1024, 1025, 4095, 4096, 4097}) {
      inputs.put("pattern_" + length, Arrays.copyOf(pattern, length));
    }
    // Force unsigned-byte handling in every tail position.
    for (int length = 1; length <= 32; length++) {
      byte[] bytes = new byte[length];
      Arrays.fill(bytes, (byte) 0xff);
      inputs.put("ff_" + length, bytes);
    }
    for (int length : new int[] {1, 15, 16, 17, 31, 32, 33}) {
      inputs.put("zero_" + length, new byte[length]);
    }
    inputs.put("ascii_foo", "foo".getBytes(StandardCharsets.UTF_8));
    inputs.put("ascii_fox", "The quick brown fox jumps over the lazy dog".getBytes(StandardCharsets.UTF_8));
    inputs.put("utf8", "雪☃🐼".getBytes(StandardCharsets.UTF_8));
    inputs.put("utf8_nfc", "é".getBytes(StandardCharsets.UTF_8));
    inputs.put("utf8_nfd", "e\u0301".getBytes(StandardCharsets.UTF_8));
    inputs.put("embedded_nul", new byte[] {'a', 0, 'b', 0, (byte) 0xff});
    inputs.put("int32_min", ByteBuffer.allocate(4).order(ByteOrder.LITTLE_ENDIAN).putInt(Integer.MIN_VALUE).array());
    inputs.put("int32_max", ByteBuffer.allocate(4).order(ByteOrder.LITTLE_ENDIAN).putInt(Integer.MAX_VALUE).array());
    inputs.put("int32_minus_one", ByteBuffer.allocate(4).order(ByteOrder.LITTLE_ENDIAN).putInt(-1).array());
    inputs.put("int64_min", ByteBuffer.allocate(8).order(ByteOrder.LITTLE_ENDIAN).putLong(Long.MIN_VALUE).array());
    inputs.put("int64_max", ByteBuffer.allocate(8).order(ByteOrder.LITTLE_ENDIAN).putLong(Long.MAX_VALUE).array());
    inputs.put("int64_minus_one", ByteBuffer.allocate(8).order(ByteOrder.LITTLE_ENDIAN).putLong(-1).array());

    try (BufferedWriter out = Files.newBufferedWriter(directory.resolve("murmur3_vectors.json"), StandardCharsets.UTF_8)) {
      out.write("{\n  \"guava_version\": \"33.4.8-jre\", \"seed\": 0, \"expected_insertions\": 500, \"error_rate\": 0.01,\n  \"vectors\": [\n");
      boolean first = true;
      for (Map.Entry<String, byte[]> entry : inputs.entrySet()) {
        if (!first) out.write(",\n");
        first = false;
        byte[] key = entry.getValue();
        out.write("    {\"name\": \"" + entry.getKey() + "\", \"input_hex\": \"" + hex(key)
            + "\", \"hash_hex\": \"" + Hashing.murmur3_128().hashBytes(key).toString() + "\"");
        for (BloomFilterStrategies strategy : BloomFilterStrategies.values()) {
          BloomFilter<byte[]> filter = BloomFilter.create(Funnels.byteArrayFunnel(), 500L, .01, strategy);
          filter.put(key);
          ByteArrayOutputStream bytes = new ByteArrayOutputStream();
          filter.writeTo(bytes);
          ByteBuffer wire = ByteBuffer.wrap(bytes.toByteArray()).order(ByteOrder.BIG_ENDIAN);
          int ordinal = wire.get() & 0xff;
          int hashes = wire.get() & 0xff;
          int words = wire.getInt();
          out.write(", \"mitz" + (ordinal == 0 ? "32" : "64") + "\": {\"hash_functions\": " + hashes
              + ", \"word_count\": " + words + ", \"set_bits\": [");
          boolean firstBit = true;
          for (int wordIndex = 0; wordIndex < words; wordIndex++) {
            long word = wire.getLong();
            for (int bit = 0; bit < 64; bit++) {
              if ((word & (1L << bit)) != 0) {
                if (!firstBit) out.write(", ");
                firstBit = false;
                out.write(Integer.toString(wordIndex * 64 + bit));
              }
            }
          }
          out.write("]}");
        }
        out.write("}");
      }
      out.write("\n  ]\n}\n");
    }
  }

  private static String hex(byte[] bytes) {
    char[] digits = "0123456789abcdef".toCharArray();
    char[] result = new char[bytes.length * 2];
    for (int i = 0; i < bytes.length; i++) {
      result[i * 2] = digits[(bytes[i] & 0xff) >>> 4];
      result[i * 2 + 1] = digits[bytes[i] & 15];
    }
    return new String(result);
  }
}
