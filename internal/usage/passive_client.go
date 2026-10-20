package usage

import (
	"context"
	"encoding/json"

	"ubunatic.com/harnez/internal/server"
	"ubunatic.com/harnez/internal/usagestore"
)

func SendPassiveUpdate(ctx context.Context, windows []usagestore.Window) server.UpdateResult {
	if _, _, _, _, _, err := usageDurations(); err != nil {
		return server.UpdateDropped
	}
	cfg, err := server.LoadConfig()
	if err != nil {
		return server.UpdateDropped
	}
	body, err := json.Marshal(struct {
		Windows []usagestore.Window `json:"windows"`
	}{windows})
	if err != nil {
		return server.UpdateDropped
	}
	return server.PostUpdate(ctx, cfg.Address(), usageSpecPassivePath, body, cfg.UpdateTimeout)
}
