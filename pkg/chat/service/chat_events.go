package chat_service

import (
	"context"
	"github.com/evolution-foundation/evolution-go/pkg/history"
	instance_model "github.com/evolution-foundation/evolution-go/pkg/instance/model"
)

func (c *chatService) ChatEvents(ctx context.Context, instance *instance_model.Instance, after int64) (history.Page, error) {
	if instance == nil {
		return history.Page{}, history.ErrUnavailable
	}
	return c.historyStore.Read(ctx, instance.Id, instance.Jid, after, 100)
}
