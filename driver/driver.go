package driver

import (
	"fmt"
	"net"
	"net/url"
	"path"
	"path/filepath"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc"
)

const defaultName = "jokesta-csi"

type Driver struct {
	name     string
	region   string
	endpoint string

	srv   *grpc.Server
	Ready bool
	csi.UnimplementedNodeServer
	csi.UnimplementedControllerServer
	csi.UnsafeIdentityServer
}

func NewDriver(region, endpoint string) *Driver {
	return &Driver{
		name:     defaultName,
		region:   region,
		endpoint: endpoint,
	}
}

func (d *Driver) Run() error {
	// Start the gRPC server
	url, err := url.Parse(d.endpoint)
	if err != nil {
		return fmt.Errorf("parsing the endpoint %s\n", err.Error())
	}

	if url.Scheme != "unix" {
		return fmt.Errorf("only supported scheme is unix, but provided %s\n", url.Scheme)
	}

	grpcAddress := path.Join(url.Host, filepath.FromSlash(url.Path))
	if url.Host == "" {
		grpcAddress = filepath.FromSlash(url.Path)
	}

	fmt.Println(url.Host, url.Path)
	listener, err := net.Listen("unix", grpcAddress)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	d.srv = grpc.NewServer()
	csi.RegisterNodeServer(d.srv, d)
	csi.RegisterControllerServer(d.srv, d)
	csi.RegisterIdentityServer(d.srv, d)
	d.Ready = true
	d.srv.Serve(listener)
	return nil
}
