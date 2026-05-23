package pkgsite

import (
	"context"
	"time"

	"golang.org/x/sync/errgroup"
)

type WarmKind string

const (
	WarmPackage  WarmKind = "package"
	WarmPackages WarmKind = "packages"
	WarmSymbols  WarmKind = "symbols"
	WarmVersions WarmKind = "versions"
	WarmVulns    WarmKind = "vulns"
)

type WarmJob struct {
	Kind WarmKind

	Package  PackageInput
	Packages PackagesInput
	Symbols  SymbolsInput
	Versions VersionsInput
	Vulns    VulnsInput

	Drain bool
}

type Warmer interface {
	Warm(ctx context.Context, jobs ...WarmJob)
}

type AsyncWarmerOptions struct {
	Concurrency    int
	RequestTimeout time.Duration
}

type AsyncWarmer struct {
	client *Client
	opts   AsyncWarmerOptions
}

func NewAsyncWarmer(client *Client, opts AsyncWarmerOptions) *AsyncWarmer {
	if opts.Concurrency <= 0 {
		opts.Concurrency = 2
	}
	if opts.RequestTimeout <= 0 {
		opts.RequestTimeout = 5 * time.Second
	}
	return &AsyncWarmer{client: client, opts: opts}
}

func (w *AsyncWarmer) Warm(ctx context.Context, jobs ...WarmJob) {
	if w == nil || w.client == nil || len(jobs) == 0 {
		return
	}
	copied := append([]WarmJob(nil), jobs...)
	base := context.WithoutCancel(ctx)
	go func() {
		group, groupCtx := errgroup.WithContext(base)
		group.SetLimit(w.opts.Concurrency)
		for _, job := range copied {
			group.Go(func() error {
				jobCtx, cancel := context.WithTimeout(groupCtx, w.opts.RequestTimeout)
				defer cancel()
				return w.run(withoutWarming(jobCtx), job)
			})
		}
		_ = group.Wait()
	}()
}

type warmState uint8

const (
	warmStateFetch warmState = iota
	warmStateDone
)

func (w *AsyncWarmer) run(ctx context.Context, job WarmJob) error {
	state := warmStateFetch
	for state != warmStateDone {
		next, err := w.step(ctx, &job)
		if err != nil {
			return err
		}
		state = next
	}
	return nil
}

func (w *AsyncWarmer) step(ctx context.Context, job *WarmJob) (warmState, error) {
	result, err := w.fetch(ctx, *job)
	if err != nil || result.Error != nil || !job.Drain {
		return warmStateDone, err
	}
	token, _ := result.Pagination["upstreamNextPageToken"].(string)
	if token == "" {
		return warmStateDone, nil
	}
	if !job.setToken(token) {
		return warmStateDone, nil
	}
	return warmStateFetch, nil
}

func (w *AsyncWarmer) fetch(ctx context.Context, job WarmJob) (Result, error) {
	switch job.Kind {
	case WarmPackage:
		return w.client.Package(ctx, job.Package)
	case WarmPackages:
		return w.client.Packages(ctx, job.Packages)
	case WarmSymbols:
		return w.client.Symbols(ctx, job.Symbols)
	case WarmVersions:
		return w.client.Versions(ctx, job.Versions)
	case WarmVulns:
		return w.client.Vulns(ctx, job.Vulns)
	default:
		return Result{}, nil
	}
}

func (j *WarmJob) setToken(token string) bool {
	switch j.Kind {
	case WarmPackages:
		j.Packages.Token = token
	case WarmSymbols:
		j.Symbols.Token = token
	case WarmVersions:
		j.Versions.Token = token
	case WarmVulns:
		j.Vulns.Token = token
	default:
		return false
	}
	return true
}

type warmingContextKey struct{}

func withoutWarming(ctx context.Context) context.Context {
	return context.WithValue(ctx, warmingContextKey{}, true)
}

func warmingDisabled(ctx context.Context) bool {
	disabled, _ := ctx.Value(warmingContextKey{}).(bool)
	return disabled
}
