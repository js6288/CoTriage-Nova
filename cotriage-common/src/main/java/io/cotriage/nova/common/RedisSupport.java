package io.cotriage.nova.common;

import io.lettuce.core.RedisClient;
import io.lettuce.core.RedisURI;

public final class RedisSupport {
  private RedisSupport() {}

  public static RedisURI uri(String address) {
    return RedisURI.create(address.contains("://") ? address : "redis://" + address);
  }

  public static RedisClient client(String address) {
    return RedisClient.create(uri(address));
  }
}
