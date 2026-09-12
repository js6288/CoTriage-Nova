package io.cotriage.nova.server;

import static io.cotriage.nova.server.Values.*;

import io.cotriage.nova.common.Kube;
import io.kubernetes.client.openapi.apis.CoreV1Api;
import io.kubernetes.client.openapi.models.*;
import java.time.Instant;
import java.util.*;

public class DeviceService {
  final EventStore events;
  final CoreV1Api kube;

  public DeviceService(EventStore events) throws Exception {
    this(events, Kube.core());
  }

  DeviceService(EventStore events, CoreV1Api kube) {
    this.events = events;
    this.kube = kube;
  }

  public Map<String, Object> devices(Map<String, String> query) {
    try {
      var nodes = kube.listNode().timeoutSeconds(8).execute().getItems();
      var collectors = new HashMap<String, String>();
      try {
        var pods =
            kube.listNamespacedPod(env("COTRIAGE_NAMESPACE", "cotriage"))
                .labelSelector(env("COLLECTOR_LABEL_SELECTOR", "app=cotriage-log-collector"))
                .timeoutSeconds(8)
                .execute()
                .getItems();
        for (var pod : pods)
          if (pod.getSpec() != null
              && pod.getSpec().getNodeName() != null
              && !pod.getSpec().getNodeName().isBlank())
            collectors.put(pod.getSpec().getNodeName(), collectorStatus(pod));
      } catch (Exception ignored) {
      }
      var items = new ArrayList<Map<String, Object>>();
      for (var node : nodes) {
        if (node.getMetadata() == null) continue;
        var labels =
            Objects.requireNonNullElse(node.getMetadata().getLabels(), Map.<String, String>of());
        if (labels.containsKey("node-role.kubernetes.io/control-plane")) continue;
        var status = nodeStatus(node);
        var equipment = labels.getOrDefault("cotriage.io/equipment", "").trim();
        var site = labels.getOrDefault("cotriage.io/site", "").trim();
        if (!trim(query, "site").isEmpty() && !trim(query, "site").equals(site)
            || !trim(query, "equipment").isEmpty() && !trim(query, "equipment").equals(equipment)
            || !trim(query, "status").isEmpty() && !trim(query, "status").equals(status)) continue;
        var name = node.getMetadata().getName();
        var item =
            obj(
                "node_name",
                name,
                "equipment",
                def(equipment, "unknown"),
                "site",
                def(site, "unknown"),
                "status",
                status,
                "collector_status",
                collectors.getOrDefault(name, "NotFound"));
        item.putAll(events.deviceStats(equipment));
        items.add(item);
      }
      items.sort(Comparator.comparing(m -> str(m, "node_name")));
      int total = items.size(),
          limit =
              integer(
                  query,
                  "limit",
                  Integer.parseInt(env("DEVICES_DEFAULT_LIMIT", "200")),
                  0,
                  Integer.parseInt(env("DEVICES_MAX_LIMIT", "1000"))),
          offset = integer(query, "offset", 0, 0, Integer.MAX_VALUE);
      offset = Math.min(offset, total);
      return obj(
          "items",
          limit > 0 ? items.subList(offset, Math.min(total, offset + limit)) : items,
          "total",
          total,
          "generated_at",
          Instant.now().toString());
    } catch (Exception e) {
      throw new ApiFailure(500, "Kubernetes devices query: " + e.getMessage());
    }
  }

  static String nodeStatus(V1Node node) {
    if (node.getStatus() != null && node.getStatus().getConditions() != null)
      for (var condition : node.getStatus().getConditions())
        if ("Ready".equals(condition.getType()))
          return "True".equals(condition.getStatus())
              ? "Ready"
              : "False".equals(condition.getStatus()) ? "NotReady" : "Unknown";
    return "Unknown";
  }

  static String collectorStatus(V1Pod pod) {
    if (pod.getStatus() == null) return "Unknown";
    if (pod.getStatus().getContainerStatuses() != null)
      for (var c : pod.getStatus().getContainerStatuses())
        if (c.getState() != null && c.getState().getWaiting() != null) {
          String reason = c.getState().getWaiting().getReason();
          return "CrashLoopBackOff".equals(reason) || "ImagePullBackOff".equals(reason)
              ? reason
              : "Unknown";
        }
    var phase = pod.getStatus().getPhase();
    return "Running".equals(phase) || "Pending".equals(phase) ? phase : "Unknown";
  }
}
