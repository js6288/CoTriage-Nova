package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/pangjian-pj/cotriage-dev/edge-infer/internal/config"
	"github.com/pangjian-pj/cotriage-dev/edge-infer/internal/infer"
	"github.com/pangjian-pj/cotriage-dev/edge-infer/internal/worker"
	"github.com/redis/go-redis/v9"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("[edge-infer] load config: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	defer func() { _ = rdb.Close() }()

	rules, err := infer.LoadRules(cfg.RulesFile)
	if err != nil {
		log.Fatalf("[edge-infer] load rules: %v", err)
	}
	classifier, err := infer.NewHTTPClassifier(cfg.ClassifierURL, cfg.ClassifierTimeout)
	if err != nil {
		log.Fatalf("[edge-infer] create classifier: %v", err)
	}
	edgeLLM, err := infer.NewOpenAIChatLLM(cfg.LLMURL, cfg.LLMTimeout)
	if err != nil {
		log.Fatalf("[edge-infer] create edge llm: %v", err)
	}
	engine, err := infer.NewEngine(rdb, rules, cfg.TemplatesFile, cfg.TemplateCacheTTL, cfg.LogMaxBytes, classifier, edgeLLM)
	if err != nil {
		log.Fatalf("[edge-infer] create engine: %v", err)
	}
	infer.StartRemoteConfigSync(ctx, engine, cfg.EdgeConfigURL, cfg.ConfigRefresh, cfg.ConfigTimeout)

	log.Printf("[edge-infer] start redis=%s raw=%s inferred=%s group=%s consumer=%s classifier=%t edge_llm=%t edge_config=%t", cfg.RedisAddr, cfg.RawStream, cfg.InferredStream, cfg.ConsumerGroup, cfg.ConsumerName, classifier != nil, edgeLLM != nil, cfg.EdgeConfigURL != "")
	if err := worker.New(rdb, cfg, engine).Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Printf("[edge-infer] exit with error: %v", err)
		os.Exit(1)
	}
	log.Printf("[edge-infer] exit")
}
