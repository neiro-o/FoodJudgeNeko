package database

import (
	"context"
	"fmt"
)

func Ready(ctx context.Context) error {
	if Client == nil || RedisClient == nil || ESClient == nil {
		return fmt.Errorf("dependencies not connected")
	}
	if err := Client.Ping(ctx, nil); err != nil {
		return err
	}
	if err := RedisClient.Ping(ctx).Err(); err != nil {
		return err
	}
	res, err := ESClient.Ping(ESClient.Ping.WithContext(ctx))
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.IsError() {
		return fmt.Errorf("Elasticsearch unavailable")
	}
	return nil
}
