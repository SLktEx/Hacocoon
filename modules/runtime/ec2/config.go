package ec2

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type Config struct {
	Region           string
	ImageID          string
	InstanceType     string
	SubnetID         string
	SecurityGroupIDs []string
	InstanceProfile  string
	WorkspaceBucket  string
	WorkspacePrefix  string
}

func ConfigFromEnv() Config {
	groups := splitComma(os.Getenv("HACO_EC2_SECURITY_GROUP_IDS"))
	return Config{
		Region:           os.Getenv("HACO_EC2_REGION"),
		ImageID:          os.Getenv("HACO_EC2_AMI"),
		InstanceType:     os.Getenv("HACO_EC2_INSTANCE_TYPE"),
		SubnetID:         os.Getenv("HACO_EC2_SUBNET_ID"),
		SecurityGroupIDs: groups,
		InstanceProfile:  os.Getenv("HACO_EC2_INSTANCE_PROFILE"),
		WorkspaceBucket:  os.Getenv("HACO_EC2_WORKSPACE_BUCKET"),
		WorkspacePrefix:  os.Getenv("HACO_EC2_WORKSPACE_PREFIX"),
	}
}

var (
	regionPattern        = regexp.MustCompile(`^[a-z]{2}(?:-gov)?-[a-z0-9-]+-\d+$`)
	imagePattern         = regexp.MustCompile(`^ami-[a-zA-Z0-9]+$`)
	subnetPattern        = regexp.MustCompile(`^subnet-[a-zA-Z0-9]+$`)
	securityGroupPattern = regexp.MustCompile(`^sg-[a-zA-Z0-9]+$`)
	bucketPattern        = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$`)
)

func (c Config) normalized() (Config, error) {
	c.Region = strings.TrimSpace(c.Region)
	c.ImageID = strings.TrimSpace(c.ImageID)
	c.InstanceType = strings.TrimSpace(c.InstanceType)
	c.SubnetID = strings.TrimSpace(c.SubnetID)
	c.InstanceProfile = strings.TrimSpace(c.InstanceProfile)
	c.WorkspaceBucket = strings.TrimSpace(c.WorkspaceBucket)
	c.WorkspacePrefix = strings.Trim(strings.TrimSpace(c.WorkspacePrefix), "/")
	if c.WorkspacePrefix == "" {
		c.WorkspacePrefix = "hacocoon"
	}
	for i := range c.SecurityGroupIDs {
		c.SecurityGroupIDs[i] = strings.TrimSpace(c.SecurityGroupIDs[i])
	}

	switch {
	case !regionPattern.MatchString(c.Region):
		return Config{}, fmt.Errorf("EC2 region %q: %w", c.Region, core.ErrRuntimeUnavailable)
	case !imagePattern.MatchString(c.ImageID):
		return Config{}, fmt.Errorf("EC2 AMI %q: %w", c.ImageID, core.ErrRuntimeUnavailable)
	case c.InstanceType == "" || unsafeToken(c.InstanceType):
		return Config{}, fmt.Errorf("EC2 instance type %q: %w", c.InstanceType, core.ErrRuntimeUnavailable)
	case !subnetPattern.MatchString(c.SubnetID):
		return Config{}, fmt.Errorf("EC2 subnet %q: %w", c.SubnetID, core.ErrRuntimeUnavailable)
	case c.InstanceProfile == "" || unsafeToken(c.InstanceProfile):
		return Config{}, fmt.Errorf("EC2 instance profile %q: %w", c.InstanceProfile, core.ErrRuntimeUnavailable)
	case !bucketPattern.MatchString(c.WorkspaceBucket):
		return Config{}, fmt.Errorf("EC2 workspace bucket %q: %w", c.WorkspaceBucket, core.ErrRuntimeUnavailable)
	case unsafeToken(c.WorkspacePrefix) || strings.Contains(c.WorkspacePrefix, ".."):
		return Config{}, fmt.Errorf("EC2 workspace prefix %q: %w", c.WorkspacePrefix, core.ErrRuntimeUnavailable)
	}
	if len(c.SecurityGroupIDs) == 0 {
		return Config{}, fmt.Errorf("EC2 security groups: %w", core.ErrRuntimeUnavailable)
	}
	for _, id := range c.SecurityGroupIDs {
		if !securityGroupPattern.MatchString(id) {
			return Config{}, fmt.Errorf("EC2 security group %q: %w", id, core.ErrRuntimeUnavailable)
		}
	}
	return c, nil
}

func splitComma(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if value := strings.TrimSpace(part); value != "" {
			out = append(out, value)
		}
	}
	return out
}

func unsafeToken(value string) bool { return strings.ContainsAny(value, "\r\n\x00") }
