package resource

import (
	"context"
	"errors"
	"strings"

	"google.golang.org/grpc"
)

type indexServer struct {
	s     *server
	index *Index
	ws    *indexWatchServer
}

func (is indexServer) Search(ctx context.Context, req *SearchRequest) (*SearchResponse, error) {
	res := &SearchResponse{}
	return res, nil
}

func (is indexServer) History(ctx context.Context, req *HistoryRequest) (*HistoryResponse, error) {
	return nil, nil
}

func (is indexServer) Origin(ctx context.Context, req *OriginRequest) (*OriginResponse, error) {
	return nil, nil
}

func (is *indexServer) Index(ctx context.Context, req *IndexRequest) (*IndexResponse, error) {
	if req.Key == nil {
		is.index = NewIndex(is.s, Opts{})
		err := is.index.Init(ctx)
		if err != nil {
			return nil, err
		}
		return nil, nil
	}

	err := is.index.Index(ctx, &Data{Key: req.Key, Value: req.Value})
	if err != nil {
		return nil, err
	}
	return nil, nil
}

// Init sets the resource server on the index server
// so we can call the resource server from the index server
// TODO: a chicken and egg problem - index server needs the resource server but the resource server is created with the index server
func (is *indexServer) Init(ctx context.Context, rs *server) error {
	is.s = rs
	is.ws = &indexWatchServer{
		is:      is,
		context: ctx,
	}
	// TODO: how to watch all resources?
	wr := &WatchRequest{
		Options: &ListOptions{
			Key: &ResourceKey{
				Group:    "playlist.grafana.app",
				Resource: "playlists",
			},
		},
	}
	// TODO: handle watch error
	var err error
	go func() {
		err = rs.Watch(wr, is.ws)
	}()
	return err
}

func NewResourceIndexServer() ResourceIndexServer {
	return &indexServer{}
}

type ResourceIndexer interface {
	Init(context.Context, *server) error
}

type indexWatchServer struct {
	grpc.ServerStream
	context context.Context
	is      *indexServer
}

func (f *indexWatchServer) Send(we *WatchEvent) error {
	r, err := getResource(we.Resource.Value)
	if err != nil {
		return err
	}

	key := &ResourceKey{
		Group:     getGroup(r),
		Resource:  r.Kind,
		Namespace: r.Metadata.Namespace,
		Name:      r.Metadata.Name,
	}

	value := &ResourceWrapper{
		ResourceVersion: we.Resource.Version,
		Value:           we.Resource.Value,
	}

	_, err = f.is.Index(f.context, &IndexRequest{Key: key, Value: value})
	if err != nil {
		return err
	}
	return nil
}

func (f *indexWatchServer) RecvMsg(m interface{}) error {
	return nil
}

func (f *indexWatchServer) SendMsg(m interface{}) error {
	return errors.New("not implemented")
}

func (f *indexWatchServer) Context() context.Context {
	if f.context == nil {
		f.context = context.Background()
	}
	return f.context
}

type Data struct {
	Key   *ResourceKey
	Value *ResourceWrapper
}

func getGroup(r *Resource) string {
	v := strings.Split(r.ApiVersion, "/")
	if len(v) > 0 {
		return v[0]
	}
	return ""
}
