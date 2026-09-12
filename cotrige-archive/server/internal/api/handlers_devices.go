package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pangjian-pj/cotriage-dev/server/internal/model"
	"github.com/pangjian-pj/cotriage-dev/server/internal/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (h *Handler) ListDevices(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 8*time.Second)
	defer cancel()

	qSite := strings.TrimSpace(c.Query("site"))
	qEquipment := strings.TrimSpace(c.Query("equipment"))
	qStatus := strings.TrimSpace(c.Query("status"))
	limit := util.ClampInt(util.QueryInt(c, "limit", h.Cfg.DevicesDefaultLimit), 0, h.Cfg.DevicesMaxLimit)
	offset := util.MaxInt(util.QueryInt(c, "offset", 0), 0)

	nl, err := h.Kube.Client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 查询 log-collector pod，按 nodeName 映射状态
	pl, err := h.Kube.Client.CoreV1().Pods(h.Cfg.DevicesNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: h.Cfg.CollectorLabelSelector,
	})
	if err != nil {
		pl = &corev1.PodList{}
	}
	collectorByNode := map[string]model.CollectorStatus{}
	for _, p := range pl.Items {
		if p.Spec.NodeName == "" {
			continue
		}
		collectorByNode[p.Spec.NodeName] = podCollectorStatus(&p)
	}

	items := make([]model.DeviceItem, 0, len(nl.Items))
	for _, n := range nl.Items {
		// 只把 worker 当设备：不含 control-plane role
		if _, ok := n.Labels["node-role.kubernetes.io/control-plane"]; ok {
			continue
		}

		status := nodeReadyStatus(&n)
		equipment := strings.TrimSpace(n.Labels["cotriage.io/equipment"])
		site := strings.TrimSpace(n.Labels["cotriage.io/site"])

		if qSite != "" && site != qSite {
			continue
		}
		if qEquipment != "" && equipment != qEquipment {
			continue
		}
		if qStatus != "" && string(status) != qStatus {
			continue
		}

		collector := collectorByNode[n.Name]
		if collector == "" {
			collector = model.CollectorStatusNotFound
		}

		lastTS, rate1m := h.redisStats(ctx, equipment)

		items = append(items, model.DeviceItem{
			NodeName:        n.Name,
			Equipment:       fallbackUnknown(equipment),
			Site:            fallbackUnknown(site),
			Status:          status,
			CollectorStatus: collector,
			LastEventTime:   lastTS,
			IngestRate1m:    rate1m,
		})
	}

	sort.Slice(items, func(i, j int) bool { return items[i].NodeName < items[j].NodeName })

	total := len(items)
	if offset > total {
		offset = total
	}
	end := total
	if limit > 0 && offset+limit < end {
		end = offset + limit
	}
	paged := items
	if limit > 0 {
		paged = items[offset:end]
	}

	resp := model.ListDevicesResponse{
		Items:       paged,
		Total:       total,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	c.JSON(http.StatusOK, resp)
}

func fallbackUnknown(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "unknown"
	}
	return v
}

func nodeReadyStatus(n *corev1.Node) model.DeviceStatus {
	for _, cond := range n.Status.Conditions {
		if cond.Type != corev1.NodeReady {
			continue
		}
		switch cond.Status {
		case corev1.ConditionTrue:
			return model.DeviceStatusReady
		case corev1.ConditionFalse:
			return model.DeviceStatusNotReady
		default:
			return model.DeviceStatusUnknown
		}
	}
	return model.DeviceStatusUnknown
}

func podCollectorStatus(p *corev1.Pod) model.CollectorStatus {
	for _, cs := range p.Status.ContainerStatuses {
		if cs.State.Waiting != nil {
			r := cs.State.Waiting.Reason
			switch r {
			case string(model.CollectorStatusCrashLoop):
				return model.CollectorStatusCrashLoop
			case string(model.CollectorStatusImagePullError):
				return model.CollectorStatusImagePullError
			default:
				return model.CollectorStatusUnknown
			}
		}
	}

	switch p.Status.Phase {
	case corev1.PodRunning:
		return model.CollectorStatusRunning
	case corev1.PodPending:
		return model.CollectorStatusPending
	default:
		return model.CollectorStatusUnknown
	}
}

func (h *Handler) redisStats(ctx context.Context, equipment string) (*string, int) {
	equipment = strings.TrimSpace(equipment)
	if equipment == "" {
		return nil, 0
	}

	// 先拿 stream 最新一条，作为“now”的锚点。
	// 这样即使本机时间（或 Redis TIME）与生产写入时间有漂移，也能保证窗口以真实数据时间为准。
	latest, err := h.Redis.XRevRangeN(ctx, h.Cfg.RedisStream, "+", "-", 1).Result()
	if err != nil || len(latest) == 0 {
		return nil, 0
	}

	// 最新消息的 stream-id 形如: <msTime>-<seq>
	// 取 msTime 部分作为时间戳。
	parts := strings.SplitN(latest[0].ID, "-", 2)
	if len(parts) == 0 {
		return nil, 0
	}
	ms, err := util.ParseInt64(parts[0])
	if err != nil {
		return nil, 0
	}
	now := time.UnixMilli(ms).UTC()
	nowID := latest[0].ID

	start := now.Add(-h.Cfg.EventsLookback)
	startID := fmt.Sprintf("%d-0", start.UnixMilli())

	msgs, err := h.Redis.XRange(ctx, h.Cfg.RedisStream, startID, nowID).Result()
	if err != nil {
		return nil, 0
	}

	var lastTS *string
	rate := 0
	for _, m := range msgs {
		pv, _ := m.Values["payload"].(string)
		if pv == "" {
			continue
		}
		var pp struct {
			TS        string `json:"ts"`
			Equipment string `json:"equipment"`
		}
		if err := json.Unmarshal([]byte(pv), &pp); err != nil {
			continue
		}
		if strings.TrimSpace(pp.Equipment) == equipment {
			rate++
			ts := strings.TrimSpace(pp.TS)
			if ts != "" {
				lastTS = &ts
			}
		}
	}

	return lastTS, rate
}

// keep util in deps so Go doesn't complain in future refactors
var _ = util.UnmarshalJSON[any]
