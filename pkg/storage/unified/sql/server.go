package sql

import (
	"context"

	infraDB "github.com/grafana/grafana/pkg/infra/db"
	"github.com/grafana/grafana/pkg/infra/tracing"
	"github.com/grafana/grafana/pkg/services/featuremgmt"
	"github.com/grafana/grafana/pkg/setting"
	"github.com/grafana/grafana/pkg/storage/unified/resource"
	"github.com/grafana/grafana/pkg/storage/unified/sql/db/dbimpl"
)

// Creates a new ResourceServer
func NewResourceServer(db infraDB.DB, cfg *setting.Cfg, features featuremgmt.FeatureToggles, tracer tracing.Tracer) (resource.ResourceServer, error) {
	opts := resource.ResourceServerOptions{
		Tracer: tracer,
	}

	eDB, err := dbimpl.ProvideResourceDB(db, cfg, features, tracer)
	if err != nil {
		return nil, err
	}
	store, err := NewBackend(BackendOptions{DBProvider: eDB, Tracer: tracer})
	if err != nil {
		return nil, err
	}
	opts.Backend = store
	opts.Diagnostics = store
	opts.Lifecycle = store

	if features.IsEnabledGlobally(featuremgmt.FlagUnifiedStorageSearch) {
		opts.Index = resource.NewResourceIndexServer()
		server, err := resource.NewResourceServer(opts)
		if err != nil {
			return nil, err
		}
		// initialze the search index
		// TODO: pass context to NewRsouceServer function
		_, err = server.Index(context.Background(), &resource.IndexRequest{})
		return server, err
	}

	return resource.NewResourceServer(opts)
}
