package herdr

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync/atomic"
)

// ndjsonClient speaks the herdr socket API: newline-delimited JSON requests
// {"id": ..., "method": ..., "params": {...}} over a unix socket or TCP, with
// responses {"id": ..., "result": {...}} matched by request ID (the wire
// contract herdr.dev/docs/socket-api + `herdr api schema --json`).
//
// The herdr server closes the socket after answering one request (the herdr CLI
// itself dials a fresh socket per invocation), so call() is a DIAL-PER-CALL
// exchange on a fresh connection — observed against a live herdr 0.8.2 server.
type ndjsonClient struct {
	target string // "unix:///path" | "tcp://host:port" | "host:port"
	seq    atomic.Uint64
}

func newNDJSONClient(target string) *ndjsonClient {
	return &ndjsonClient{target: target}
}

func splitTarget(target string) (network, addr string) {
	if rest, ok := strings.CutPrefix(target, "unix://"); ok {
		return "unix", rest
	}
	if rest, ok := strings.CutPrefix(target, "tcp://"); ok {
		return "tcp", rest
	}
	return "tcp", target
}

// rpcError is the herdr error envelope (schemas: error_response).
type rpcError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// apiError wraps a herdr server error with the method that failed.
type apiError struct {
	Method  string
	Code    string
	Message string
}

func (e *apiError) Error() string {
	if e.Code == "" {
		return fmt.Sprintf("herdr %s: %s", e.Method, e.Message)
	}
	return fmt.Sprintf("herdr %s: %s (%s)", e.Method, e.Message, e.Code)
}

// rpcResponse is the herdr response envelope. Per the API schema the result arm
// carries its own `type` discriminator and the server may also mirror it at the
// envelope level — armType accepts either form.
type rpcResponse struct {
	ID     string          `json:"id"`
	Result json.RawMessage `json:"result"`
	Type   string          `json:"type"`
	Error  *rpcError       `json:"error"`
}

func (r *rpcResponse) armType() string {
	var probe struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(r.Result, &probe); err == nil && probe.Type != "" {
		return probe.Type
	}
	return r.Type
}

// call sends one NDJSON request on a FRESH connection and reads the matching
// response. Non-matching frames (e.g. pushed events) are skipped until the
// response ID matches or the server closes.
func (c *ndjsonClient) call(ctx context.Context, method string, params any) (json.RawMessage, string, error) {
	network, addr := splitTarget(c.target)
	var d net.Dialer
	conn, err := d.DialContext(ctx, network, addr)
	if err != nil {
		return nil, "", fmt.Errorf("herdr: dial %s: %w", c.target, err)
	}
	defer conn.Close()

	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	if params == nil {
		params = struct{}{}
	}
	id := fmt.Sprintf("charly:%d", c.seq.Add(1))
	req := map[string]any{"id": id, "method": method, "params": params}

	enc := json.NewEncoder(conn)
	if err := enc.Encode(req); err != nil {
		return nil, "", fmt.Errorf("herdr %s: send: %w", method, err)
	}

	dec := json.NewDecoder(conn)
	for {
		var resp rpcResponse
		if err := dec.Decode(&resp); err != nil {
			return nil, "", fmt.Errorf("herdr %s: recv: %w", method, err)
		}
		if resp.ID != id {
			continue
		}
		if resp.Error != nil {
			return nil, "", &apiError{Method: method, Code: resp.Error.Code, Message: resp.Error.Message}
		}
		return resp.Result, resp.armType(), nil
	}
}

// decodeResult decodes a call's result payload into the target struct.
func decodeResult[T any](raw json.RawMessage, wantType string, got string) (*T, error) {
	if wantType != "" && got != wantType {
		return nil, fmt.Errorf("herdr: unexpected result type %q (want %q)", got, wantType)
	}
	var out T
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("herdr: decode %s result: %w", wantType, err)
	}
	return &out, nil
}
