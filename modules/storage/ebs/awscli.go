package ebs

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

type AWSCLI struct {
	runner host.Runner
	region string
}

func NewAWSCLI(runner host.Runner, region string) *AWSCLI {
	return &AWSCLI{runner: runner, region: strings.TrimSpace(region)}
}

type awsVolume struct {
	VolumeID         string `json:"VolumeId"`
	Size             int64  `json:"Size"`
	AvailabilityZone string `json:"AvailabilityZone"`
	VolumeType       string `json:"VolumeType"`
	Encrypted        bool   `json:"Encrypted"`
	KMSKeyID         string `json:"KmsKeyId"`
}

func (a *AWSCLI) DescribeVolume(ctx context.Context, id string) (Volume, error) {
	result, err := a.aws(ctx, "ec2", "describe-volumes", "--volume-ids", id, "--query", "Volumes[0]", "--output", "json")
	if err != nil {
		return Volume{}, err
	}
	var v awsVolume
	if err := json.Unmarshal([]byte(result.Stdout), &v); err != nil {
		return Volume{}, fmt.Errorf("decode EBS volume: %w", err)
	}
	if v.VolumeID == "" || v.Size <= 0 || v.AvailabilityZone == "" || v.VolumeType == "" {
		return Volume{}, core.ErrIncompatibleState
	}
	return Volume{
		ID:               v.VolumeID,
		SizeGiB:          v.Size,
		AvailabilityZone: v.AvailabilityZone,
		Type:             v.VolumeType,
		Encrypted:        v.Encrypted,
		KMSKeyID:         v.KMSKeyID,
	}, nil
}

func (a *AWSCLI) CreateVolume(ctx context.Context, source Volume, size int64, clientToken string) (string, error) {
	if strings.TrimSpace(clientToken) == "" || strings.TrimSpace(clientToken) != clientToken || len(clientToken) > 64 || strings.ContainsAny(clientToken, "\r\n\x00") {
		return "", core.ErrInvalidArgument
	}
	args := []string{
		"ec2", "create-volume",
		"--availability-zone", source.AvailabilityZone,
		"--size", fmt.Sprint(size),
		"--volume-type", source.Type,
		"--client-token", clientToken,
	}
	if source.Encrypted {
		args = append(args, "--encrypted")
	}
	if source.KMSKeyID != "" {
		args = append(args, "--kms-key-id", source.KMSKeyID)
	}
	args = append(args, "--query", "VolumeId", "--output", "text")
	result, err := a.aws(ctx, args...)
	if err != nil {
		return "", err
	}
	id := strings.TrimSpace(result.Stdout)
	if !strings.HasPrefix(id, "vol-") || strings.ContainsAny(id, "\r\n\x00 ") {
		return "", core.ErrIncompatibleState
	}
	return id, nil
}

func (a *AWSCLI) AttachVolume(ctx context.Context, volume, instance, device string) error {
	_, err := a.aws(ctx, "ec2", "attach-volume", "--volume-id", volume, "--instance-id", instance, "--device", device)
	return err
}

func (a *AWSCLI) DetachVolume(ctx context.Context, volume, instance string) error {
	_, err := a.aws(ctx, "ec2", "detach-volume", "--volume-id", volume, "--instance-id", instance)
	return err
}

func (a *AWSCLI) WaitAvailable(ctx context.Context, volume string) error {
	_, err := a.aws(ctx, "ec2", "wait", "volume-available", "--volume-ids", volume)
	return err
}

func (a *AWSCLI) WaitInUse(ctx context.Context, volume string) error {
	_, err := a.aws(ctx, "ec2", "wait", "volume-in-use", "--volume-ids", volume)
	return err
}

func (a *AWSCLI) DeleteVolume(ctx context.Context, volume string) error {
	_, err := a.aws(ctx, "ec2", "delete-volume", "--volume-id", volume)
	return err
}

func (a *AWSCLI) aws(ctx context.Context, args ...string) (host.Result, error) {
	if a == nil || a.runner == nil || a.region == "" || strings.ContainsAny(a.region, "\r\n\x00") {
		return host.Result{}, core.ErrInvalidArgument
	}
	all := append([]string{"--region", a.region, "--no-cli-pager"}, args...)
	return a.runner.Run(ctx, "aws", all...)
}
