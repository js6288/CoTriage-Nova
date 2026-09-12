package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type RawEventPayload struct {
	EventID   string `json:"event_id"`
	TS        string `json:"ts"`
	Equipment string `json:"equipment"`
	Log       struct {
		Raw string `json:"raw"`
	} `json:"log"`
}

type Config struct {
	LogPath      string
	RedisAddr    string
	Stream       string
	NodeName     string
	Equipment    string // 最终写入 payload，用 node label 优先填充
	StreamMaxLen int64
}

func getenv(key, def string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	return v
}

func loadConfig() Config {
	return Config{
		LogPath:   getenv("LOG_PATH", "/var/log/cotriage/input/current.log"),
		RedisAddr: getenv("REDIS_ADDR", "redis:6379"),
		Stream:    getenv("REDIS_STREAM", "cotriage.events.raw"),
		NodeName:  getenv("NODE_NAME", ""),
		// 兜底值：如果 Kube API 读不到 node label，会用这个；本地 debug 时你也可以手动设置 EQUIPMENT
		Equipment:    getenv("EQUIPMENT", ""),
		StreamMaxLen: getenvInt64("REDIS_STREAM_MAXLEN", 100000),
	}
}

func getenvInt64(key string, def int64) int64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return def
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed < 1 {
		log.Printf("[log-collector] invalid %s=%q, use default %d", key, value, def)
		return def
	}
	return parsed
}

func newKubeClientset() (*kubernetes.Clientset, error) {
	// 优先走 kubeconfig：方便本地 debug（launch.json 里已经设置了 KUBECONFIG）
	if kubeconfig := strings.TrimSpace(os.Getenv("KUBECONFIG")); kubeconfig != "" {
		cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, err
		}
		return kubernetes.NewForConfig(cfg)
	}

	// 再走 in-cluster（集群内 DaemonSet 运行）
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(cfg)
}

func getEquipmentFromNodeLabel(ctx context.Context, nodeName string) (string, error) {
	if strings.TrimSpace(nodeName) == "" {
		return "", errors.New("NODE_NAME is empty")
	}
	cs, err := newKubeClientset()
	if err != nil {
		return "", fmt.Errorf("init kube client: %w", err)
	}
	n, err := cs.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("get node %q: %w", nodeName, err)
	}
	v := strings.TrimSpace(n.Labels["cotriage.io/equipment"])
	if v == "" {
		return "", fmt.Errorf("label cotriage.io/equipment not found on node %q", nodeName)
	}
	return v, nil
}

func newRedisClient(addr string) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr: addr,
	})
}

func waitForFile(ctx context.Context, path string, poll time.Duration) error {
	t := time.NewTicker(poll)
	defer t.Stop()

	for {
		_, err := os.Stat(path)
		if err == nil {
			return nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("stat %s: %w", path, err)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
		}
	}
}

// tailAndPublish 会从文件末尾开始持续读取新行（类似 tail -F，但不处理轮转）
// 设计目标：先满足 MVP（replayer 持续追加 current.log），后续再加 logrotate/inode 处理。
func tailAndPublish(ctx context.Context, cfg Config, rdb *redis.Client) error {
	if err := waitForFile(ctx, cfg.LogPath, 1*time.Second); err != nil {
		return err
	}

	f, err := os.Open(cfg.LogPath)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	defer f.Close()

	// 从文件末尾开始，只消费新写入的行
	if _, err := f.Seek(0, io.SeekEnd); err != nil {
		return fmt.Errorf("seek end: %w", err)
	}

	reader := bufio.NewReaderSize(f, 256*1024)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				// 没有新数据就等一下
				time.Sleep(200 * time.Millisecond)
				continue
			}
			return fmt.Errorf("read line: %w", err)
		}

		line = strings.TrimRight(line, "\r\n")
		if strings.TrimSpace(line) == "" {
			continue
		}

		eventID := uuid.NewString()
		payload := RawEventPayload{
			EventID:   eventID,
			TS:        time.Now().UTC().Format(time.RFC3339Nano),
			Equipment: cfg.Equipment,
		}
		payload.Log.Raw = line

		b, err := json.Marshal(payload)
		if err != nil {
			// 理论不应发生；记录后跳过
			log.Printf("marshal payload failed: %v", err)
			continue
		}

		res := rdb.XAdd(ctx, &redis.XAddArgs{
			Stream: cfg.Stream,
			MaxLen: cfg.StreamMaxLen,
			Approx: true,
			Values: map[string]any{
				"event_id": eventID,
				"payload":  string(b),
			},
		})
		if err := res.Err(); err != nil {
			log.Printf("XADD failed (stream=%s): %v", cfg.Stream, err)
			// 简单退避，避免 redis 短暂不可用导致热循环
			time.Sleep(500 * time.Millisecond)
			continue
		}
	}
}

func main() {
	cfg := loadConfig()

	// 正确来源：读取“所在 node 的 label”
	if v, err := getEquipmentFromNodeLabel(context.Background(), cfg.NodeName); err == nil {
		cfg.Equipment = v
	} else {
		log.Printf("[log-collector] get equipment from node label failed: %v", err)
	}
	if strings.TrimSpace(cfg.Equipment) == "" {
		cfg.Equipment = getenv("HOSTNAME", "unknown")
	}
	log.Printf("[log-collector] start: LOG_PATH=%s REDIS_ADDR=%s STREAM=%s STREAM_MAXLEN=%d EQUIPMENT=%s", cfg.LogPath, cfg.RedisAddr, cfg.Stream, cfg.StreamMaxLen, cfg.Equipment)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	rdb := newRedisClient(cfg.RedisAddr)
	defer func() { _ = rdb.Close() }()

	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Printf("[log-collector] redis ping failed: %v", err)
	}

	if err := tailAndPublish(ctx, cfg, rdb); err != nil && !errors.Is(err, context.Canceled) {
		log.Printf("[log-collector] exit with error: %v", err)
		os.Exit(1)
	}
	log.Printf("[log-collector] exit")
}
