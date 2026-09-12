package io.cotriage.nova.server;

import static io.cotriage.nova.server.Values.*;
import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.Mockito.*;

import com.sun.net.httpserver.HttpServer;
import io.kubernetes.client.openapi.ApiClient;
import io.kubernetes.client.openapi.apis.CoreV1Api;
import io.kubernetes.client.openapi.models.*;
import java.net.InetSocketAddress;
import java.nio.charset.StandardCharsets;
import java.util.*;
import org.junit.jupiter.api.Test;

class DeviceServiceTest {
  @Test
  void queriesKubernetesNodesAndPodsFiltersControlPlaneAndUsesRedisStats() throws Exception {
    var server = HttpServer.create(new InetSocketAddress("127.0.0.1", 0), 0);
    server.createContext(
        "/api/v1/nodes",
        exchange -> {
          byte[] body =
              """
              {"kind":"NodeList","apiVersion":"v1","metadata":{},"items":[
               {"metadata":{"name":"control","labels":{"node-role.kubernetes.io/control-plane":""}}},
               {"metadata":{"name":"worker-b","labels":{"cotriage.io/equipment":"machine-b","cotriage.io/site":"site-1"}},"status":{"conditions":[{"type":"Ready","status":"False"}]}},
               {"metadata":{"name":"worker-a","labels":{"cotriage.io/equipment":"machine-a","cotriage.io/site":"site-1"}},"status":{"conditions":[{"type":"Ready","status":"True"}]}}
              ]}
              """
                  .getBytes(StandardCharsets.UTF_8);
          exchange.getResponseHeaders().set("Content-Type", "application/json");
          exchange.sendResponseHeaders(200, body.length);
          exchange.getResponseBody().write(body);
          exchange.close();
        });
    server.createContext(
        "/api/v1/namespaces/cotriage/pods",
        exchange -> {
          byte[] body =
              """
              {"kind":"PodList","apiVersion":"v1","metadata":{},"items":[{"metadata":{"name":"collector-a"},"spec":{"nodeName":"worker-a","containers":[]},"status":{"phase":"Running"}}]}
              """
                  .getBytes(StandardCharsets.UTF_8);
          exchange.getResponseHeaders().set("Content-Type", "application/json");
          exchange.sendResponseHeaders(200, body.length);
          exchange.getResponseBody().write(body);
          exchange.close();
        });
    server.start();
    try {
      var events = mock(EventStore.class);
      when(events.deviceStats(anyString()))
          .thenReturn(obj("last_event_time", "2026-09-12T00:00:00Z", "ingest_rate_1m", 7));
      var api =
          new CoreV1Api(
              new ApiClient().setBasePath("http://127.0.0.1:" + server.getAddress().getPort()));
      var devices = new DeviceService(events, api);
      var response = devices.devices(Map.of());
      assertEquals(2, num(response, "total"));
      var first = (Map<String, Object>) list(response.get("items")).getFirst();
      assertEquals("worker-a", first.get("node_name"));
      assertEquals("Running", first.get("collector_status"));
      assertEquals(7, num(first, "ingest_rate_1m"));
      assertEquals(1, num(devices.devices(Map.of("status", "Ready", "site", "site-1")), "total"));
      var all = devices.devices(Map.of("offset", "99", "limit", "0"));
      assertEquals(
          2, list(all.get("items")).size(), "zero limit retains original unpaged behavior");
    } finally {
      server.stop(0);
    }
  }

  @Test
  void podWaitingReasonHasPriorityOverPhase() {
    var pod =
        new V1Pod()
            .status(
                new V1PodStatus()
                    .phase("Running")
                    .containerStatuses(
                        List.of(
                            new V1ContainerStatus()
                                .state(
                                    new V1ContainerState()
                                        .waiting(
                                            new V1ContainerStateWaiting()
                                                .reason("CrashLoopBackOff"))))));
    assertEquals("CrashLoopBackOff", DeviceService.collectorStatus(pod));
    assertEquals("Unknown", DeviceService.nodeStatus(new V1Node()));
  }
}
