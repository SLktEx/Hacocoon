//go:build !linux

package cli

import (
	"context"
	awsplugin "github.com/SLktEx/Hacocoon/internal/adapters/aws"
	"github.com/SLktEx/Hacocoon/internal/core"
)

func saveAWSDownload(context.Context, awsDownloadClient, awsplugin.GetSpec, string) error {
	return core.ErrUnsupported
}
