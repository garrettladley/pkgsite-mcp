package pkgsite

import (
	"context"
	"time"

	"github.com/garrettladley/pkgsite-mcp/internal/observability"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
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

var _ Warmer = (*AsyncWarmer)(nil)

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
	trace.SpanFromContext(ctx).SetAttributes(observability.WarmAttrs{QueueCount: len(jobs), Outcome: observability.WarmOutcomeScheduled}.Attributes()...)
	copied := append([]WarmJob(nil), jobs...)
	base := context.WithoutCancel(ctx)
	go func() {
		group, groupCtx := errgroup.WithContext(base)
		group.SetLimit(w.opts.Concurrency)
		for _, job := range copied {
			group.Go(func() error {
				jobCtx, cancel := context.WithTimeout(groupCtx, w.opts.RequestTimeout)
				defer cancel()
				jobCtx, span := observability.Tracer("pkgsite-warm").Start(jobCtx, "pkgsite.warm "+string(job.Kind), trace.WithAttributes(observability.WarmAttrs{Kind: string(job.Kind), Drain: job.Drain}.Attributes()...))
				defer span.End()
				outcome, err := w.run(withoutWarming(jobCtx), job)
				if err != nil {
					span.RecordError(err)
					span.SetStatus(codes.Error, err.Error())
				}
				span.SetAttributes(observability.WarmAttrs{Kind: string(job.Kind), Drain: job.Drain, Outcome: outcome}.Attributes()...)
				return err
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

func (w *AsyncWarmer) run(ctx context.Context, job WarmJob) (observability.WarmOutcome, error) {
	if !validWarmKind(job.Kind) {
		return observability.WarmOutcomeUnknownKind, nil
	}
	outcome := observability.WarmOutcomeSuccess
	state := warmStateFetch
	for state != warmStateDone {
		next, nextOutcome, err := w.step(ctx, &job)
		if err != nil {
			return observability.WarmOutcomeTransportError, err
		}
		if nextOutcome != "" {
			outcome = nextOutcome
		}
		state = next
		if next == warmStateDone {
			return outcome, nil
		}
	}
	return outcome, nil
}

func (w *AsyncWarmer) step(ctx context.Context, job *WarmJob) (warmState, observability.WarmOutcome, error) {
	result, err := w.fetch(ctx, *job)
	if err != nil {
		return warmStateDone, observability.WarmOutcomeTransportError, err
	}
	if result.Error != nil {
		return warmStateDone, observability.WarmOutcomeAPIError, nil
	}
	if !job.Drain {
		return warmStateDone, observability.WarmOutcomeSuccess, nil
	}
	token, _ := result.Pagination["upstreamNextPageToken"].(string)
	if token == "" {
		return warmStateDone, observability.WarmOutcomeSuccess, nil
	}
	if !job.setToken(token) {
		return warmStateDone, observability.WarmOutcomeSkipped, nil
	}
	return warmStateFetch, "", nil
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

func validWarmKind(kind WarmKind) bool {
	switch kind {
	case WarmPackage, WarmPackages, WarmSymbols, WarmVersions, WarmVulns:
		return true
	default:
		return false
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
