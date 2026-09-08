//go:build !linux

package main

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/core"
	awsplugin "github.com/SLktEx/Hacocoon/modules/capability/aws"
)

func saveAWSDownload(context.Context, awsDownloadClient, awsplugin.GetSpec, string) error {
	return core.ErrUnsupported
}
