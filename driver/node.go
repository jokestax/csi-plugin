package driver

import (
	"context"

	"github.com/container-storage-interface/spec/lib/go/csi"
)

func (d *Driver) NodeStageVolume(c context.Context, r *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	return nil, nil
}

func (d *Driver) NodeUnstageVolume(c context.Context, r *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	return nil, nil
}
func (d *Driver) NodePublishVolume(c context.Context, r *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	return nil, nil
}
func (d *Driver) NodeUnpublishVolume(c context.Context, r *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	return nil, nil
}
func (d *Driver) NodeGetVolumeStats(c context.Context, r *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	return nil, nil
}
func (d *Driver) NodeExpandVolume(c context.Context, r *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	return nil, nil
}
func (d *Driver) NodeGetCapabilities(c context.Context, r *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	return nil, nil
}
func (d *Driver) NodeGetInfo(c context.Context, r *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	return nil, nil
}
