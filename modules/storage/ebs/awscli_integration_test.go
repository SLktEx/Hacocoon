package ebs

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/host"
)

func TestAWSCLICrossesProcessBoundaryWithProviderSpecificCommands(t *testing.T) {
	root := t.TempDir(); bin := filepath.Join(root, "bin"); if err := os.Mkdir(bin, 0o755); err != nil { t.Fatal(err) }
	logPath := filepath.Join(root, "aws.log")
	script := `#!/bin/sh
set -eu
printf '%s\n' "$*" >> "$HACO_FAKE_AWS_LOG"
case "$*" in
  *"describe-volumes"*) printf '%s\n' '{"VolumeId":"vol-source","Size":100,"AvailabilityZone":"ap-northeast-1a","VolumeType":"gp3","Encrypted":true,"KmsKeyId":"alias/test"}' ;;
  *"create-volume"*) printf '%s\n' 'vol-target' ;;
esac
`
	aws := filepath.Join(bin, "aws"); if err := os.WriteFile(aws, []byte(script), 0o755); err != nil { t.Fatal(err) }
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH")); t.Setenv("HACO_FAKE_AWS_LOG", logPath)
	api := NewAWSCLI(host.ExecRunner{}, "ap-northeast-1")
	source, err := api.DescribeVolume(context.Background(), "vol-source"); if err != nil { t.Fatal(err) }
	target, err := api.CreateVolume(context.Background(), source, 60); if err != nil { t.Fatal(err) }; if target != "vol-target" { t.Fatalf("target=%q", target) }
	if err := api.AttachVolume(context.Background(), target, "i-0123456789abcdef0", "/dev/sdg"); err != nil { t.Fatal(err) }
	if err := api.DetachVolume(context.Background(), "vol-source", "i-0123456789abcdef0"); err != nil { t.Fatal(err) }
	if err := api.DeleteVolume(context.Background(), target); err != nil { t.Fatal(err) }
	content, err := os.ReadFile(logPath); if err != nil { t.Fatal(err) }; log := string(content)
	for _, want := range []string{"--region ap-northeast-1 --no-cli-pager ec2 describe-volumes", "ec2 create-volume --availability-zone ap-northeast-1a --size 60 --volume-type gp3 --encrypted --kms-key-id alias/test", "ec2 wait volume-available --volume-ids vol-target", "ec2 attach-volume", "ec2 detach-volume", "ec2 delete-volume"} { if !strings.Contains(log, want) { t.Fatalf("missing %q in:\n%s", want, log) } }
}
