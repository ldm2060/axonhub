package biz

import (
	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/pkg/xcache"
	"github.com/ldm2060/axonhub/llm/httpclient"
)

func NewChannelServiceForTest(client *ent.Client) *ChannelService {
	mockSysSvc := &SystemService{
		AbstractService: &AbstractService{
			db: client,
		},
		Cache: xcache.NewFromConfig[ent.System](xcache.Config{Mode: xcache.ModeMemory}),
	}

	svc := NewChannelService(ChannelServiceParams{
		CacheConfig:   xcache.Config{Mode: xcache.ModeMemory},
		Ent:           client,
		SystemService: mockSysSvc,
		HttpClient:    httpclient.NewHttpClient(),
	})

	svc.SetEnabledChannelsForTest([]*Channel{})

	return svc
}
