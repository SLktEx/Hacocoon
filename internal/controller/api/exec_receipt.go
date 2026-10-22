package controlapi

import (
	"bytes"
	"encoding/json"
	"github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/core"
	"io"
)

type execResponse struct {
	Result core.ExecutionResult `json:"result"`
	Error  *responseStatus      `json:"error,omitempty"`
}

func decodeExecResult(data []byte) (core.ExecutionResult, error) {
	var response struct {
		Result *core.ExecutionResult `json:"result"`
		Error  *responseStatus       `json:"error,omitempty"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&response) != nil || response.Result == nil || decoder.Decode(new(any)) != io.EOF {
		return core.ExecutionResult{}, control.ErrProtocol
	}
	return *response.Result, responseError(response.Error)
}
