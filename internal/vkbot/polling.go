package vkbot

import (
	"context"
	"fmt"
	"time"

	"github.com/ArtemST2006/VkBot/internal/lib/logger"
)

// RunPolling receives VK community updates and passes them to the handler.
func RunPolling(ctx context.Context, client *Client, handler *Handler) error {
	server, err := client.GetLongPollServer(ctx)
	if err != nil {
		return err
	}
	for ctx.Err() == nil {
		result, err := client.GetUpdates(ctx, server)
		if err == nil {
			switch result.Failed {
			case 0:
				for _, update := range result.Updates {
					if err = handler.Handle(ctx, update); err != nil {
						break
					}
				}
				if err == nil {
					server.TS = result.TS
					continue
				}
			case 1:
				if result.TS == "" {
					err = fmt.Errorf("VK Long Poll expired ts without replacement")
				} else {
					server.TS = result.TS
					continue
				}
			case 2, 3:
				server, err = client.GetLongPollServer(ctx)
				if err == nil {
					continue
				}
			default:
				err = fmt.Errorf("VK Long Poll failed with code %d", result.Failed)
			}
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		handler.log.WarnContext(ctx, "VK update failed; retrying", logger.Err(err))
		timer := time.NewTimer(2 * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return ctx.Err()
}
