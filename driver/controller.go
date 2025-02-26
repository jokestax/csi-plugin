package driver

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/civo/civogo"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const DefaultVolumeSizeGB int = 10

// BytesInGigabyte describes how many bytes are in a gigabyte
const BytesInGigabyte int64 = 1024 * 1024 * 1024

// CivoVolumeAvailableRetries is the number of times we will retry to check if a volume is available
const CivoVolumeAvailableRetries int = 20

var supportedAccessModes = map[csi.VolumeCapability_AccessMode_Mode]struct{}{
	csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER:      {},
	csi.VolumeCapability_AccessMode_SINGLE_NODE_READER_ONLY: {},
}

func (d *Driver) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	log.Info().Msg("Request: CreateVolume")

	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "CreateVolume Name must be provided")
	}

	if req.VolumeCapabilities == nil || len(req.VolumeCapabilities) == 0 {
		return nil, status.Error(codes.InvalidArgument, "CreateVolume Volume capabilities must be provided")
	}

	log.Info().Str("name", req.Name).Interface("capabilities", req.VolumeCapabilities).Msg("Creating volume")

	// Check capabilities
	for _, cap := range req.VolumeCapabilities {
		if _, ok := supportedAccessModes[cap.GetAccessMode().GetMode()]; !ok {
			return nil, status.Error(codes.InvalidArgument, "CreateVolume access mode isn't supported")
		}
		if _, ok := cap.GetAccessType().(*csi.VolumeCapability_Block); ok {
			return nil, status.Error(codes.InvalidArgument, "CreateVolume block types aren't supported, only mount types")
		}
	}

	// Determine required size
	bytes, err := getVolSizeInBytes(req.GetCapacityRange())
	if err != nil {
		return nil, err
	}

	desiredSize := bytes / BytesInGigabyte
	if (bytes % BytesInGigabyte) != 0 {
		desiredSize++
	}

	log.Debug().Int64("size_gb", desiredSize).Msg("Volume size determined")

	log.Debug().Msg("Listing current volumes in Civo API")
	volumes, err := d.storage.ListVolumes()
	if err != nil {
		log.Error().Err(err).Msg("Unable to list volumes in Civo API")
		return nil, err
	}
	for _, v := range volumes {
		if v.Name == req.Name {
			log.Debug().Str("volume_id", v.ID).Msg("Volume already exists")
			if v.SizeGigabytes != int(desiredSize) {
				return nil, status.Error(codes.AlreadyExists, "Volume already exists with a differnt size")

			}

			available, err := d.waitForVolumeStatus(&v, "available", CivoVolumeAvailableRetries)
			if err != nil {
				log.Error().Err(err).Msg("Unable to wait for volume availability in Civo API")
				return nil, err
			}

			if available {
				return &csi.CreateVolumeResponse{
					Volume: &csi.Volume{
						VolumeId:      v.ID,
						CapacityBytes: int64(v.SizeGigabytes) * BytesInGigabyte,
					},
				}, nil
			}

			log.Error().Str("status", v.Status).Msg("Civo Volume is not 'available'")
			return nil, status.Errorf(codes.Unavailable, "Volume isn't available to be attached, state is currently %s", v.Status)
		}
	}

	// TODO: Uncomment after client implementation is complete.
	// snapshotID := ""
	// if volSource := req.GetVolumeContentSource(); volSource != nil {
	// 	if _, ok := volSource.GetType().(*csi.VolumeContentSource_Snapshot); !ok {
	// 		return nil, status.Error(codes.InvalidArgument, "Unsupported volumeContentSource type")
	// 	}
	// 	snapshot := volSource.GetSnapshot()
	// 	if snapshot == nil {
	// 		return nil, status.Error(codes.InvalidArgument, "Volume content source type is set to Snapshot, but the Snapshot is not provided")
	// 	}
	// 	snapshotID = snapshot.GetSnapshotId()
	// 	if snapshotID == "" {
	// 		return nil, status.Error(codes.InvalidArgument, "Volume content source type is set to Snapshot, but the SnapshotID is not provided")
	// 	}
	// }

	log.Debug().Msg("Volume doesn't currently exist, will need creating")

	log.Debug().Msg("Requesting available capacity in client's quota from the Civo API")
	quota, err := d.storage.GetQuota()
	if err != nil {
		log.Error().Err(err).Msg("Unable to get quota from Civo API")
		return nil, err
	}
	availableSize := int64(quota.DiskGigabytesLimit - quota.DiskGigabytesUsage)
	if availableSize < desiredSize {
		log.Error().Msg("Requested volume would exceed storage quota available")
		return nil, status.Errorf(codes.OutOfRange, "Requested volume would exceed volume space quota by %d GB", desiredSize-availableSize)
	} else if quota.DiskVolumeCountUsage >= quota.DiskVolumeCountLimit {
		log.Error().Msg("Requested volume would exceed volume quota available")
		return nil, status.Errorf(codes.OutOfRange, "Requested volume would exceed volume count limit quota of %d", quota.DiskVolumeCountLimit)
	}

	log.Debug().Int("disk_gb_limit", quota.DiskGigabytesLimit).Int("disk_gb_usage", quota.DiskGigabytesUsage).Msg("Quota has sufficient capacity remaining")

	v := &civogo.VolumeConfig{
		Name:          req.Name,
		Region:        d.region,
		SizeGigabytes: int(desiredSize),
		// SnapshotID: snapshotID, // TODO: Uncomment after client implementation is complete.
	}
	log.Debug().Msg("Creating volume in Civo API")
	result, err := d.storage.NewVolume(v)
	if err != nil {
		log.Error().Err(err).Msg("Unable to create volume in Civo API")
		return nil, err
	}

	log.Info().Str("volume_id", result.ID).Msg("Volume created in Civo API")

	volume, err := d.storage.GetVolume(result.ID)
	if err != nil {
		log.Error().Err(err).Msg("Unable to get volume updates in Civo API")
		return nil, err
	}

	log.Debug().Str("volume_id", result.ID).Msg("Waiting for volume to become available in Civo API")
	available, err := d.waitForVolumeStatus(volume, "available", CivoVolumeAvailableRetries)
	if err != nil {
		log.Error().Err(err).Msg("Volume availability never completed successfully in Civo API")
		return nil, err
	}

	if available {
		return &csi.CreateVolumeResponse{
			Volume: &csi.Volume{
				VolumeId:      volume.ID,
				CapacityBytes: int64(v.SizeGigabytes) * BytesInGigabyte,
			},
		}, nil
	}

	log.Error().Err(err).Msg("Civo Volume is not 'available'")
	return nil, status.Errorf(codes.Unavailable, "Civo Volume %q is not \"available\", state currently is %q", volume.ID, volume.Status)
}

func getVolSizeInBytes(capRange *csi.CapacityRange) (int64, error) {
	if capRange == nil {
		return int64(DefaultVolumeSizeGB) * BytesInGigabyte, nil
	}

	// Volumes can be of a flexible size, but they must specify one of the fields, so we'll use that
	bytes := capRange.GetRequiredBytes()
	if bytes == 0 {
		bytes = capRange.GetLimitBytes()
	}

	return bytes, nil
}

// waitForVolumeAvailable will just sleep/loop waiting for Civo's API to report it's available, or hit a defined
// number of retries
func (d *Driver) waitForVolumeStatus(vol *civogo.Volume, desiredStatus string, retries int) (bool, error) {
	log.Info().Str("volume_id", vol.ID).Str("desired_state", desiredStatus).Msg("Waiting for Volume to entered desired state")
	var v *civogo.Volume
	var err error

	for i := 0; i < retries; i++ {
		time.Sleep(5 * time.Second)

		v, err = d.storage.GetVolume(vol.ID)
		if err != nil {
			log.Error().Err(err).Msg("Unable to get volume updates in Civo API")
			return false, err
		}

		if v.Status == desiredStatus {
			return true, nil
		}
	}
	return false, fmt.Errorf("volume isn't %s, state is currently %s", desiredStatus, v.Status)
}

func (d *Driver) DeleteVolume(c context.Context, r *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	return nil, nil
}
func (d *Driver) ControllerPublishVolume(c context.Context, r *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	return nil, nil
}
func (d *Driver) ControllerUnpublishVolume(c context.Context, r *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	return nil, nil
}
func (d *Driver) ValidateVolumeCapabilities(c context.Context, r *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	return nil, nil
}
func (d *Driver) ListVolumes(c context.Context, r *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	return nil, nil
}
func (d *Driver) GetCapacity(c context.Context, r *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
	return nil, nil
}
func (d *Driver) ControllerGetCapabilities(c context.Context, r *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {

	caps := []*csi.ControllerServiceCapability{}

	for _, c := range []csi.ControllerServiceCapability_RPC_Type{
		csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
		csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME,
	} {
		caps = append(caps, &csi.ControllerServiceCapability{
			Type: &csi.ControllerServiceCapability_Rpc{
				Rpc: &csi.ControllerServiceCapability_RPC{
					Type: c,
				},
			},
		})
	}

	return &csi.ControllerGetCapabilitiesResponse{
		Capabilities: caps,
	}, nil
}

func (d *Driver) CreateSnapshot(c context.Context, r *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	return nil, nil
}
func (d *Driver) DeleteSnapshot(c context.Context, r *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	return nil, nil
}
func (d *Driver) ListSnapshots(c context.Context, r *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	return nil, nil
}

func (d *Driver) ControllerExpandVolume(c context.Context, r *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	return nil, nil
}
func (d *Driver) ControllerGetVolume(c context.Context, r *csi.ControllerGetVolumeRequest) (*csi.ControllerGetVolumeResponse, error) {
	return nil, nil
}
func (d *Driver) ControllerModifyVolume(c context.Context, r *csi.ControllerModifyVolumeRequest) (*csi.ControllerModifyVolumeResponse, error) {
	return nil, nil
}
