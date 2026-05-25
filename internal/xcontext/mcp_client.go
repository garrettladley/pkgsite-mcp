package xcontext

import "context"

type mcpClientKey struct{}

type MCPClient struct {
	Address string
	Port    int
}

func WithMCPClient(ctx context.Context, client MCPClient) context.Context {
	if client.Address == "" && client.Port <= 0 {
		return ctx
	}
	return context.WithValue(ctx, mcpClientKey{}, client)
}

func MCPClientFrom(ctx context.Context) (MCPClient, bool) {
	client, ok := ctx.Value(mcpClientKey{}).(MCPClient)
	return client, ok
}

type warmingDisabledKey struct{}

func WithoutWarming(ctx context.Context) context.Context {
	return context.WithValue(ctx, warmingDisabledKey{}, true)
}

func WarmingDisabled(ctx context.Context) bool {
	disabled, _ := ctx.Value(warmingDisabledKey{}).(bool)
	return disabled
}
