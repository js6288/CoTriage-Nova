package api

import (
	redis "github.com/redis/go-redis/v9"

	"github.com/pangjian-pj/cotriage-dev/server/internal/config"
	"github.com/pangjian-pj/cotriage-dev/server/internal/kube"
	"github.com/pangjian-pj/cotriage-dev/server/internal/store"
	"gorm.io/gorm"
)

type Handler struct {
	Cfg   config.Config
	Kube  *kube.Kube
	Redis *redis.Client

	DB                  *gorm.DB
	EdgeProvidersStore  *store.EdgeProvidersStore
	CloudProvidersStore *store.CloudProvidersStore
	ClassifierStore     *store.ClassifierConfigsStore
	EdgeKnowledgeStore  *store.EdgeKnowledgeStore
	RouterStore         *store.RouterStore
}
