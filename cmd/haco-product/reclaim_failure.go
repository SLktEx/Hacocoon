package main

import (
	"errors"
	"fmt"
	"io"

	"github.com/SLktEx/Hacocoon/internal/reclaimclient"
)

func writeReclaimInvocationFailure(out io.Writer, err error) {
	var failure *reclaimclient.InvocationError
	if !errors.As(err, &failure) || !failure.Failure.Valid() {
		return
	}
	f := failure.Failure
	_, _ = fmt.Fprintln(out, cliMessage("reclaim.failure", cliMessage("reclaim.phase."+f.Phase), cliMessage("reclaim.stage."+f.Stage)))
	if f.NativeError != 0 {
		_, _ = fmt.Fprintln(out, cliMessage("reclaim.native_error", f.NativeError))
	}
	key := "reclaim.inspect"
	switch f.Stage {
	case "registration", "enrollment", "installation", "binding":
		key = "reclaim.installation"
	case "disk_access", "record_access", "windows_owner":
		key = "reclaim.access"
	case "exclusion", "intent":
		key = "reclaim.existing"
	}
	_, _ = fmt.Fprintln(out, cliMessage(key))
}
