package plugin

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

const Version = 1
const Limit = 65536

type Request struct {
	Version   int             `json:"version"`
	Arguments json.RawMessage `json:"arguments"`
}
type Response struct {
	Version int    `json:"version"`
	Output  string `json:"output,omitempty"`
	Error   string `json:"error,omitempty"`
}

func Serve(in io.Reader, out io.Writer, call func(json.RawMessage) (string, error)) error {
	data, err := io.ReadAll(io.LimitReader(in, Limit+1))
	if err != nil {
		return err
	}
	if len(data) > Limit {
		return errors.New("plugin request too large")
	}
	var request Request
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err = d.Decode(&request); err != nil {
		return err
	}
	if err = d.Decode(new(any)); err != io.EOF {
		return errors.New("expected one plugin request")
	}
	if request.Version != Version {
		return errors.New("unsupported plugin protocol")
	}
	output, err := call(request.Arguments)
	response := Response{Version: Version, Output: output}
	if err != nil {
		response.Error = err.Error()
		response.Output = ""
	}
	data, err = json.Marshal(response)
	if err != nil {
		return err
	}
	if len(data) > Limit {
		return errors.New("plugin response too large")
	}
	_, err = out.Write(data)
	return err
}
