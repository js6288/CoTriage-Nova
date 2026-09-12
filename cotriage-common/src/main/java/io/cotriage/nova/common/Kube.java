package io.cotriage.nova.common;

import io.kubernetes.client.openapi.ApiClient;
import io.kubernetes.client.openapi.apis.CoreV1Api;
import io.kubernetes.client.util.ClientBuilder;
import io.kubernetes.client.util.KubeConfig;
import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;

public final class Kube {
  private Kube() {}

  public static ApiClient client() throws IOException {
    return client(Env.get("KUBECONFIG", ""));
  }

  public static ApiClient client(String kubeconfig) throws IOException {
    if (kubeconfig != null && !kubeconfig.isBlank()) {
      try (var reader = Files.newBufferedReader(Path.of(kubeconfig))) {
        return ClientBuilder.kubeconfig(KubeConfig.loadKubeConfig(reader)).build();
      }
    }
    return ClientBuilder.cluster().build();
  }

  public static CoreV1Api core() throws IOException {
    return new CoreV1Api(client());
  }
}
