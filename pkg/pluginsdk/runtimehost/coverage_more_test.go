package runtimehost_test

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/structpb"

	pluginv1 "github.com/prairie-server/prairie-plugin-sdk/pkg/pluginproto/prairie/plugin/v1"
	"github.com/prairie-server/prairie-plugin-sdk/pkg/pluginsdk/runtimehost"
)

func (f *fakeServer) MintScopedStream(_ context.Context, req *pluginv1.MintScopedStreamRequest) (*pluginv1.MintScopedStreamResponse, error) {
	return &pluginv1.MintScopedStreamResponse{
		StreamUrl:     "https://example.test/stream",
		PlayMethod:    req.GetPlayMethod(),
		ExpiresAtUnix: req.GetExpiresAtUnix(),
	}, nil
}

func (f *fakeServer) ResolveCatalogImageURLs(_ context.Context, req *pluginv1.ResolveCatalogImageURLsRequest) (*pluginv1.ResolveCatalogImageURLsResponse, error) {
	out := make(map[string]string, len(req.GetPaths()))
	for _, p := range req.GetPaths() {
		out[p] = "https://cdn.example/" + p
	}
	return &pluginv1.ResolveCatalogImageURLsResponse{Urls: out}, nil
}

type errServer struct {
	pluginv1.UnimplementedRuntimeHostServer
}

func (errServer) ListLibraries(context.Context, *pluginv1.ListLibrariesRequest) (*pluginv1.ListLibrariesResponse, error) {
	return nil, errors.New("boom")
}
func (errServer) CheckMediaPresence(context.Context, *pluginv1.CheckMediaPresenceRequest) (*pluginv1.CheckMediaPresenceResponse, error) {
	return nil, errors.New("boom")
}
func (errServer) ListInstalledPlugins(context.Context, *pluginv1.ListInstalledPluginsRequest) (*pluginv1.ListInstalledPluginsResponse, error) {
	return nil, errors.New("boom")
}
func (errServer) ListLibraryMedia(context.Context, *pluginv1.ListLibraryMediaRequest) (*pluginv1.ListLibraryMediaResponse, error) {
	return nil, errors.New("boom")
}
func (errServer) GetCatalogStats(context.Context, *pluginv1.GetCatalogStatsRequest) (*pluginv1.GetCatalogStatsResponse, error) {
	return nil, errors.New("boom")
}
func (errServer) CallPluginHTTP(context.Context, *pluginv1.CallPluginHTTPRequest) (*pluginv1.CallPluginHTTPResponse, error) {
	return nil, errors.New("boom")
}
func (errServer) ResolveCatalogImageURLs(context.Context, *pluginv1.ResolveCatalogImageURLsRequest) (*pluginv1.ResolveCatalogImageURLsResponse, error) {
	return nil, errors.New("boom")
}
func (errServer) PublishEvent(context.Context, *pluginv1.PublishEventRequest) (*pluginv1.PublishEventResponse, error) {
	return nil, errors.New("boom")
}
func (errServer) GetHostInfo(context.Context, *pluginv1.GetHostInfoRequest) (*pluginv1.GetHostInfoResponse, error) {
	return nil, errors.New("boom")
}
func (errServer) SetGlobalConfigEntry(context.Context, *pluginv1.SetGlobalConfigEntryRequest) (*pluginv1.SetGlobalConfigEntryResponse, error) {
	return nil, errors.New("boom")
}

func dialHost(t *testing.T, srv pluginv1.RuntimeHostServer) *grpc.ClientConn {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	g := grpc.NewServer()
	pluginv1.RegisterRuntimeHostServer(g, srv)
	go func() { _ = g.Serve(lis) }()
	t.Cleanup(g.Stop)
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(_ context.Context, _ string) (net.Conn, error) { return lis.Dial() }),
	)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func TestMintScopedStreamAndResolveImages(t *testing.T) {
	srv := &fakeServer{}
	conn := dial(t, srv)
	c := runtimehost.NewClient(conn)

	if _, err := c.MintScopedStream(context.Background(), runtimehost.ScopedStreamRequest{}); err == nil {
		t.Fatal("expected media file id required")
	}
	got, err := c.MintScopedStream(context.Background(), runtimehost.ScopedStreamRequest{
		MediaFileID: 9,
		PlayMethod:  "DirectPlay",
		ExpiresAt:   time.Unix(1700000000, 0),
	})
	if err != nil {
		t.Fatalf("MintScopedStream: %v", err)
	}
	if got.StreamURL == "" || got.PlayMethod != "DirectPlay" {
		t.Fatalf("got %+v", got)
	}

	empty, err := c.ResolveCatalogImageURLs(context.Background(), nil, "")
	if err != nil || len(empty) != 0 {
		t.Fatalf("empty paths: %v %v", empty, err)
	}
	urls, err := c.ResolveCatalogImageURLs(context.Background(), []string{"a.webp"}, "w500")
	if err != nil || urls["a.webp"] == "" {
		t.Fatalf("ResolveCatalogImageURLs: %v %v", urls, err)
	}
}

func TestCallPluginHTTPValidationAndDefaults(t *testing.T) {
	srv := &fakeServer{}
	conn := dial(t, srv)
	c := runtimehost.NewClient(conn)

	if _, err := c.CallPluginHTTP(context.Background(), runtimehost.CallPluginHTTPRequest{Path: "/x"}); err == nil {
		t.Fatal("expected installation id")
	}
	if _, err := c.CallPluginHTTP(context.Background(), runtimehost.CallPluginHTTPRequest{InstallationID: 1}); err == nil {
		t.Fatal("expected path")
	}
	if _, err := c.CallPluginHTTP(context.Background(), runtimehost.CallPluginHTTPRequest{
		InstallationID: 1,
		Path:           "/x",
		Query:          map[string]any{"bad": make(chan int)},
	}); err == nil {
		t.Fatal("expected query encode error")
	}
	resp, err := c.CallPluginHTTP(context.Background(), runtimehost.CallPluginHTTPRequest{
		InstallationID: 7,
		Path:           "/ping",
	})
	if err != nil || resp.StatusCode != 204 {
		t.Fatalf("CallPluginHTTP: %+v %v", resp, err)
	}
	if srv.callHTTPReq.GetMethod() != "GET" {
		t.Fatalf("default method = %q", srv.callHTTPReq.GetMethod())
	}

	emptyPresence, err := c.CheckMediaPresence(context.Background(), "tmdb", "movie", nil)
	if err != nil || len(emptyPresence) != 0 {
		t.Fatalf("empty ids: %v %v", emptyPresence, err)
	}
	if err := c.SetGlobalConfigEntry(context.Background(), "k", nil); err != nil {
		t.Fatalf("nil value: %v", err)
	}
}

func TestClientRPCErrorPaths(t *testing.T) {
	c := runtimehost.NewClient(dialHost(t, errServer{}))
	ctx := context.Background()
	if _, err := c.ListLibraries(ctx, ""); err == nil {
		t.Fatal("ListLibraries")
	}
	if _, err := c.CheckMediaPresence(ctx, "tmdb", "movie", []string{"1"}); err == nil {
		t.Fatal("CheckMediaPresence")
	}
	if _, err := c.ListInstalledPlugins(ctx); err == nil {
		t.Fatal("ListInstalledPlugins")
	}
	if _, err := c.ListLibraryMedia(ctx, runtimehost.ListLibraryMediaRequest{}); err == nil {
		t.Fatal("ListLibraryMedia")
	}
	if _, err := c.GetCatalogStats(ctx, nil); err == nil {
		t.Fatal("GetCatalogStats")
	}
	if _, err := c.CallPluginHTTP(ctx, runtimehost.CallPluginHTTPRequest{InstallationID: 1, Path: "/x"}); err == nil {
		t.Fatal("CallPluginHTTP")
	}
	if _, err := c.MintScopedStream(ctx, runtimehost.ScopedStreamRequest{MediaFileID: 1}); err == nil {
		t.Fatal("MintScopedStream")
	}
	if _, err := c.ResolveCatalogImageURLs(ctx, []string{"a"}, ""); err == nil {
		t.Fatal("ResolveCatalogImageURLs")
	}
	if err := c.PublishEvent(ctx, "n", map[string]any{"ok": true}); err == nil {
		t.Fatal("PublishEvent")
	}
	if _, err := c.GetHostInfo(ctx); err == nil {
		t.Fatal("GetHostInfo")
	}
	if err := c.SetGlobalConfigEntry(ctx, "k", map[string]any{"bad": make(chan int)}); err == nil {
		t.Fatal("SetGlobalConfigEntry encode")
	}
}

func TestCallPluginJSONErrorStringAndDecode(t *testing.T) {
	srv := &fakeServer{
		callHTTPResp: &pluginv1.CallPluginHTTPResponse{
			StatusCode: 200,
			Body:       []byte(`not-json`),
		},
	}
	conn := dial(t, srv)
	c := runtimehost.NewClient(conn)
	var dest map[string]any
	if err := c.CallPluginJSON(context.Background(), runtimehost.CallPluginJSONRequest{
		InstallationID: 1,
		Path:           "/x",
		Response:       &dest,
	}); err == nil {
		t.Fatal("expected decode error")
	}
	if err := c.CallPluginJSON(context.Background(), runtimehost.CallPluginJSONRequest{
		InstallationID:   1,
		Path:             "/x",
		MaxResponseBytes: 1,
		Response:         &dest,
	}); err == nil {
		t.Fatal("expected max bytes error")
	}
	if err := c.CallPluginJSON(context.Background(), runtimehost.CallPluginJSONRequest{
		InstallationID: 1,
		Path:           "/x",
		Request:        make(chan int),
	}); err == nil {
		t.Fatal("expected marshal error")
	}
	statusErr := &runtimehost.HTTPStatusError{StatusCode: 500, Body: []byte("x")}
	if statusErr.Error() == "" {
		t.Fatal("empty error string")
	}
	// empty body + nil response is fine
	srv.callHTTPResp = &pluginv1.CallPluginHTTPResponse{StatusCode: 204}
	if err := c.CallPluginJSON(context.Background(), runtimehost.CallPluginJSONRequest{
		InstallationID: 1,
		Path:           "/x",
	}); err != nil {
		t.Fatalf("empty ok: %v", err)
	}
}

func TestDiscoveryHelpersNilSafe(t *testing.T) {
	if runtimehost.Capability(nil, "x") != nil {
		t.Fatal("nil plugin")
	}
	if runtimehost.HasCapability(nil, "x") {
		t.Fatal("nil has")
	}
	if got := runtimehost.CapabilityMetadata(nil); got == nil || len(got) != 0 {
		t.Fatalf("nil metadata = %v", got)
	}
	if runtimehost.CapabilityMetadataString(nil, "k") != "" {
		t.Fatal("nil string")
	}
	if runtimehost.CapabilityMetadataStrings(nil, "k") != nil {
		t.Fatal("nil strings")
	}
	meta, _ := structpb.NewStruct(map[string]any{"n": float64(1), "s": "x", "arr": []any{"a", 2}})
	cap := &pluginv1.CapabilityDescriptor{Metadata: meta}
	if runtimehost.CapabilityMetadataString(cap, "n") != "" {
		t.Fatal("non-string metadata")
	}
	if got := runtimehost.CapabilityMetadataStrings(cap, "arr"); len(got) != 1 || got[0] != "a" {
		t.Fatalf("arr = %v", got)
	}
	if runtimehost.CapabilityMetadataStrings(cap, "missing") != nil {
		t.Fatal("missing arr")
	}
	if _, err := runtimehost.NewClient(dial(t, &fakeServer{})).ListInstalledPluginsByCapability(context.Background(), "x"); err != nil {
		t.Fatalf("ListInstalledPluginsByCapability empty: %v", err)
	}
}
